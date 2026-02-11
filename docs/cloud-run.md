# Cloud Run + Eventarc (GCS) notes

This service is designed to run on **Cloud Run** and be triggered by **Eventarc** on **Cloud Storage object finalized** events.

## Buckets and prefixes

The handler intentionally uses two buckets:

- `INPUT_BUCKET` — where upstream systems upload input files.
- `OUTPUT_BUCKET` — where the pipeline writes deterministic outputs.

The handler **ignores events** whose bucket does not match `INPUT_BUCKET`.

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
