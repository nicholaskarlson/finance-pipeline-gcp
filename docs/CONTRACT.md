# Contract (Anchor repo)

This document defines the **stable, testable contract** for the Book 2 anchor workflow.

If you change any behavior here, treat it as a contract change:
update fixtures/goldens together and let CI be your witness.

---

## 1) Event parsing boundary (Repo A)

This service delegates CloudEvent / Eventarc envelope parsing to:

- **proof-first-event-contracts** (Book 2 Repo A)

Operational rule:

- If the event contract returns **error.txt** (malformed or unsupported envelope), this service **ACKs (204)** and does not run work.
- If the event contract returns a deterministic **ignore** decision, this service **ACKs (204)** and does not run work.
- Only a deterministic **run** decision is eligible to trigger work.

---

## 2) Trigger rule (what starts a run)

A run is triggered only when the (unescaped) object name matches:

- `in/<run_id>/right.csv`

Where:

- `<run_id>` is 1–64 chars
- first char is alphanumeric
- remaining chars are alphanumeric, `-`, or `_`

This restriction prevents prefix escape / path traversal.

The run downloads both inputs from the input bucket:

- `in/<run_id>/left.csv`
- `in/<run_id>/right.csv`

Important: inputs are downloaded from GCS (INPUT_BUCKET), not trusted from the event body.

---

## 3) Replay safety (idempotency)

Before doing any work, the service checks for completion markers in the output bucket:

- `out/<run_id>/_SUCCESS.json`
- `out/<run_id>/_ERROR.json`

If either exists, the event is **ACKed (204)** and no work is repeated.

---

## 4) Work performed (local run directory)

Work is performed in a temp workspace, producing a run directory:

- `<tmp>/out/<run_id>/`

The run directory contains:

- `tree/` — normalized, stable input/work tree
- `pack/` — verifiable evidence bundle produced by the pinned auditpack tool
- `_SUCCESS.json` or `_ERROR.json` — completion marker (see below)

Within `tree/`:

- `tree/inputs/left.csv`
- `tree/inputs/right.csv`
- `tree/work/**` (recon outputs)
- optional: `tree/error.txt` (if recon fails)

If recon fails, the service still produces and verifies a pack; the overall run is marked as an error.

---

## 5) Completion markers (atomic, deterministic)

A completion marker is always written into the run directory before upload:

- `_SUCCESS.json` if the run completed successfully
- `_ERROR.json` if the run failed due to “bad data” (e.g., recon failure)

Marker rules:

- JSON is UTF-8, pretty-printed, and ends with a trailing newline.
- Contains:
  - `run_id`
  - `status`: `"success"` or `"error"`
  - optional `error`: first line only (no volatile paths / multi-line dumps)

Markers are written atomically using a temp file + rename.

---

## 6) Upload rule (what is persisted)

The entire run directory is uploaded to OUTPUT_BUCKET under:

- `out/<run_id>/...`

Including the completion marker and the pack.

Downstream consumers should use the marker presence (`_SUCCESS.json` / `_ERROR.json`) to avoid reading partial outputs.

---

## 7) Failure semantics (when we retry)

- **Internal errors** (token fetch, downloads, uploads, marker write) return **5xx** so the event can be retried.
- **Bad data** (recon failure) returns **204** to avoid retries, and the run is recorded as `_ERROR.json` plus deterministic evidence in `tree/error.txt` (pack still verifies).
- **Event contract errors / ignores** return **204** and do not emit outputs.

---

## 8) Determinism requirements

- No timestamps, UUIDs, random IDs, or host-specific paths embedded into artifacts.
- Stable ordering for file lists and JSON fields.
- LF line endings for text artifacts; `.gitattributes` normalizes checkout to LF.
