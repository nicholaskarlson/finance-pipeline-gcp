# Cloud Run + Eventarc (GCS) notes

This service is designed to run on **Cloud Run** and be triggered by **Eventarc** on **Cloud Storage object finalized** events.

## Buckets and prefixes

The handler intentionally uses two buckets:

- `INPUT_BUCKET` — where upstream systems upload input files.
- `OUTPUT_BUCKET` — where the pipeline writes deterministic outputs.

The handler **ignores events** whose bucket does not match `INPUT_BUCKET`.

## Event parsing and decision rules (source of truth)

Cloud Storage events delivered via **Eventarc** can arrive in more than one JSON shape, and Eventarc communicates the event type via the `Ce-Type` header (for example: `google.cloud.storage.object.v1.finalized`).

Rather than re-implement (and slowly diverge on) these parsing + filtering rules inside this repo, **finance-pipeline-gcp delegates event parsing and “should we run?” decisions to**:

- **proof-first-event-contracts** (the contract repo; tagged snapshots are the book’s stable source of truth)

### What this repo does

On every HTTP POST from Eventarc:

1. Read the raw request body.
2. Call the contract parser/decider (see “Contract outputs” below).
3. If the decision is “ignore” or “error”, **ACK with HTTP 204** and do nothing (so Eventarc doesn’t retry noise).
4. If the decision is “run”, continue the pipeline using the parsed object reference.

### Contract inputs

The contract supports two Eventarc body shapes (both are treated identically):

- **Direct body**: `{ "bucket": "...", "name": "..." }`
- **Envelope body**: `{ "data": { "bucket": "...", "name": "..." } }`

Event type comes from:

- `Ce-Type` request header (ex: `google.cloud.storage.object.v1.finalized`)

### Contract outputs

The contract produces deterministic artifacts:

- `decision.json` — should we run, and why?
- `object_ref.json` — the bucket/object name (including an unescaped name if applicable)
- `error.txt` — expected-fail contract output for malformed / missing inputs

This repo uses those same semantics:

- **Non-finalize events are ignored** (ACK 204).
- **Wrong-bucket events are ignored** (ACK 204).
- **Malformed / unexpected events are ACKed** (204) to avoid retry storms.

### Fixtures (Book 2 friendly)

The contract repo provides the “truth table” as fixtures and goldens, so Book 2 can reference only:

- fixture folder name
- `body.json` and `ce_type.txt` (Eventarc delivery) *(or `event.json` for CloudEvent JSON)*
- expected artifacts: `decision.json`, `object_ref.json`, `error.txt`

See the contract repo README for the exact fixture layout and artifact meanings.

### Expected object layout

Upload the *right* file to:

- `gs://$INPUT_BUCKET/in/<run_id>/right.csv`
  - `run_id` must be 1–64 chars: letters/digits, plus `-` and `_` (first char must be a letter or digit).

The server then downloads:

- `gs://$INPUT_BUCKET/in/<run_id>/left.csv`
- `gs://$INPUT_BUCKET/in/<run_id>/right.csv`

…and writes outputs to:

- `gs://$OUTPUT_BUCKET/out/<run_id>/...`

You can change the prefixes via:

- `INPUT_PREFIX` (default `in/`)
- `OUTPUT_PREFIX` (default `out/`)

## Completion markers

The pipeline writes a small completion marker at the root of each output run:

- `gs://$OUTPUT_BUCKET/out/<run_id>/_SUCCESS.json` on success
- `gs://$OUTPUT_BUCKET/out/<run_id>/_ERROR.json` on failure (pack still uploaded)

Downstream consumers should wait for one of these markers before reading other outputs to avoid partial runs.

Markers are uploaded last, so their presence means other outputs for the run are already visible.

The server is idempotent: if either marker already exists for a run, the event is ACKed (204) and no work is repeated.

## Environment variables

Required:
- `INPUT_BUCKET`
- `OUTPUT_BUCKET`

Optional:
- `PORT` (default `8080`)
- `INPUT_PREFIX` (default `in/`)
- `OUTPUT_PREFIX` (default `out/`)

## Local smoke

Your smoke tests POST `{}` (empty event) and expect `204`.

To ensure the server is safe to run locally without GCS credentials, the handler also treats any event that does **not** match `INPUT_BUCKET` as a no-op.

## GCS client hardening (optional)

The server and pipeline use simple HTTP calls to GCS. You can tune retry/timeouts via env vars:

- `GCS_RETRIES` (default `3`) — total attempts for token/download/upload
- `GCS_TOKEN_TIMEOUT` (default `10s`) — per-attempt timeout for metadata token
- `GCS_DOWNLOAD_TIMEOUT` (default `60s`) — per-attempt timeout for downloads
- `GCS_UPLOAD_TIMEOUT` (default `60s`) — per-attempt timeout for uploads
- `GCS_RETRY_BACKOFF` (default `200ms`) — initial backoff between retries
- `GCS_RETRY_MAX_BACKOFF` (default `2s`) — max backoff between retries

Retries are attempted for `429` and `5xx` responses, plus network timeouts/temporary errors.
