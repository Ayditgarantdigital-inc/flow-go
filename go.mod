module github.com/dapperlabs/flow-go

go 1.13

require (
	github.com/dapperlabs/flow-go/crypto v0.2.5-beta
	github.com/dapperlabs/flow-go/language v0.2.5-beta
	github.com/dapperlabs/flow-go/protobuf v0.2.5-beta
	github.com/dchest/siphash v1.2.1
	github.com/dgraph-io/badger/v2 v2.0.0
	github.com/ethereum/go-ethereum v1.9.9
	github.com/gammazero/deque v0.0.0-20190521012701-46e4ffb7a622
	github.com/go-test/deep v1.0.4
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/ipfs/go-log v0.0.1
	github.com/jrick/bitset v1.0.0
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8
	github.com/libp2p/go-libp2p v0.4.2
	github.com/libp2p/go-libp2p-core v0.2.5
	github.com/libp2p/go-libp2p-pubsub v0.2.5
	github.com/libp2p/go-tcp-transport v0.1.1
	github.com/magiconair/properties v1.8.1
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.1
	github.com/rs/zerolog v1.15.0
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc
	go.uber.org/atomic v1.4.0
	golang.org/x/crypto v0.0.0-20191219195013-becbf705a915
	google.golang.org/grpc v1.26.0
	zombiezen.com/go/capnproto2 v2.17.0+incompatible
)

replace mellium.im/sasl => github.com/mellium/sasl v0.2.1

replace github.com/dapperlabs/flow-go => ./

replace github.com/dapperlabs/flow-go/language => ./language

replace github.com/dapperlabs/flow-go/crypto => ./crypto

replace github.com/dapperlabs/flow-go/protobuf => ./protobuf
