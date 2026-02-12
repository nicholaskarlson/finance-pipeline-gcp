# finance-pipeline-gcp

Portfolio MVP: the **Audit-Proof Drop Folder** workflow.

**Pitch:** Upload two CSVs (**left** + **right**). The system:
1) runs **deterministic** normalization + reconciliation,
2) produces an **audit-pack style** set of artifacts (so results can be verified later),
3) emits a simple, human-readable summary report.

This repo is intentionally small and “proof-first”: the goal is not “pretty dashboards,” it’s **repeatable, checkable outputs**.

---

## What this does (conceptually)

You provide two datasets that should mostly agree (for example: bank export vs. ledger export).

The pipeline classifies rows into buckets such as:

- **matched**: same record on both sides (by a stable key)
- **left_only**: only found on left
- **right_only**: only found on right
- **mismatched**: key exists on both sides, but one or more fields differ

Everything is generated in a stable order and format so CI (and you) can verify outputs byte-for-byte.

---

## Quick start (local)

### 1) Run the proof gate

```bash
gofmt -w cmd internal
make verify
go test -count=1 ./...
```

- `make verify` is the **proof gate**: tests + deterministic demo runs.
- `make demo` runs the same inputs twice and `diff`s the full output trees.

### 2) Optional: local server smoke

```bash
PORT=18080 make server-smoke
```

This starts the Cloud Run handler locally and POSTs `{}`; the safe behavior is to return **204** (no-op).

---

## Input contract (CSV)

The demo uses this simple schema:

| column | type | notes |
|---|---|---|
| `id` | string | stable key for matching |
| `date` | YYYY-MM-DD | ISO date |
| `amount` | decimal | keep as text/decimal; avoid float surprises |
| `description` | string | free text |

If you extend the schema, keep the contract explicit and update fixtures + goldens.

---

## Artifacts produced (stable layout)

Whether you run locally (`cmd/pipeline run`) or via Cloud Run, the pipeline produces the same **run folder** layout.

For an output base `./out/demo1` and `--run-id demo`:

```
./out/demo1/demo/
  tree/
    inputs/
      left.csv
      right.csv
    work/
      ... (recon outputs)
    error.txt              # only on "bad data" (recon failure)
  pack/
    ... (auditpack outputs; verifiable)
  _SUCCESS.json            # terminal marker (uploaded/written last)
  _ERROR.json              # terminal marker (uploaded/written last)
```

Key idea:
- `tree/` is the deterministic evidence folder (inputs + work outputs).
- `pack/` is a verifiable audit pack built from `tree/`.
- A completion marker (`_SUCCESS.json` or `_ERROR.json`) is the **done signal**.

---

## Determinism + idempotency contract

This repo guarantees:

- **Byte-stable outputs** for the same inputs (proof gate enforces this).
- **Stable ordering** (sorted walks; no map iteration surprises in output-shaping code).
- **Atomic writes** for important files (write temp → rename).
- **Deterministic upload order**: all artifacts first, completion marker **last**.
- **Idempotent server behavior**: if a completion marker already exists for a run, the server ACKs (204) and does nothing.

See `docs/CONVENTIONS.md` for the full contract.

---

## Event contract dependency (Book 2 source of truth)

Event parsing + filtering rules (finalize vs ignore, bucket guardrails, name unescaping, expected-fail behavior) are delegated to:

- `github.com/nicholaskarlson/proof-first-event-contracts` (pinned in `go.mod`)

For Book 2, we freeze both repos with matching tags (e.g. `book2-v1`) so the book references **tags, not moving HEADs**.

See `docs/cloud-run.md` for details.

---

## Repo layout (high level)

- `cmd/` — CLI entry points (`run`, `server`)
- `internal/` — pipeline orchestration + Cloud Run handler
- `fixtures/` — small reproducible datasets for demos/tests
- `docs/` — design notes (short, practical, contract-first)

---

## macOS note (make)

This repo’s Makefile is written for GNU make (CI uses `gmake` on macOS).
If you’re on macOS:

```bash
brew install make
gmake verify
PORT=18080 gmake server-smoke
```

---

## License

MIT (see `LICENSE`).
