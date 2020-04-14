package integration

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSingleInstance(t *testing.T) {

	// set up a single instance to run4
	// NOTE: currently, the HotStuff logic will infinitely call back on itself
	// with a single instance, leading to a boundlessly growing call stack,
	// which slows down the mocks significantly due to splitting the callstack
	// to find the calling function name; we thus limit it to 128 for now
	finalView := uint64(128)
	in := NewInstance(t,
		WithStopCondition(ViewReached(finalView)),
	)

	// start and check we have reached stop condition
	err := in.loop.Start()
	require.True(t, errors.Is(err, errStopCondition))

	// check if forks and pacemaker are in expected view state
	assert.Equal(t, finalView-3, in.forks.FinalizedView(), "finalized view should be three lower than current view")
}

func TestThreeInstances(t *testing.T) {

	// test parameters
	// NOTE: block finalization seems to be rather slow on CI at the moment,
	// needing around 1 minute on Travis for 1000 blocks and 10 minutes on
	// TeamCity for 1000 blocks; in order to avoid test timeouts, we keep the
	// number low here
	num := 3
	finalView := uint64(128)

	// generate three hotstuff participants
	participants := unittest.IdentityListFixture(num)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(10*time.Second, 10*time.Second, 0.5, 1.5, 1*time.Second)
	require.NoError(t, err)

	// set up three instances that are exactly the same
	instances := make([]*Instance, 0, num)
	for n := 0; n < num; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(instances)

	// start all three instances and wait for them to wrap up
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.loop.Start()
			require.True(t, errors.Is(err, errStopCondition))
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	in1 := instances[0]
	in2 := instances[1]
	in3 := instances[2]
	assert.Equal(t, in1.forks.FinalizedBlock(), in2.forks.FinalizedBlock(), "second instance should have same finalized block as first instance")
	assert.Equal(t, in1.forks.FinalizedBlock(), in3.forks.FinalizedBlock(), "third instance should have same finalized block as first instance")
}

func TestSevenInstances(t *testing.T) {

	// test parameters
	// NOTE: block finalization seems to be rather slow on CI at the moment,
	// needing around 1 minute on Travis for 1000 blocks and 10 minutes on
	// TeamCity for 1000 blocks; in order to avoid test timeouts, we keep the
	// number low here
	numPass := 5
	numFail := 2
	finalView := uint64(128)

	// generate the seven hotstuff participants
	participants := unittest.IdentityListFixture(numPass + numFail)
	instances := make([]*Instance, 0, numPass+numFail)
	root := DefaultRoot()
	timeouts, err := timeout.NewConfig(10*time.Second, 10*time.Second, 0.5, 1.5, 1*time.Second)
	require.NoError(t, err)

	// set up five instances that work fully
	for n := 0; n < numPass; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
		)
		instances = append(instances, in)
	}

	// set up two instances which can't vote
	for n := numPass; n < numPass+numFail; n++ {
		in := NewInstance(t,
			WithRoot(root),
			WithParticipants(participants),
			WithLocalID(participants[n].NodeID),
			WithTimeouts(timeouts),
			WithStopCondition(ViewReached(finalView)),
			WithOutgoingVotes(BlockAllVotes),
		)
		instances = append(instances, in)
	}

	// connect the communicators of the instances together
	Connect(instances)

	// start all seven instances and wait for them to wrap up
	var wg sync.WaitGroup
	for _, in := range instances {
		wg.Add(1)
		go func(in *Instance) {
			err := in.loop.Start()
			require.True(t, errors.Is(err, errStopCondition))
			wg.Done()
		}(in)
	}
	wg.Wait()

	// check that all instances have the same finalized block
	ref := instances[0]
	for i := 1; i < len(instances); i++ {
		assert.Equal(t, ref.forks.FinalizedBlock(), instances[i].forks.FinalizedBlock(), "instance %d should have same finalized block as first instance")
	}
}
