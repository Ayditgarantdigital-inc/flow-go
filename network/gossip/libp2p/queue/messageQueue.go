package queue

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
)

// MessageQueue is the interface of the inbound message queue
type MessageQueue interface {
	// Insert inserts the message in queue
	Insert(message interface{}) error
	// Remove removes the message from the queue in priority order. If no message is found, this call blocks.
	// If two messages have the same priority, items are de-queued in insertion order
	Remove() interface{}
	// Len gives the current length of the queue
	Len() int
}

type Priority int

const LowPriority = Priority(1)
const MediumPriority = Priority(5)
const HighPriority = Priority(10)

// MessagePriorityFunc - the callback function to derive priority of a message
type MessagePriorityFunc func(message interface{}) (Priority, error)

// MessageQueueImpl is the heap based priority queue implementation of the MessageQueue implementation
type MessageQueueImpl struct {
	pq           *priorityQueue
	cond         *sync.Cond
	priorityFunc MessagePriorityFunc
	ctx          context.Context
}

func (mq *MessageQueueImpl) Insert(message interface{}) error {

	if err := mq.ctx.Err(); err != nil {
		return err
	}

	// determine the message priority
	priority, err := mq.priorityFunc(message)
	if err != nil {
		return fmt.Errorf("failed to dervie message priority: %w", err)
	}

	// create the queue item
	item := &item{
		message:  message,
		priority: int(priority),
	}

	// lock the underlying mutex
	mq.cond.L.Lock()

	// push message to the underlying priority queue
	heap.Push(mq.pq, item)

	// signal a waiting routine that a message is now available
	mq.cond.Signal()

	// unlock the underlying mutex
	mq.cond.L.Unlock()

	return nil
}

func (mq *MessageQueueImpl) Remove() interface{} {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	for mq.pq.Len() == 0 {

		// if the context has been canceled, don't wait
		if err := mq.ctx.Err(); err != nil {
			return nil
		}

		mq.cond.Wait()
	}
	return heap.Pop(mq.pq).(*item).message
}

func (mq *MessageQueueImpl) Len() int {
	mq.cond.L.Lock()
	defer mq.cond.L.Unlock()
	return mq.pq.Len()
}

func NewMessageQueue(ctx context.Context, priorityFunc MessagePriorityFunc) *MessageQueueImpl {
	var items = make([]*item, 0)
	pq := priorityQueue(items)
	mq := &MessageQueueImpl{
		pq:           &pq,
		priorityFunc: priorityFunc,
		ctx:          ctx,
	}
	m := sync.Mutex{}
	mq.cond = sync.NewCond(&m)

	// kick off a go routine to unblock queue readers on shutdown
	go func() {
		<-ctx.Done()
		// unblock receive
		mq.cond.Broadcast()
	}()

	return mq
}
