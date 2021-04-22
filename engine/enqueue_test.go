package engine_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/fifoqueue"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEngine tests the integration of MessageHandler and FifoQueue that buffer and deliver
// matched messages to corresponding handlers
type TestEngine struct {
	unit           *engine.Unit
	log            zerolog.Logger
	notify         <-chan struct{} // when new message is added to queue
	ready          sync.WaitGroup
	messageHandler *engine.MessageHandler
	queueA         *fifoqueue.FifoQueue
	queueB         *fifoqueue.FifoQueue

	mu       sync.RWMutex
	messages []interface{}
}

type messageA struct {
	n int
}

type messageB struct {
	n int
}

type messageC struct {
	s string
}

func NewEngine(log zerolog.Logger) (*TestEngine, error) {
	queueA, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(10),
		fifoqueue.WithLengthObserver(func(len int) {}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue A: %w", err)
	}

	queueB, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(10),
		fifoqueue.WithLengthObserver(func(len int) {}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue B: %w", err)
	}

	// define message queueing behaviour
	handler, notify := engine.NewMessageHandler(
		log,
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				switch msg.Payload.(type) {
				case *messageA:
					return true
				default:
					return false
				}
			},
			Store: queueA,
		},
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				switch msg.Payload.(type) {
				case *messageB:
					return true
				default:
					return false
				}
			},
			Map: func(msg *engine.Message) *engine.Message {
				b := msg.Payload.(*messageB)
				return &engine.Message{
					OriginID: msg.OriginID,
					Payload: &messageC{
						s: fmt.Sprintf("c-%v", b.n),
					},
				}
			},
			Store: queueB,
		},
	)

	eng := &TestEngine{
		unit:           engine.NewUnit(),
		log:            log,
		messageHandler: handler,
		notify:         notify,
		queueA:         queueA,
		queueB:         queueB,
	}

	return eng, nil
}

func (e *TestEngine) Process(originID flow.Identifier, event interface{}) error {
	return e.messageHandler.Process(originID, event)
}

func (e *TestEngine) Ready() <-chan struct{} {
	e.ready.Add(1)
	e.unit.Launch(e.loop)
	return e.unit.Ready(func() {
		e.ready.Wait()
	})
}

func (e *TestEngine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *TestEngine) loop() {
	e.ready.Done()

	for {
		select {
		case <-e.unit.Quit():
			return
		case <-e.notify:
			e.log.Info().Msg("new message arrived")
			err := e.processAvailableMessages()
			if err != nil {
				e.log.Fatal().Err(err).Msg("internal error processing message from the fifo queue")
			}
			e.log.Info().Msg("message processed")
		}
	}
}

func (e *TestEngine) processAvailableMessages() error {
	for {
		msg, ok := e.queueA.Get()
		if ok {
			err := e.OnProcessA(msg.OriginID, msg.Payload.(*messageA))
			if err != nil {
				return fmt.Errorf("could not handle block proposal: %w", err)
			}
			continue
		}

		msg, ok = e.queueB.Get()
		if ok {
			err := e.OnProcessC(msg.OriginID, msg.Payload.(*messageC))
			if err != nil {
				return fmt.Errorf("could not handle block vote: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}

}

func (e *TestEngine) OnProcessA(originID flow.Identifier, msg *messageA) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.messages = append(e.messages, msg)
	return nil
}

func (e *TestEngine) OnProcessC(originID flow.Identifier, msg *messageC) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.messages = append(e.messages, msg)
	return nil
}

func (e *TestEngine) MessageCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.messages)
}

func WithEngine(t *testing.T, f func(*TestEngine)) {
	lg := unittest.Logger()
	eng, err := NewEngine(lg)
	require.NoError(t, err)
	<-eng.Ready()
	f(eng)
	<-eng.Done()
}

// if TestEngine receives messages of same type, the engine will handle them message
func TestProcessMessageSameType(t *testing.T) {
	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()
	m1 := &messageA{n: 1}
	m2 := &messageA{n: 2}
	m3 := &messageA{n: 3}
	m4 := &messageA{n: 4}

	WithEngine(t, func(eng *TestEngine) {
		require.NoError(t, eng.Process(id1, m1))
		require.NoError(t, eng.Process(id2, m2))
		require.NoError(t, eng.Process(id1, m3))
		require.NoError(t, eng.Process(id2, m4))

		require.Eventuallyf(t, func() bool {
			return len(eng.messages) == 4
		}, 2*time.Second, 10*time.Millisecond, "expect %v messages, but go %v messages",
			4, eng.messages)

		require.Equal(t, m1, eng.messages[0])
		require.Equal(t, m2, eng.messages[1])
		require.Equal(t, m3, eng.messages[2])
		require.Equal(t, m4, eng.messages[3])
	})
}

