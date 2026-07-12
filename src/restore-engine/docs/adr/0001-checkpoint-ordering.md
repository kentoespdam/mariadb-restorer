# Checkpoint is written after COMMIT, and the resume batch tolerates duplicates

The local checkpoint (SQLite) and the remote data write (MariaDB COMMIT) are
two independent persistence systems with no distributed transaction between
them. We order them so that a crash is always recoverable: execute a **Batch**
inside one transaction, `COMMIT` to MariaDB, and only *then* write the batch's
ending byte offset to SQLite.

This guarantees **zero silent data loss**: if we crash before COMMIT, MariaDB
rolls the batch back AND the checkpoint has not advanced, so resume re-executes
the batch cleanly. The only residual risk is the narrow window where a COMMIT
succeeds but the process dies before the checkpoint write — on resume that one
batch is re-executed. We absorb this by running the **Resume Batch** (the first
batch after any resume) with error tolerance for duplicate-entry / table-exists,
since `INSERT`/`CREATE` are not idempotent.

## Considered Options

- **Checkpoint per-statement (PRD §5.3 as written):** rejected — with
  `autocommit=0`, a checkpoint can advance past statements MariaDB later rolls
  back on crash, causing silent data loss.
- **Two-phase commit / XA:** rejected — heavy, and MariaDB XA + a local SQLite
  resource is not a real distributed transaction anyway. The gap is never truly
  zero without it, and error-tolerant replay closes the gap far more cheaply.

## Consequences

- Batch size bounds are max(statement-count, bytes-processed) — whichever first
  — because one extended INSERT can be hundreds of MB.
- COMMIT cadence and checkpoint cadence are the same event, by construction.
