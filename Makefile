SHELL := /usr/bin/env bash
.RECIPEPREFIX := >

GO ?= go
GOBIN ?= $(shell $(GO) env GOPATH)/bin

RECON ?= $(GOBIN)/recon
AUDITPACK ?= $(GOBIN)/auditpack

.PHONY: tools demo verify clean

tools:
> @echo "Installing pinned book snapshot tools..."
> $(GO) install github.com/nicholaskarlson/proof-first-recon/cmd/recon@book-v1
> $(GO) install github.com/nicholaskarlson/proof-first-auditpack/cmd/auditpack@book-v1

demo: tools
> rm -rf ./out/demo1 ./out/demo2
> mkdir -p ./out
> $(GO) run ./cmd/pipeline run \
>   --left ./fixtures/demo/left.csv \
>   --right ./fixtures/demo/right.csv \
>   --out ./out/demo1 \
>   --run-id demo \
>   --recon "$(RECON)" \
>   --auditpack "$(AUDITPACK)"
> $(GO) run ./cmd/pipeline run \
>   --left ./fixtures/demo/left.csv \
>   --right ./fixtures/demo/right.csv \
>   --out ./out/demo2 \
>   --run-id demo \
>   --recon "$(RECON)" \
>   --auditpack "$(AUDITPACK)"
> diff -qr ./out/demo1 ./out/demo2
> "$(AUDITPACK)" verify --pack ./out/demo1/demo/pack
> @echo "OK: demo is deterministic and pack verifies."

verify:
> $(GO) test -count=1 ./...
> $(MAKE) demo

clean:
> rm -rf ./out

.PHONY: demo-bad

demo-bad: tools
> rm -rf ./out/bad1 ./out/bad2
> mkdir -p ./out
> bash -ceu ' \
>   set +e; \
>   $(GO) run ./cmd/pipeline run \
>     --left ./fixtures/bad/left.csv \
>     --right ./fixtures/bad/right.csv \
>     --out ./out/bad1 \
>     --run-id baddemo \
>     --recon "$(RECON)" \
>     --auditpack "$(AUDITPACK)"; \
>   rc=$$?; \
>   test $$rc -ne 0; \
>   test -f ./out/bad1/baddemo/tree/error.txt; \
>   "$(AUDITPACK)" verify --pack ./out/bad1/baddemo/pack; \
> '
> bash -ceu ' \
>   set +e; \
>   $(GO) run ./cmd/pipeline run \
>     --left ./fixtures/bad/left.csv \
>     --right ./fixtures/bad/right.csv \
>     --out ./out/bad2 \
>     --run-id baddemo \
>     --recon "$(RECON)" \
>     --auditpack "$(AUDITPACK)"; \
>   rc=$$?; \
>   test $$rc -ne 0; \
>   test -f ./out/bad2/baddemo/tree/error.txt; \
>   "$(AUDITPACK)" verify --pack ./out/bad2/baddemo/pack; \
> '
> diff -qr ./out/bad1 ./out/bad2
> @echo "OK: bad lane is deterministic (error.txt + verifiable pack)."
