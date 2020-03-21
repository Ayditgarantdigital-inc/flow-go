module github.com/dapperlabs/flow-go/integration

go 1.13

require (
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/dapperlabs/flow-go v0.3.1
	github.com/dapperlabs/flow-go-sdk v0.0.0-20200114214559-9ed2f832fcdc
	github.com/docker/docker v1.4.2-0.20190513124817-8c8457b0f2f8
	github.com/docker/go-connections v0.4.0
	github.com/m4ksio/testingdock v0.4.1
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
)

replace github.com/dapperlabs/flow-go => ../

replace github.com/dapperlabs/flow-go/crypto => ../crypto
