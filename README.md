# finance-pipeline-gcp

Portfolio MVP: the **Audit-Proof Drop Folder** workflow.

![ci](https://github.com/nicholaskarlson/finance-pipeline-gcp/actions/workflows/ci.yml/badge.svg)
![license](https://img.shields.io/badge/license-MIT-blue.svg)

> **Book:** *Proof-First Pipelines in the Cloud* (Book 2)  
> This repo is the **Anchor (Repo 1 of 4)**. The exact code referenced in the manuscript is tagged **[`book2-v1`](https://github.com/nicholaskarlson/finance-pipeline-gcp/tree/book2-v1)**.

A run is triggered by a Cloud Storage event, downloads two CSVs (**left** + **right**), produces deterministic artifacts, and uploads a verifiable evidence bundle plus a completion marker.

**Go baseline:** 1.22.x (CI witnesses ubuntu/macos/windows on 1.22.x, plus ubuntu “stable”).

## Book 2 suite map

This repo is designed to be used alongside the other Book 2 repos:

- **[finance-pipeline-gcp](https://github.com/nicholaskarlson/finance-pipeline-gcp)** — anchor drop-folder workflow (trigger → run → artifacts → markers)
- **[proof-first-event-contracts](https://github.com/nicholaskarlson/proof-first-event-contracts)** — event parsing contract + fixtures/goldens + expected-fail
- **[proof-first-deploy-gcp](https://github.com/nicholaskarlson/proof-first-deploy-gcp)** — deterministic deploy evidence (render + verify) + fixtures/goldens
- **[proof-first-casefiles](https://github.com/nicholaskarlson/proof-first-casefiles)** — engagement kits you can hand to a client (or use in teaching)

## Quickstart

Run the proof gate:

```bash
make verify
# (optional) Equivalent, if you want to run it directly:
# go test -count=1 ./...
```

`make verify` runs tests and then runs a deterministic demo twice and `diff`s the output trees.

Optional: local server smoke (returns 204 on `{}`):

```bash
PORT=18080 make server-smoke
```

## What this does (conceptually)

You provide two datasets that should mostly agree (for example: bank export vs. ledger export). The pipeline classifies rows into buckets such as:

- **matched**: same record on both sides (by a stable key)
- **left_only**: only found on left
- **right_only**: only found on right
- **mismatched**: key exists on both sides, but one or more fields differ

Everything is generated in a stable order and format so CI (and you) can verify outputs byte-for-byte.

## Drop-folder contract (GCS naming)

The Cloud Run handler triggers only on a finalized object named:

- `in/<run_id>/right.csv`

Where `<run_id>` is intentionally restrictive (alphanumeric plus `-` and `_`) to prevent prefix escape.

When triggered, the service downloads *both* inputs from the input bucket:

- `in/<run_id>/left.csv`
- `in/<run_id>/right.csv`

And uploads outputs to the output bucket prefix:

- `out/<run_id>/...`

A completion marker is always written into the run directory and uploaded as one of:

- `_SUCCESS.json`
- `_ERROR.json`

The service is replay-safe: if either marker already exists at `out/<run_id>/`, the event is ACKed and no work is repeated.

See `docs/CONTRACT.md` for the precise rules.

## Commands

Run the pipeline locally (no cloud):

```bash
go run ./cmd/pipeline run   --left ./fixtures/demo/left.csv   --right ./fixtures/demo/right.csv   --out ./out   --run-id demo
```

This produces:

- `./out/demo/tree/**` (inputs + work outputs + optional `error.txt`)
- `./out/demo/pack/**` (verifiable evidence bundle)

## Docs

- `docs/CONVENTIONS.md` — determinism rules shared across Book 2 repos
- `docs/CONTRACT.md` — trigger rules, replay safety, output layout, failure semantics
- `docs/HANDOFF.md` — what to hand to a client, how to verify later
- `docs/cloud-run.md` — Cloud Run + Eventarc notes

## License

MIT (see `LICENSE`).
