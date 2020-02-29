// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	ggio "github.com/gogo/protobuf/io"
	"github.com/libp2p/go-libp2p-core/helpers"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/validators"
)

type communicationMode int

const (
	NoOp communicationMode = iota
	OneToOne
	OneToK
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	log        zerolog.Logger
	codec      network.Codec
	ov         middleware.Overlay
	wg         *sync.WaitGroup
	libP2PNode *P2PNode
	stop       chan struct{}
	me         flow.Identifier
	host       string
	port       string
	validators []validators.MessageValidator
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger, codec network.Codec, address string, flowID flow.Identifier) (*Middleware, error) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	p2p := &P2PNode{}
	ctx, cancel := context.WithCancel(context.Background())

	// add validators to filter out unwanted messages received by this node
	validators := []validators.MessageValidator{
		validators.NewSenderValidator(flowID),      // validator to filter out messages sent by this node itself
		validators.NewTargetValidator(log, flowID), // validator to filter out messages not intended for this node
	}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		ctx:        ctx,
		cancel:     cancel,
		log:        log,
		codec:      codec,
		libP2PNode: p2p,
		wg:         &sync.WaitGroup{},
		stop:       make(chan struct{}),
		me:         flowID,
		host:       ip,
		port:       port,
		validators: validators,
	}

	return m, err
}

// Me returns the flow identifier of the this middleware
func (m *Middleware) Me() flow.Identifier {
	return m.me
}

// GetIPPort returns the ip address and port number associated with the middleware
func (m *Middleware) GetIPPort() (string, string) {
	return m.libP2PNode.GetIPPort()
}

// Start will start the middleware.
func (m *Middleware) Start(ov middleware.Overlay) error {

	m.ov = ov

	// create a discovery object to help libp2p discover peers
	d := NewDiscovery(m.log, m.ov, m.me, m.stop)

	// create PubSub options for libp2p to use the discovery object
	psOptions := []pubsub.Option{pubsub.WithDiscovery(d),
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	}

	nodeAddress := NodeAddress{Name: m.me.String(), IP: m.host, Port: m.port}

	// start the libp2p node
	err := m.libP2PNode.Start(m.ctx, nodeAddress, m.log, m.handleIncomingStream, psOptions...)

	if err != nil {
		return fmt.Errorf("failed to start libp2p node: %w", err)
	}

	// the ip,port may change after libp2p has been started. e.g. 0.0.0.0:0 would change to an actual IP and port
	m.host, m.port = m.libP2PNode.GetIPPort()

	return nil
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)

	// cancel the context (this also signals libp2p go routines to exit)
	m.cancel()

	// stop libp2p
	done, err := m.libP2PNode.Stop()
	if err != nil {
		log.Error().Err(err).Msg("stopping failed")
	} else {
		<-done
		log.Debug().Msg("node stopped successfully")
	}

	// wait for the go routines spawned by middleware to stop
	m.wg.Wait()
}

// Send sends the message to the set of target ids
// If there is only one target NodeID, then a direct 1-1 connection is used by calling middleware.sendDirect
// Otherwise, middleware.publish is used, which uses the PubSub method of communication.
func (m *Middleware) Send(channelID uint8, msg interface{}, targetIDs ...flow.Identifier) error {
	var err error
	mode := m.chooseMode(channelID, msg, targetIDs...)
	// decide what mode of communication to use
	switch mode {
	case NoOp:
		// TODO: Decide if this is actually an error or not
		// return fmt.Errorf("empty list of target Ids")
		return nil
	case OneToOne:
		if targetIDs[0] == m.me {
			// to avoid self dial by the underlay
			m.log.Debug().Msg("self dial attempt")
			return nil
		}
		err = m.sendDirect(targetIDs[0], msg)
	case OneToK:
		err = m.publish(strconv.Itoa(int(channelID)), msg)
	default:
		err = fmt.Errorf("invalid communcation mode: %d", mode)
	}

	if err != nil {
		return fmt.Errorf("failed to send message to %s:%w", targetIDs, err)
	}
	return nil
}

// chooseMode determines the communication mode to use. Currently it only considers the length of the targetIDs.
func (m *Middleware) chooseMode(_ uint8, _ interface{}, targetIDs ...flow.Identifier) communicationMode {
	switch len(targetIDs) {
	case 0:
		return NoOp
	case 1:
		return OneToOne
	default:
		return OneToK
	}
}

// sendDirect will try to send the given message to the given peer utilizing a 1-1 direct connection
func (m *Middleware) sendDirect(targetID flow.Identifier, msg interface{}) error {

	switch msg := msg.(type) {
	case *message.Message:

		// get an identity to connect to. The identity provides the destination TCP address.
		idsMap, err := m.ov.Identity()
		if err != nil {
			return fmt.Errorf("could not get identities: %w", err)
		}
		flowIdentity, found := idsMap[targetID]
		if !found {
			return fmt.Errorf("could not get identity for %s: %w", targetID.String(), err)
		}

		// create new stream
		// (streams don't need to be reused and are fairly inexpensive to be created for each send.
		// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
		// sent out the the receiver
		stream, err := m.connect(flowIdentity.NodeID.String(), flowIdentity.Address)
		if err != nil {
			return fmt.Errorf("could not create new stream for %s: %w", targetID.String(), err)
		}

		// create a gogo protobuf writer
		bufw := bufio.NewWriter(stream)
		writer := ggio.NewDelimitedWriter(bufw)

		err = writer.WriteMsg(msg)
		if err != nil {
			return fmt.Errorf("failed to send message to %s: %w", targetID.String(), err)
		}

		// flush the stream
		err = bufw.Flush()
		if err != nil {
			return fmt.Errorf("failed to flush stream for %s: %w", targetID.String(), err)
		}

		// close the stream immediately
		// helpers.FullClose will close the stream, wait for an EOF and then call reset on the stream
		// this is the ideal way of closing the stream in libp2p as of now
		go helpers.FullClose(stream)

		return nil

	default:
		err := errors.Errorf("invalid message type (%T)", msg)
		return err
	}
}

