#!/usr/bin/env bash
set -euo pipefail

# Run from repo root (finance-pipeline-gcp)
if [ ! -f go.mod ]; then
  echo "error: run from repo root (finance-pipeline-gcp)" >&2
  exit 1
fi

# Remove internal/event package (Option B)
if [ -d internal/event ]; then
  git rm -r internal/event
fi

# Format + verify
gofmt -w internal/server

go test ./...
make verify
PORT=18080 make server-smoke

echo "OK: Option B applied (server calls event-contracts directly; internal/event removed)"
