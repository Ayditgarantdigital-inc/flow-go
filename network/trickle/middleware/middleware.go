// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package middleware

import (
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/trickle"
)

// Deprecated: use libp2p.Middleware instead
//
// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	log   zerolog.Logger
	codec network.Codec
	ov    trickle.Overlay
	slots chan struct{} // semaphore for outgoing connection slots
	conns map[flow.Identifier]*Connection
	ln    net.Listener
	wg    *sync.WaitGroup
	stop  chan struct{}
}

// New creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func New(log zerolog.Logger, codec network.Codec, conns uint, address string) (*Middleware, error) {

	// initialize the listener so we can receive incoming connections
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, errors.Wrapf(err, "could not listen on address (%s)", address)
	}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		log:   log.With().Str("component", "middleware").Logger(),
		codec: codec,
		slots: make(chan struct{}, conns),
		conns: make(map[flow.Identifier]*Connection),
		ln:    ln,
		wg:    &sync.WaitGroup{},
		stop:  make(chan struct{}),
	}

	return m, nil
}

// Start will start the middleware.
func (m *Middleware) Start(ov trickle.Overlay) {
	m.ov = ov
	m.wg.Add(2)
	go m.rotate()
	go m.host()
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)
	m.Lock()
	_ = m.ln.Close()
	for _, conn := range m.conns {
		conn.stop()
	}
	m.Unlock()
	m.wg.Wait()
}

// Send will try to send the given message to the given peer.
func (m *Middleware) Send(nodeID flow.Identifier, msg interface{}) error {
	m.Lock()
	defer m.Unlock()

	// get the conn from our list
	conn, ok := m.conns[nodeID]
	if !ok {
		return errors.Errorf("connection not found (node_id: %s)", nodeID)
	}

	// check if the peer is still running
	select {
	case <-conn.done:
		return errors.Errorf("connection already closed (node_id: %s)", nodeID)
	default:
	}

	// whichever comes first, sending the message or ending the provided context
	select {
	case <-conn.done:
		return errors.Errorf("connection has closed (node_id: %s)", nodeID)
	case conn.outbound <- msg:
		return nil
	}
}

func (m *Middleware) rotate() {
	defer m.wg.Done()

Loop:
	for {
		select {

		// for each free connection slot, we create a new connection
		case m.slots <- struct{}{}:

			// launch connection attempt
			m.wg.Add(1)
			go m.connect()

			// TODO: add proper rate limiter
			time.Sleep(time.Second)

		case <-m.stop:
			break Loop
		}
	}
}

func (m *Middleware) connect() {
	defer m.wg.Done()

	log := m.log.With().Logger()

	// make sure we free up the connection slop once we drop the peer
	defer m.release(m.slots)

	// get an address to connect to
	address, err := m.ov.Address()
	if err != nil {
		log.Error().Err(err).Msg("could not get address")
		return
	}

	log = log.With().Str("address", address).Logger()

	// create the new connection
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Error().Err(err).Msg("could not dial address")
		return
	}

	// make sure we close the connection once the handling is done
	defer conn.Close()

	// this is a blocking call so that the defered cleanups run only after we are
	// done handling this peer
	m.handle(conn)
}

func (m *Middleware) host() {
	defer m.wg.Done()

	log := m.log.With().Str("listen_address", m.ln.Addr().String()).Logger()

	for {

		// accept the next waiting connection
		conn, err := m.ln.Accept()
		if isClosedErr(err) {
			log.Debug().Msg("stopped accepting connections")
			break
		}
		if err != nil {
			log.Error().Err(err).Msg("could not accept connection")
			break
		}

		// initialize the connection
		m.wg.Add(1)
		go m.init(conn)
	}
}

func (m *Middleware) init(conn net.Conn) {
	defer m.wg.Done()

	log := m.log.With().
		Str("local_addr", conn.LocalAddr().String()).
		Str("remote_addr", conn.RemoteAddr().String()).
		Logger()

	// make sure we close the connection when we are done handling the peer
	defer conn.Close()

	// get a free connection slot and make sure to free it after we drop the peer
	select {
	case m.slots <- struct{}{}:
		defer m.release(m.slots)
	default:
		log.Debug().Msg("connection slots full")
		return
	}

	// this is a blocking call, so that the defered resource cleanup happens after
	// we are done handling the connection
	m.handle(conn)
}

func (m *Middleware) handle(netc net.Conn) {

	log := m.log.With().
		Str("local_addr", netc.LocalAddr().String()).
		Str("remote_addr", netc.RemoteAddr().String()).
		Logger()

	// initialize the encoder/decoder and create the connection handler
	conn := NewConnection(log, m.codec, netc)

	// execute the initial handshake
	nodeID, err := m.ov.Handshake(conn)
	if isClosedErr(err) {
		log.Debug().Msg("connection aborted remotely")
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("could not execute handshake")
		return
	}

	log = log.With().Hex("node_id", nodeID[:]).Logger()

	// register the peer with the returned peer ID
	m.add(nodeID, conn)
	defer m.remove(nodeID)

	log.Info().Msg("connection established")

	// start processing messages in the background
	conn.Process(nodeID)

	// process incoming messages for as long as the peer is running
ProcessLoop:
	for {
		select {
		case <-conn.done:
			break ProcessLoop
		case msg := <-conn.inbound:
			err = m.ov.Receive(nodeID, msg)
			if err != nil {
				log.Error().Err(err).Msg("could not receive message")
				continue ProcessLoop
			}
		}
	}

	log.Info().Msg("connection closed")
}

// release will release one resource on the given semaphore.
func (m *Middleware) release(slots chan struct{}) {
	<-slots
}

// add will add the given conn with the given address to our list in a
// concurrency-safe manner.
func (m *Middleware) add(nodeID flow.Identifier, conn *Connection) {
	m.Lock()
	defer m.Unlock()
	m.conns[nodeID] = conn
}

// remove will remove the connection with the given nodeID from the list in
// a concurrency-safe manner.
func (m *Middleware) remove(nodeID flow.Identifier) {
	m.Lock()
	defer m.Unlock()
	delete(m.conns, nodeID)
}