// connect creates a new stream
func (m *Middleware) connect(flowID string, address string) (libp2pnetwork.Stream, error) {

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not parse address %s:%v", address, err)
	}

	// Create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowID, IP: ip, Port: port}

	// Create a stream for it
	stream, err := m.libP2PNode.CreateStream(m.ctx, nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream for %s:%v", nodeAddress.Name, err)
	}

	log.Info().Str("targetid", flowID).Str("address", address).Msg("stream created")
	return stream, nil
}

// handleIncomingStream handles an incoming stream from a remote peer
// this is a blocking call, so that the deferred resource cleanup happens after
// we are done handling the connection
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {
	m.wg.Add(1)
	defer m.wg.Done()

	log := m.log.With().
		Str("local_addr", s.Conn().LocalPeer().String()).
		Str("remote_addr", s.Conn().RemotePeer().String()).
		Logger()

	// initialize the encoder/decoder and create the connection handler
	conn := NewReadConnection(log, s)

	// make sure we close the connection when we are done handling the peer
	defer conn.stop()

	log.Info().Msg("incoming connection established")

	// start processing messages in the background
	go conn.ReceiveLoop()

	// process incoming messages for as long as the peer is running
ProcessLoop:
	for {
		select {
		case <-m.stop:
			m.log.Info().Msg("exiting process loop: middleware stops")
			break ProcessLoop
		case <-conn.done:
			m.log.Info().Msg("exiting process loop: connection stops")
			break ProcessLoop
		case msg, ok := <-conn.inbound:
			if !ok {
				m.log.Info().Msg("exiting process loop: connection stops")
				break ProcessLoop
			}
			m.processMessage(msg)
			continue ProcessLoop
		}
	}

	log.Info().Msg("middleware closed the connection")
}

// Subscribe will subscribe the middleware for a topic with the same name as the channelID
func (m *Middleware) Subscribe(channelID uint8) error {
	// A Flow ChannelID becomes the topic ID in libp2p.
	s, err := m.libP2PNode.Subscribe(m.ctx, strconv.Itoa(int(channelID)))
	if err != nil {
		return fmt.Errorf("failed to subscribe for channel %d: %w", channelID, err)
	}
	rs := NewReadSubscription(m.log, s)
	go rs.ReceiveLoop()

	// add to waitgroup to wait for the inbound subscription go routine during stop
	m.wg.Add(1)
	go m.handleInboundSubscription(rs)
	return nil
}

// handleInboundSubscription reads the messages from the channel written to by readsSubscription and processes them
func (m *Middleware) handleInboundSubscription(rs *ReadSubscription) {
	defer m.wg.Done()
	// process incoming messages for as long as the peer is running
SubscriptionLoop:
	for {
		select {
		case <-m.stop:
			// middleware stops
			m.log.Info().Msg("exiting subscription loop: middleware stops")
			break SubscriptionLoop
		case <-rs.done:
			// subscription stops
			m.log.Info().Msg("exiting subscription loop: connection stops")
			break SubscriptionLoop
		case msg, ok := <-rs.inbound:
			if !ok {
				m.log.Info().Msg("exiting subscription loop: connection stops")
				break SubscriptionLoop
			}
			m.processMessage(msg)
			continue SubscriptionLoop
		}
	}
}

// processMessage processes a message and eventually passes it to the overlay
func (m *Middleware) processMessage(msg *message.Message) {

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(*msg) {
			return
		}
	}

	// if validation passed, send the message to the overlay
	err := m.ov.Receive(flow.HashToID(msg.OriginID), msg)
	if err != nil {
		log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Publish publishes the given payload on the topic
func (m *Middleware) publish(topic string, msg interface{}) error {
	switch msg := msg.(type) {
	case *message.Message:

		// convert the message to bytes to be put on the wire.
		data, err := msg.Marshal()
		if err != nil {
			return fmt.Errorf("failed to marshal the message: %w", err)
		}

		// publish the bytes on the topic
		// pubsub.GossipSubDlo is the minimal number of peer connections that libp2p will maintain
		// for this node.
		// TODO: specify a minpeers that makes more sense for Flow (Currently, broadcast if there is at least one peer listening)
		err = m.libP2PNode.Publish(m.ctx, topic, data, 1)
		if err != nil {
			return fmt.Errorf("failed to publish the message: %w", err)
		}
	default:
		err := fmt.Errorf("middleware received invalid message type (%T)", msg)
		return err
	}
	return nil
}
