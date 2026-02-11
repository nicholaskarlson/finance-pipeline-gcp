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

The server then downloads:

- `gs://$INPUT_BUCKET/in/<run_id>/left.csv`
- `gs://$INPUT_BUCKET/in/<run_id>/right.csv`

…and writes outputs to:

- `gs://$OUTPUT_BUCKET/out/<run_id>/...`

You can change the prefixes via:

- `INPUT_PREFIX` (default `in/`)
- `OUTPUT_PREFIX` (default `out/`)

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
