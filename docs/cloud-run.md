# Cloud Run + Eventarc (GCS) notes

This service is designed to run on **Cloud Run** and be triggered by **Eventarc**
on **Cloud Storage object finalized** events.

The goal is safety and determinism:
- ignore noise,
- run only when the input contract matches,
- publish artifacts in a stable layout,
- and make retries/idempotency boring.

---

## Buckets and prefixes

The handler uses two buckets:

- `INPUT_BUCKET` — where upstream systems upload input files.
- `OUTPUT_BUCKET` — where the pipeline writes deterministic outputs.

Prefixes are configurable:

- `INPUT_PREFIX` (default `in/`)
- `OUTPUT_PREFIX` (default `out/`)

Safety rule:
- the server **ignores events** whose bucket does not match `INPUT_BUCKET`.

---

## Event parsing + “should we run?” (source of truth)

Cloud Storage events delivered via Eventarc can arrive in more than one JSON shape,
and Eventarc communicates the event type via the `Ce-Type` header
(e.g. `google.cloud.storage.object.v1.finalized`).

Rather than re-implement (and slowly diverge on) these parsing + filtering rules here,
**finance-pipeline-gcp delegates event parsing and run/ignore decisions to**:

- `github.com/nicholaskarlson/proof-first-event-contracts` (pinned in `go.mod`)

For Book 2, we freeze both repos with matching tags (e.g. `book2-v1`), and the manuscript
references the **tagged contract version** as the stable source of truth.

### Contract inputs (Eventarc delivery)

- request header: `Ce-Type`
- request body:
  - direct: `{ "bucket": "...", "name": "..." }`
  - envelope: `{ "data": { "bucket": "...", "name": "..." } }`

### Contract outputs (semantic interface)

The contract produces deterministic semantics via artifacts (and fixtures/goldens in its repo):

- `decision.json` — run vs ignore, with a reason
- `object_ref.json` — bucket/object name (including `name_unescaped` when applicable)
- `error.txt` — expected-fail output for malformed/missing inputs

This repo applies those semantics:
- ignore noise (ACK 204),
- ignore wrong bucket (ACK 204),
- ACK expected-fail (204) to avoid retry storms,
- proceed only on “run”.

---

## What triggers an actual run (extra filter in this repo)

Even when the contract says “run”, this service only triggers the pipeline when the object name matches:

- `in/<run_id>/right.csv`  (using `INPUT_PREFIX`)

`run_id` must be 1–64 chars:
letters/digits, plus `-` and `_` (first char must be alphanumeric).

All other object names are treated as safe no-ops (ACK 204).

Why `right.csv`?
It provides a single, deterministic “completion” signal for upstream uploads:
upstream writes `left.csv` first, then `right.csv` last.

---

## End-to-end flow (handler behavior)

On every HTTP POST from Eventarc:

1. Read request body (capped at 1MiB) and `Ce-Type`.
2. Call the contract: parse + decide with `INPUT_BUCKET` as the bucket guardrail.
3. If contract returns “ignore” or “expected-fail”, **ACK 204** and stop.
4. Parse `run_id` from object name; if not `in/<run_id>/right.csv`, **ACK 204** and stop.
5. Idempotency check:
   - if `out/<run_id>/_SUCCESS.json` exists → ACK 204 and stop
   - if `out/<run_id>/_ERROR.json` exists → ACK 204 and stop
6. Download inputs from `INPUT_BUCKET` (not from the event payload):
   - `in/<run_id>/left.csv`
   - `in/<run_id>/right.csv`
7. Run the pipeline (recon + auditpack) into a temp workspace.
   - On recon failure, write `tree/error.txt` (bad data lane).
   - Always build + verify the audit pack (`pack/`).
8. Write the completion marker into the run directory (`_SUCCESS.json` or `_ERROR.json`).
   - Marker is written atomically (temp → rename).
9. Upload the entire run directory to `OUTPUT_BUCKET` under `out/<run_id>/`.
   - Upload order is deterministic.
   - Completion markers are uploaded **last**.

Response policy:
- For “bad data” (recon failure), the server still returns **204** so Eventarc does not retry.
- The server returns **5xx** only for internal/transient failures (env/config, token fetch, GCS I/O),
  where retry can be useful.

---

## Output layout (stable, book-friendly)

Each run is uploaded to:

- `gs://$OUTPUT_BUCKET/out/<run_id>/...`

Layout:

```
out/<run_id>/
  tree/
    inputs/left.csv
    inputs/right.csv
    work/...
    error.txt              # only on recon failure
  pack/...
  _SUCCESS.json            # terminal marker (uploaded last)
  _ERROR.json              # terminal marker (uploaded last)
```

Downstream consumers should wait for `_SUCCESS.json` or `_ERROR.json`
before reading other outputs to avoid partial runs.

---

## Environment variables

Required:
- `INPUT_BUCKET`
- `OUTPUT_BUCKET`

Optional:
- `PORT` (default `8080`)
- `INPUT_PREFIX` (default `in/`)
- `OUTPUT_PREFIX` (default `out/`)

Optional GCS retry hardening (all optional; reasonable defaults exist):
- `GCS_RETRIES` (default `3`)
- `GCS_TOKEN_TIMEOUT` (default `10s`)
- `GCS_DOWNLOAD_TIMEOUT` (default `60s`)
- `GCS_UPLOAD_TIMEOUT` (default `60s`)
- `GCS_RETRY_BACKOFF` (default `200ms`)
- `GCS_RETRY_MAX_BACKOFF` (default `2s`)

---

## Local smoke

The smoke test starts the server and POSTs `{}`; safe behavior is **204**.

```bash
PORT=18080 make server-smoke
```

The handler treats events that do not match `INPUT_BUCKET` as no-ops,
so you can run the smoke test without GCS credentials.
