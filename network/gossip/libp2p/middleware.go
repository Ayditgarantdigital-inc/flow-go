// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"context"
	"fmt"
	"net"
	"sync"

	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	log        zerolog.Logger
	codec      network.Codec
	ov         middleware.Overlay
	cc         *ConnectionCache
	wg         *sync.WaitGroup
	libP2PNode *P2PNode
	stop       chan struct{}
	me         flow.Identifier
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger, codec network.Codec, address string, flowID flow.Identifier) (*Middleware, error) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	nodeAddress := NodeAddress{Name: flowID.String(), IP: ip, Port: port}
	p2p := &P2PNode{}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		log:        log,
		codec:      codec,
		cc:         NewConnectionCache(),
		libP2PNode: p2p,
		wg:         &sync.WaitGroup{},
		stop:       make(chan struct{}),
		me:         flowID,
	}

	// Start the libp2p node
	err = p2p.Start(context.Background(), nodeAddress, log, m.handleIncomingStream)

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
func (m *Middleware) Start(ov middleware.Overlay) {
	m.ov = ov
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)

	// Stop all the connections
	for _, conn := range m.cc.GetAll() {
		conn.stop()
	}

	// Stop libp2p
	err := m.libP2PNode.Stop()
	if err != nil {
		log.Error().Err(err).Msg("stopping failed")
	} else {
		log.Debug().Msg("node stopped successfully")
	}
	m.wg.Wait()
}

// Send will try to send the given message to the given peer.
func (m *Middleware) Send(targetID flow.Identifier, msg interface{}) error {
	m.Lock()
	defer m.Unlock()
	found, stale := false, false
	var conn *WriteConnection

	log := m.log.With().Str("nodeid", targetID.String()).Logger()

	if conn, found = m.cc.Get(targetID); found {
		// check if the peer is still running
		select {
		case <-conn.done:
			// connection found to be stale; replace with a new one
			log.Debug().Msg("existing connection already closed ")
			stale = true
			conn = nil
			m.cc.Remove(targetID)
		default:
			log.Debug().Msg("reusing existing connection")
		}
	} else {
		log.Debug().Str("nodeid", targetID.String()).Msg("connection not found, creating one")
	}

	if !found || stale {

		// get an identity to connect to. The identity provides the destination TCP address.
		flowIdentity, err := m.ov.Identity(targetID)
		if err != nil {
			return fmt.Errorf("could not get identity for %s: %w", targetID.String(), err)
		}

		// create new connection
		conn, err = m.connect(flowIdentity.NodeID.String(), flowIdentity.Address)
		if err != nil {
			return fmt.Errorf("could not create new connection for %s: %w", targetID.String(), err)
		}

		// cache the connection against the node id
		m.cc.Add(flowIdentity.NodeID, conn)

		// kick-off a go routine (one for each outbound connection)
		m.wg.Add(1)
		go m.handleOutboundConnection(flowIdentity.NodeID, conn)

	}

	// send the message if connection still valid
	select {
	case <-conn.done:
		return errors.Errorf("connection has closed (node_id: %s)", targetID.String())
	default:
		switch msg := msg.(type) {
		case *message.Message:
			// Write message to outbound channel only if it is of the correct type
			conn.outbound <- msg
		default:
			err := errors.Errorf("middleware received invalid message type (%T)", msg)
			return err
		}
		return nil
	}
}

// connect creates a new connection
func (m *Middleware) connect(flowID string, address string) (*WriteConnection, error) {

	log := m.log.With().Str("targetid", flowID).Str("address", address).Logger()

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not parse address %s:%v", address, err)
	}

	// Create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowID, IP: ip, Port: port}

	// Create a stream for it
	stream, err := m.libP2PNode.CreateStream(context.Background(), nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream for %s:%v", nodeAddress.Name, err)
	}

	clog := m.log.With().
		Str("local_addr", stream.Conn().LocalPeer().String()).
		Str("remote_addr", stream.Conn().RemotePeer().String()).
		Logger()

	// create the write connection handler
	conn := NewWriteConnection(clog, stream)

	log.Info().Msg("connection established")

	return conn, nil
}

func (m *Middleware) handleOutboundConnection(targetID flow.Identifier, conn *WriteConnection) {
	defer m.wg.Done()
	// Remove the conn from the cache when done
	defer m.cc.Remove(targetID)
	// make sure we close the stream once the handling is done
	defer conn.stream.Close()
	// kick off the send loop
	conn.SendLoop()
}

// handleIncomingStream handles an incoming stream from a remote peer
// this is a blocking call, so that the deferred resource cleanup happens after
// we are done handling the connection
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {
	m.wg.Add(1)
	defer m.wg.Done()

	// make sure we close the connection when we are done handling the peer
	defer s.Close()

	log := m.log.With().
		Str("local_addr", s.Conn().LocalPeer().String()).
		Str("remote_addr", s.Conn().RemotePeer().String()).
		Logger()

	// initialize the encoder/decoder and create the connection handler
	conn := NewReadConnection(log, s)

	log.Info().Msg("incoming connection established")

	// start processing messages in the background
	go conn.ReceiveLoop()

	// process incoming messages for as long as the peer is running
ProcessLoop:
	for {
		select {
		case <-conn.done:
			m.log.Info().Msg("middleware stopped reception of incoming messages")
			break ProcessLoop
		case msg := <-conn.inbound:
			log.Info().Msg("middleware received a new message")
			nodeID, err := getSenderID(msg)
			if err != nil {
				log.Error().Err(err).Msg("could not extract sender ID")
				continue ProcessLoop
			}
			err = m.ov.Receive(*nodeID, msg)
			if err != nil {
				log.Error().Err(err).Msg("could not deliver payload")
				continue ProcessLoop
			}
		}
	}

	log.Info().Msg("middleware closed the connection")
}

func getSenderID(msg *message.Message) (*flow.Identifier, error) {
	// Extract sender id
	if len(msg.SenderID) < 32 {
		err := fmt.Errorf("invalid sender id")
		return nil, err
	}
	var senderID [32]byte
	copy(senderID[:], msg.SenderID)
	var id flow.Identifier = senderID
	return &id, nil
}