// if TestEngine receives messages of different types, the engine will handle them message
func TestProcessMessageDifferentType(t *testing.T) {
	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()
	m1 := &messageA{n: 1}
	m2 := &messageA{n: 2}
	m3 := &messageB{n: 3}
	m4 := &messageB{n: 4}

	WithEngine(t, func(eng *TestEngine) {
		require.NoError(t, eng.Process(id1, m1))
		require.NoError(t, eng.Process(id2, m2))
		require.NoError(t, eng.Process(id1, m3))
		require.NoError(t, eng.Process(id2, m4))

		require.Eventuallyf(t, func() bool {
			return len(eng.messages) == 4
		}, 2*time.Second, 10*time.Millisecond, "expect %v messages, but go %v messages",
			4, eng.messages)

		require.Equal(t, m1, eng.messages[0])
		require.Equal(t, m2, eng.messages[1])
		require.Equal(t, &messageC{s: "c-3"}, eng.messages[2])
		require.Equal(t, &messageC{s: "c-4"}, eng.messages[3])
	})
}

// if TestEngine receives messages in a period of time, they all will be handled
func TestProcessMessageInterval(t *testing.T) {
	id1 := unittest.IdentifierFixture()
	id2 := unittest.IdentifierFixture()
	m1 := &messageA{n: 1}
	m2 := &messageA{n: 2}
	m3 := &messageA{n: 3}
	m4 := &messageA{n: 4}

	WithEngine(t, func(eng *TestEngine) {
		require.NoError(t, eng.Process(id1, m1))
		time.Sleep(3 * time.Millisecond)
		require.NoError(t, eng.Process(id2, m2))
		time.Sleep(3 * time.Millisecond)
		require.NoError(t, eng.Process(id1, m3))
		time.Sleep(3 * time.Millisecond)
		require.NoError(t, eng.Process(id2, m4))

		require.Eventuallyf(t, func() bool {
			return len(eng.messages) == 4
		}, 2*time.Second, 10*time.Millisecond, "expect %v messages, but go %v messages",
			4, eng.messages)

		require.Equal(t, m1, eng.messages[0])
		require.Equal(t, m2, eng.messages[1])
		require.Equal(t, m3, eng.messages[2])
		require.Equal(t, m4, eng.messages[3])
	})
}

// if TestEngine receives 100 messages for each type, the engine will handle them all
func TestProcessMessageMultiAll(t *testing.T) {

	WithEngine(t, func(eng *TestEngine) {
		count := 100
		for i := 0; i < count; i++ {
			require.NoError(t, eng.Process(unittest.IdentifierFixture(), &messageA{n: i}))
		}

		require.Eventuallyf(t, func() bool {
			return eng.MessageCount() == count
		}, 2*time.Second, 10*time.Millisecond, "expect %v messages, but go %v messages",
			count, eng.MessageCount())
	})
}

// if TestEngine receives 100 messages for each type with interval, the engine will handle them all
func TestProcessMessageMultiInterval(t *testing.T) {

	WithEngine(t, func(eng *TestEngine) {
		count := 100
		for i := 0; i < count; i++ {
			time.Sleep(1 * time.Millisecond)
			require.NoError(t, eng.Process(unittest.IdentifierFixture(), &messageB{n: i}))
		}

		require.Eventuallyf(t, func() bool {
			return eng.MessageCount() == count
		}, 2*time.Second, 10*time.Millisecond, "expect %v messages, but go %v messages",
			count, eng.MessageCount())
	})
}

// if TestEngine receives 100 messages for each type concurrently, the engine will handle them all one after another
func TestProcessMessageMultiConcurrent(t *testing.T) {

	WithEngine(t, func(eng *TestEngine) {
		count := 100
		var sent sync.WaitGroup
		for i := 0; i < count; i++ {
			sent.Add(1)
			go func(i int) {
				require.NoError(t, eng.Process(unittest.IdentifierFixture(), &messageA{n: i}))
				sent.Done()
			}(i)
		}
		sent.Wait()

		require.Eventuallyf(t, func() bool {
			return eng.MessageCount() == count
		}, 2*time.Second, 10*time.Millisecond, "expect %v messages, but go %v messages",
			count, eng.MessageCount())
	})
}
