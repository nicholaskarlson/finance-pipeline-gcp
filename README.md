# finance-pipeline-gcp

Portfolio MVP: the **Audit‑Proof Drop Folder** workflow.

**Pitch:** Upload two CSVs (**left** + **right**). The system:
1) runs **deterministic** normalization + reconciliation,
2) produces an **audit‑pack style** set of artifacts (so results can be verified later),
3) emits a simple, human‑readable summary report.

This repo is intentionally small and “proof‑first”: the goal is not “pretty dashboards,” it’s **repeatable, checkable outputs**.

---

## What this does (conceptually)

You provide two datasets that should mostly agree (for example: bank export vs. ledger export).

The pipeline classifies rows into buckets such as:

- **matched**: same record on both sides (by a stable key)
- **left_only**: only found on left
- **right_only**: only found on right
- **mismatched**: key exists on both sides, but one or more fields differ

Everything is generated in a stable order and format so that CI (and you) can verify outputs byte‑for‑byte.

---

## Quick start (local)

### 1) Create demo inputs

```bash
mkdir -p fixtures/demo

cat > fixtures/demo/left.csv <<'EOF'
id,date,amount,description
a1,2026-01-01,10.00,coffee
a2,2026-01-02,20.00,books
a3,2026-01-03,30.00,groceries
EOF

cat > fixtures/demo/right.csv <<'EOF'
id,date,amount,description
a1,2026-01-01,10.00,coffee
a3,2026-01-03,30.00,groceries
b9,2026-01-09,99.00,unknown
EOF
```

### 2) Run the verification gate

```bash
gofmt -w cmd internal
make verify
go test -count=1 ./...
```

> `make verify` is the “proof gate.” It should fail loudly if outputs drift.

---

## Input contract (CSV)

The demo uses this simple schema:

| column | type | notes |
|---|---|---|
| `id` | string | stable key for matching |
| `date` | YYYY-MM-DD | ISO date |
| `amount` | decimal | keep as text/decimal, avoid float surprises |
| `description` | string | free text |

If you extend the schema, keep the contract explicit and update fixtures + goldens.

---

## Determinism contract (what we guarantee)

- **Event filtering attaching to Cloud Run** is delegated to `proof-first-event-contracts` (tagged snapshot for Book 2).
This project is designed so that:

- the same inputs produce the same outputs (byte‑for‑byte),
- ordering is stable (no map iteration surprises),
- numeric formatting is consistent,
- and verification is automated (tests + fixtures + goldens).

If you touch output formatting, treat it as a breaking change:
update fixtures/goldens together and let CI be your witness.

---

## Repo layout (high level)

- `cmd/` — CLI entry points (pipeline runner)
- `internal/` — core reconciliation + normalization logic
- `fixtures/` — small reproducible datasets for demos/tests
- `docs/` — notes and design docs (keep short + practical)

---

## Roadmap (MVP → next)

- Add a minimal “drop folder” adapter (local folder first, then cloud)
- Emit a structured JSON summary (for UI / downstream automation)
- Add an audit‑pack manifest (hashes + run metadata) to harden proof
- Define a tiny schema registry (so contracts are versioned)

---

## macOS tooling note (GNU make)

This repo intentionally uses **GNU make** features (e.g. `.RECIPEPREFIX`) to keep recipes readable and consistent across the book repos.

On macOS, the default `/usr/bin/make` is BSD make and may fail with errors like:

- `Makefile:13: *** missing separator. Stop.`

Use GNU make instead:

```bash
brew install make
gmake verify
PORT=18080 gmake server-smoke
```

## License

MIT (or as specified in the repo’s LICENSE file).

## Cloud Run server (Eventarc + Cloud Storage)

This repo includes a small HTTP server intended for **Cloud Run** behind an **Eventarc trigger** (Cloud Storage object finalized).

Key safety rule:
- The server requires `INPUT_BUCKET` and will **ignore events from any other bucket**.

See `docs/cloud-run.md` for the expected object layout and environment variables.
