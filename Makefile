SHELL := /usr/bin/env bash

GO ?= go
GOBIN ?= $(shell $(GO) env GOPATH)/bin

RECON ?= $(GOBIN)/recon
AUDITPACK ?= $(GOBIN)/auditpack

.PHONY: tools demo verify clean

tools:
	@echo "Installing pinned book snapshot tools..."
	$(GO) install github.com/nicholaskarlson/proof-first-recon/cmd/recon@book-v1
	$(GO) install github.com/nicholaskarlson/proof-first-auditpack/cmd/auditpack@book-v1

demo: tools
	rm -rf ./out/demo1 ./out/demo2
	mkdir -p ./out
	$(GO) run ./cmd/pipeline run \
	  --left ./fixtures/demo/left.csv \
	  --right ./fixtures/demo/right.csv \
	  --out ./out/demo1 \
	  --run-id demo \
	  --recon "$(RECON)" \
	  --auditpack "$(AUDITPACK)"
	$(GO) run ./cmd/pipeline run \
	  --left ./fixtures/demo/left.csv \
	  --right ./fixtures/demo/right.csv \
	  --out ./out/demo2 \
	  --run-id demo \
	  --recon "$(RECON)" \
	  --auditpack "$(AUDITPACK)"
	diff -qr ./out/demo1 ./out/demo2
	"$(AUDITPACK)" verify --pack ./out/demo1/demo/pack
	@echo "OK: demo is deterministic and pack verifies."

verify:
	$(GO) test -count=1 ./...
	$(MAKE) demo

clean:
	rm -rf ./out

.PHONY: demo-bad

demo-bad: tools
	rm -rf ./out/bad1 ./out/bad2
	mkdir -p ./out
	bash -ceu ' \
	  set +e; \
	  $(GO) run ./cmd/pipeline run \
	    --left ./fixtures/bad/left.csv \
	    --right ./fixtures/bad/right.csv \
	    --out ./out/bad1 \
	    --run-id baddemo \
	    --recon "$(RECON)" \
	    --auditpack "$(AUDITPACK)"; \
	  rc=$$?; \
	  test $$rc -ne 0; \
	  test -f ./out/bad1/baddemo/tree/error.txt; \
	  "$(AUDITPACK)" verify --pack ./out/bad1/baddemo/pack; \
	'
	bash -ceu ' \
	  set +e; \
	  $(GO) run ./cmd/pipeline run \
	    --left ./fixtures/bad/left.csv \
	    --right ./fixtures/bad/right.csv \
	    --out ./out/bad2 \
	    --run-id baddemo \
	    --recon "$(RECON)" \
	    --auditpack "$(AUDITPACK)"; \
	  rc=$$?; \
	  test $$rc -ne 0; \
	  test -f ./out/bad2/baddemo/tree/error.txt; \
	  "$(AUDITPACK)" verify --pack ./out/bad2/baddemo/pack; \
	'
	diff -qr ./out/bad1 ./out/bad2
	@echo "OK: bad lane is deterministic (error.txt + verifiable pack)."

IMAGE ?= finance-pipeline-gcp:local
UID_GID := $(shell id -u):$(shell id -g)

.PHONY: docker-build docker-demo docker-demo-bad

docker-build:
	docker build -t "$(IMAGE)" .

docker-demo: docker-build
	rm -rf ./out/docker1 ./out/docker2
	mkdir -p ./out
	docker run --rm -u "$(UID_GID)" -v "$(PWD):/work" -w /work "$(IMAGE)" run \
	  --left ./fixtures/demo/left.csv \
	  --right ./fixtures/demo/right.csv \
	  --out ./out/docker1 \
	  --run-id demo
	docker run --rm -u "$(UID_GID)" -v "$(PWD):/work" -w /work "$(IMAGE)" run \
	  --left ./fixtures/demo/left.csv \
	  --right ./fixtures/demo/right.csv \
	  --out ./out/docker2 \
	  --run-id demo
	diff -qr ./out/docker1 ./out/docker2
	docker run --rm -u "$(UID_GID)" -v "$(PWD):/work" -w /work "$(IMAGE)" run \
	  --left ./fixtures/demo/left.csv \
	  --right ./fixtures/demo/right.csv \
	  --out ./out/docker1 \
	  --run-id demo
	@echo "OK: docker good lane deterministic."

