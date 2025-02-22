COLLECTION = 3
CONSENSUS = 3
EXECUTION = 2
VERIFICATION = 1
ACCESS = 1
NCLUSTERS=1
PROFILER=false
CONSENSUS_DELAY=800ms
COLLECTION_DELAY=950ms

.PHONY: init
init:
	go run -tags relic bootstrap.go \
		-collection=$(COLLECTION) \
		-consensus=$(CONSENSUS) \
		-execution=$(EXECUTION) \
		-verification=$(VERIFICATION) \
		-access=$(ACCESS) \
		-nclusters=$(NCLUSTERS) \
		-profiler=$(PROFILER) \
		-consensus-delay=$(CONSENSUS_DELAY) \
		-collection-delay=$(COLLECTION_DELAY) \

# Creates a light version of the localnet with just 1 instance for each node type
.PHONY: init-light
init-light:
	$(MAKE) -e COLLECTION=1 CONSENSUS=1 EXECUTION=1 VERIFICATION=1 ACCESS=1 NCLUSTERS=1 init

.PHONY: start
start:
	docker-compose -f docker-compose.metrics.yml up -d
	DOCKER_BUILDKIT=1 COMPOSE_DOCKER_CLI_BUILD=1 docker-compose -f docker-compose.nodes.yml up --build -d

.PHONY: logs
logs:
	docker-compose -f docker-compose.nodes.yml logs -f

.PHONY: stop
stop:
	docker-compose -f docker-compose.metrics.yml -f docker-compose.nodes.yml -f docker-compose.logs.yml down -v

.PHONY: load
load:
	go run --tags relic ../loader -log-level info -tps 1,0,10,0,100 -tps-durations 30s,10s,30s,10s
