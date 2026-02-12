# Conventions (Book 2)

This repo follows the same “proof-first” conventions used across the Book 2 repos: deterministic artifacts, stable naming, and idempotent behavior.

## Line endings + encoding
- Text artifacts are written with **LF** line endings.
- JSON is UTF-8, stable field ordering is achieved by stable struct encoding + deterministic inputs.

## Deterministic ordering
- Any directory/file walks that influence outputs must be **sorted** (lexicographic).
- Upload order is deterministic; completion markers are uploaded **last**.

## Run identity + paths
- Events refer to objects under the input prefix: `in/<run_id>/...`
- Outputs are written under the output prefix: `out/<run_id>/...`
- `run_id` is sanitized to a conservative charset and used as a folder name.

## Completion markers
The pipeline writes one of these markers to the run output folder:
- `_SUCCESS.json` — the run completed and outputs are final
- `_ERROR.json` — the run failed (used to stop retries / mark terminal failure)

Markers are uploaded **after** all other run artifacts, so their presence is a reliable “done” signal.

## Idempotency behavior
- If a completion marker already exists for a run, the server ACKs (204) and does nothing.
- Duplicate Eventarc deliveries are safe (idempotent).

## Event filtering contract (source of truth)
Event parsing and decision rules (finalize vs ignore, object name unescaping, bucket guardrails) are delegated to:

- `github.com/nicholaskarlson/proof-first-event-contracts` (tagged)

This repo consumes the contract’s outputs (decision + object_ref) and focuses on the pipeline run + artifact publication.
