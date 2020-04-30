package main

import (
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	BootstrapDir          = "./bootstrap"
	DockerComposeFile     = "./docker-compose.nodes.yml"
	DefaultConsensusCount = 3
)

var consensusCount int

func init() {
	flag.IntVar(&consensusCount, "consensus", DefaultConsensusCount, "number of consensus nodes")
}

func main() {
	flag.Parse()

	fmt.Printf("Bootstrapping a network with %d consensus nodes...\n", consensusCount)

	nodes := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleExecution),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleAccess),
	}

	for i := 0; i < consensusCount; i++ {
		nodes = append(nodes, testnet.NewNodeConfig(flow.RoleConsensus))
	}

	conf := testnet.NewNetworkConfig("localnet", nodes)

	err := os.RemoveAll(BootstrapDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
	}

	err = os.Mkdir(BootstrapDir, 0755)
	if err != nil {
		panic(err)
	}

	containers, err := testnet.BootstrapNetwork(conf, BootstrapDir)
	if err != nil {
		panic(err)
	}

	err = WriteDockerComposeConfig(containers)
	if err != nil {
		panic(err)
	}

	fmt.Print("Bootstrapping success!\n\n")
	fmt.Print("Run \"make start\" to launch the network.\n")
}

type Network struct {
	Version  string
	Services Services
}

type Services map[string]Service

type Service struct {
	Build struct {
		Context    string
		Dockerfile string
		Args       map[string]string
		Target     string
	}
	Command []string
	Volumes []string
}

func WriteDockerComposeConfig(containers []testnet.ContainerConfig) error {
	f, err := os.OpenFile(DockerComposeFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}

	// overwrite current file contents
	err = f.Truncate(0)
	if err != nil {
		return err
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	services := make(Services)

	for _, container := range containers {
		// only include consensus nodes in Docker Compose network (for now)
		if container.Role != flow.RoleConsensus {
			continue
		}

		services[container.ContainerName] = Service{
			Build: struct {
				Context    string
				Dockerfile string
				Args       map[string]string
				Target     string
			}{
				Context:    "../../",
				Dockerfile: "cmd/Dockerfile",
				Args: map[string]string{
					"TARGET": container.Role.String(),
				},
				Target: "production",
			},
			Command: []string{
				fmt.Sprintf("--nodeid=%s", container.NodeID),
				"--bootstrapdir=/bootstrap",
				"--datadir=/flowdb",
				"--loglevel=DEBUG",
				"--nclusters=1",
			},
			Volumes: []string{
				fmt.Sprintf("%s:/bootstrap", BootstrapDir),
			},
		}
	}

	network := Network{
		Version:  "3.7",
		Services: services,
	}

	enc := yaml.NewEncoder(f)

	err = enc.Encode(&network)
	if err != nil {
		return err
	}

	return nil
}
