COMPOSE := docker compose -f dev/docker-compose.yaml
FILE ?= dev/samples/findings.ndjson
N ?= 0
PREFIX ?= findings
NATS_PORT ?= 4222
export NATS_PORT
NATS_URL ?= nats://localhost:$(NATS_PORT)
NETWORK ?= finding-relay_default

.PHONY: up down test publish smoke logs stream

up:
	$(COMPOSE) up --build -d
	@for url in http://localhost:8081/healthz http://localhost:8082/healthz http://localhost:8080/healthz; do \
		ok=0; \
		for i in $$(seq 1 60); do \
			if curl -fsS $$url >/dev/null 2>&1; then ok=1; break; fi; \
			sleep 1; \
		done; \
		if [ $$ok -ne 1 ]; then echo "timed out waiting for $$url"; exit 1; fi; \
	done
	@echo "stack is up"

down:
	$(COMPOSE) down -v

test:
	go test ./...

publish:
	go run ./cmd/publish -file $(FILE) -n $(N) -prefix $(PREFIX) -nats $(NATS_URL) -user scanner -password scanner-dev-4Qm8xZk2Wp

smoke: up
	curl -fsS -X POST http://localhost:8081/reset
	curl -fsS -X POST http://localhost:8082/reset
	$(MAKE) publish N=25
	@for i in $$(seq 1 30); do \
		if curl -fsS http://localhost:8081/stats | grep -q "\"tickets\":25"; then \
			curl -fsS http://localhost:8081/stats; echo; echo "smoke ok"; exit 0; \
		fi; \
		sleep 1; \
	done; \
	curl -fsS http://localhost:8081/stats; echo; echo "smoke failed: expected 25 tickets"; exit 1

logs:
	$(COMPOSE) logs -f relay

stream:
	docker run --rm --network $(NETWORK) natsio/nats-box nats --server nats://relay:relay-dev-J7vT3nRq9L@nats:4222 stream info FINDINGS
	docker run --rm --network $(NETWORK) natsio/nats-box nats --server nats://relay:relay-dev-J7vT3nRq9L@nats:4222 consumer info FINDINGS finding-relay
