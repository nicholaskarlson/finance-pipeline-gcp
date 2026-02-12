# Conventions (Book 2)

This repo follows the same “proof-first” conventions used across the Book 2 repos:
deterministic artifacts, stable naming, and idempotent behavior.

If something here changes, treat it as a contract change:
update fixtures/goldens together and let CI be your witness.

---

## Line endings + encoding

- Text artifacts are written with **LF** line endings.
- Repo checkout is normalized to LF via `.gitattributes` (`* text=auto eol=lf`).
- JSON artifacts are UTF-8 and are produced by deterministic encoding paths
  (struct encoding; avoid non-stable map iteration in output-shaping code).

---

## Deterministic ordering rules

If ordering affects an artifact, ordering must be explicit:

- Directory walks that influence outputs must be **sorted** (lexicographic by relative path).
- Cloud uploads are performed in a deterministic order:
  - files are sorted by relative path,
  - completion markers are uploaded **last**.

(See `internal/gcsutil.UploadDir`.)

---

## Run identity + paths

Cloud Run runs are keyed by `run_id` embedded in the object name:

- input objects:
  - `in/<run_id>/left.csv`
  - `in/<run_id>/right.csv`  (the finalize of this file is the trigger)

- output objects:
  - `out/<run_id>/...` (same layout as local outputs)

`run_id` is intentionally restrictive (1–64 chars):
letters/digits, plus `-` and `_` (first char must be alphanumeric).

Local CLI runs (`cmd/pipeline run`) also produce a `run_id`:
- explicit `--run-id <id>`, or
- a deterministic default computed from file contents (sha256 prefix).

---

## Artifact layout + meaning

Each run produces a stable folder layout:

```
out/<run_id>/
  tree/
    inputs/
      left.csv
      right.csv
    work/
      ... (recon outputs)
    error.txt              # only if recon fails (bad data lane)
  pack/
    ... (auditpack outputs; verifiable)
  _SUCCESS.json            # terminal marker (success)
  _ERROR.json              # terminal marker (failure)
```

Meaning:

- `tree/inputs/*` are the exact inputs used (copied to stable names).
- `tree/work/*` contains recon outputs.
- `tree/error.txt` exists only on recon failure and records deterministic evidence.
- `pack/` is produced by `auditpack run --in tree --out pack` and must pass `auditpack verify`.
- `_SUCCESS.json` / `_ERROR.json` are terminal markers:
  - written/uploaded **after** all other artifacts,
  - safe “done” signals for downstream consumers.

Completion marker contents are intentionally minimal and stable:
they include `run_id`, status, and a short error summary (first line only).

---

## Idempotency behavior (server)

The Cloud Run handler is designed to be safe under retries and duplicates:

- If `_SUCCESS.json` or `_ERROR.json` already exists for `out/<run_id>/`,
  the server ACKs the event (204) and does nothing.
- Duplicate deliveries are safe.
- “Noise” events (non-finalize, wrong bucket, wrong object path) are ignored (204).

---

## Event filtering contract (source of truth)

Event parsing and decision rules (finalize vs ignore, bucket guardrails, name unescaping,
expected-fail behavior) are delegated to:

- `github.com/nicholaskarlson/proof-first-event-contracts` (pinned in `go.mod`)

This repo consumes the contract’s decision + object reference and focuses on:
pipeline execution + deterministic artifact publication.

For Book 2, we freeze the contract repo and this repo with matching tags (e.g. `book2-v1`),
so the manuscript references tags, not moving commits.