docker-demo-bad: docker-build
	rm -rf ./out/dockerbad1 ./out/dockerbad2
	mkdir -p ./out
	bash -ceu ' \
	  set +e; \
	  docker run --rm -u "$(UID_GID)" -v "$(PWD):/work" -w /work "$(IMAGE)" run \
	    --left ./fixtures/bad/left.csv \
	    --right ./fixtures/bad/right.csv \
	    --out ./out/dockerbad1 \
	    --run-id baddemo; \
	  rc=$$?; \
	  test $$rc -ne 0; \
	  test -f ./out/dockerbad1/baddemo/tree/error.txt; \
	'
	bash -ceu ' \
	  set +e; \
	  docker run --rm -u "$(UID_GID)" -v "$(PWD):/work" -w /work "$(IMAGE)" run \
	    --left ./fixtures/bad/left.csv \
	    --right ./fixtures/bad/right.csv \
	    --out ./out/dockerbad2 \
	    --run-id baddemo; \
	  rc=$$?; \
	  test $$rc -ne 0; \
	  test -f ./out/dockerbad2/baddemo/tree/error.txt; \
	'
	diff -qr ./out/dockerbad1 ./out/dockerbad2
	@echo "OK: docker bad lane deterministic (error.txt + verifiable pack)."


.PHONY: server-smoke docker-server-smoke

# Local smoke: start server, POST `{}`, expect 204
server-smoke:
	@echo "Running local server smoke test (expects 204 on empty event)..."
	bash -ceu ' \
	  export OUTPUT_BUCKET="$${OUTPUT_BUCKET:-dummy-bucket}"; \
	  export INPUT_BUCKET="$${INPUT_BUCKET:-dummy-in-bucket}"; \
	  export PORT="$${PORT:-8080}"; \
	  ( $(GO) run ./cmd/pipeline server >/tmp/pipeline-server.log 2>&1 & echo $$! > /tmp/pipeline-server.pid ); \
	  pid=$$(cat /tmp/pipeline-server.pid); \
	  trap "kill $$pid >/dev/null 2>&1 || true" EXIT; \
	  for i in {1..150}; do \
	    code=$$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://localhost:$$PORT" -d "{}" || true); \
	    [ "$$code" = "204" ] && { echo "OK: local server returned 204"; exit 0; }; \
	    sleep 0.2; \
	  done; \
	  echo "Server did not return 204. Last logs:"; \
	  tail -n 120 /tmp/pipeline-server.log; \
	  exit 1; \
	'

# Docker smoke: run container server, POST `{}`, expect 204
docker-server-smoke: docker-build
	@echo "Running docker server smoke test (expects 204 on empty event)..."
	bash -ceu ' \
	  host_port="$${DOCKER_HOST_PORT:-18080}"; \
	  container_port="$${DOCKER_CONTAINER_PORT:-8080}"; \
	  docker rm -f fpgcp-smoke >/dev/null 2>&1 || true; \
	  docker run -d --name fpgcp-smoke -p "$$host_port:$$container_port" \
	    -e OUTPUT_BUCKET="$${OUTPUT_BUCKET:-dummy-bucket}" \
	    -e INPUT_BUCKET="$${INPUT_BUCKET:-dummy-in-bucket}" \
	    -e PORT="$$container_port" \
	    finance-pipeline-gcp:local server >/dev/null; \
	  trap "docker rm -f fpgcp-smoke >/dev/null 2>&1 || true" EXIT; \
	  for i in {1..150}; do \
	    code=$$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://localhost:$$host_port" -d "{}" || true); \
	    [ "$$code" = "204" ] && { echo "OK: docker server returned 204"; exit 0; }; \
	    sleep 0.2; \
	  done; \
	  echo "Container did not return 204. Logs:"; \
	  docker logs --tail 120 fpgcp-smoke; \
	  exit 1; \
	'


