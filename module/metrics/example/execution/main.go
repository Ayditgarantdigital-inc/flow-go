package main

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// main runs a local tracer server on the machine and starts monitoring some metrics for sake of execution, which
// increases result approvals counter and checked chunks counter 100 times each
func main() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		metrics, err := metrics.NewCollector(logger)
		if err != nil {
			panic(err)
		}
		diskTotal := rand.Int63n(1024 ^ 3)
		for i := 0; i < 1000; i++ {
			blockID := unittest.BlockFixture().ID()
			metrics.StartBlockReceivedToExecuted(blockID)

			// adds a random delay for execution duration, between 0 and 2 seconds
			time.Sleep(time.Duration(rand.Int31n(2000)) * time.Millisecond)

			metrics.ExecutionGasUsedPerBlock(uint64(rand.Int63n(1e6)))
			metrics.ExecutionStateReadsPerBlock(uint64(rand.Int63n(1e6)))

			diskIncrease := rand.Int63n(1024 ^ 2)
			diskTotal += diskIncrease
			metrics.ExecutionStateStorageDiskTotal(diskTotal)
			metrics.ExecutionStorageStateCommitment(diskIncrease)

			metrics.FinishBlockReceivedToExecuted(blockID)
		}
	})
}
