package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func main() {

	cmd.
		FlowNode("execution").
		Component("execution engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {

			node.Logger.Info().Msg("initializing execution engine")

			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)

			levelDB, err := leveldb.NewLevelDB("db/valuedb", "db/triedb")
			node.MustNot(err).Msg("could not initialize LevelDB databases")

			ls, err := ledger.NewTrieStorage(levelDB)
			node.MustNot(err).Msg("could not initialize ledger trie storage")

			execState := state.NewExecutionState(ls)

			blockExec := executor.NewBlockExecutor(vm, execState)

			// TODO: replace mock with real implementation
			collections := &storage.Collections{}

			engine, err := execution.New(
				node.Logger,
				node.Network,
				node.Me,
				collections,
				blockExec,
			)
			node.MustNot(err).Msg("could not initialize execution engine")
			return engine
		}).Run()

}
