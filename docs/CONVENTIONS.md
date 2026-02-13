# Conventions

These conventions exist so the repo can *prove* its outputs: deterministic artifacts + fixtures + goldens + a verification gate.

## Line endings
- **LF only** (enforced via `.gitattributes`).

## Determinism
Artifacts must be deterministic:
- stable ordering (no filesystem walk order dependence)
- stable formatting (JSON indentation + trailing newline where applicable)
- no timestamps, UUIDs, random IDs, or host-specific paths embedded into artifacts

## Atomic writes
Artifacts are written via temp file → rename so partial files never appear.

## Upload ordering
Cloud uploads are performed in a deterministic order:
- files are sorted by relative path
- completion markers are uploaded **last**

(See `internal/gcsutil.UploadDir`.)

## Failure evidence + markers
This repo records outcomes as deterministic, reviewable evidence:

- `_SUCCESS.json` or `_ERROR.json` is always emitted for a run (and uploaded last).
- “Bad data” runs also include `tree/error.txt` (and still produce a verifiable `pack/`).

Internal/infrastructure failures (download/upload/marker write) should fail fast and be retryable (5xx).
