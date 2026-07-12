# Checkpoint Store runs `synchronous=FULL`; the one-batch resume invariant depends on it

The **Resume Batch** (CONTEXT.md) tolerates exactly `1062`/`1050` and *only for the
first Batch after a resume*. That narrowness is correct only under one invariant:
the persisted `byte_offset` lags the last MariaDB COMMIT **by at most one Batch**.
This ADR pins the storage-durability setting that keeps that invariant true, because
getting it wrong makes a healthy restore die on resume — a surprising, data-shaped
failure that only appears after a real power loss, so it is recorded here.

## The invariant, and how a weak `synchronous` breaks it

**Checkpoint** ordering (ADR-0001) writes the `byte_offset` only *after* the batch's
COMMIT is confirmed. So on a crash there are only two states for the last batch:

- COMMIT never happened → offset still points before it → resume re-runs it cleanly.
- COMMIT happened but the crash beat the checkpoint write → offset points before an
  already-applied batch → resume re-runs exactly **one** already-committed batch,
  whose `1062`/`1050` the Resume Batch absorbs. This is the *entire* reason the
  tolerance window is one batch wide.

That second state stays one batch wide only if each checkpoint write is durable on
disk **before** the next batch is COMMITted to MariaDB. Under `synchronous=NORMAL`
(or `OFF`) SQLite reports the checkpoint "committed" while its page is still in the
OS cache, unfsynced. Then:

1. Checkpoint for batch *N* returns — but is not fsynced.
2. Batches *N+1, N+2, N+3* COMMIT to MariaDB (those COMMITs *are* durable server-side).
3. **Power loss.** The OS cache holding checkpoints *N…N+3* is gone; on disk the
   `byte_offset` has rolled back to before batch *N*.
4. `resume` seeks to *N* and replays *N, N+1, N+2, N+3* — all already applied in
   MariaDB, so every one raises `1062`/`1050`.
5. The Resume Batch tolerates the **first** replayed batch only. Batch *N+1* is a
   normal Batch again, its `1062` is **Fatal**, and the restore fails — even though
   nothing was actually wrong.

A weak `synchronous` silently widens the replay window past one batch and turns the
locked `1062`/`1050` tolerance into an unsound rule.

## Decision

- The **Checkpoint Store** opens with `PRAGMA synchronous=FULL`. Every checkpoint
  write (the per-batch `byte_offset` UPSERT) is fsync-durable before the tool issues
  the next batch's COMMIT to MariaDB. This is what guarantees the resume replay
  window is at most one batch — the precondition the Resume Batch is built on.
- `modernc.org/sqlite` honours the standard SQLite durability PRAGMAs; its own
  published benchmark harness pairs `journal_mode` with `synchronous=FULL` (confirmed
  via context7 `/websites/pkg_go_dev_modernc_org_sqlite`). Journal mode itself
  (`DELETE` vs `WAL`) is left at the driver/SQLite default and is not part of this
  decision — only the fsync-on-commit guarantee is.
- **Fast Mode does not weaken this.** Throughput is tuned by **Batch size**
  (statements per COMMIT, ADR-0009), never by checkpoint frequency or durability.
  Bigger batches mean fewer checkpoint fsyncs per GB *and* fewer MariaDB COMMITs,
  with the one-batch invariant untouched. `synchronous` is never lowered for speed.

## Why the fsync cost is affordable

A checkpoint is a single-row UPSERT; its fsync is a fraction of the work already
spent COMMitting the batch to MariaDB — a network round trip plus the server writing
every row in the batch. Paying one small local fsync per batch to keep resume correct
is negligible beside that. The cost scales with batch *count*, so larger Fast Mode
batches amortise it further rather than fighting it.

## Considered Options

- **`synchronous=NORMAL` for speed:** rejected — saves an already-small fsync by
  trading away resume correctness under power loss, which is precisely the failure
  this crash-resume tool exists to survive. The saved cost is a fraction of the batch
  COMMIT it sits next to; the risk is a healthy restore aborting with a spurious Fatal
  `1062`.
- **`synchronous=OFF`:** rejected outright — no durability guarantee at all; the
  `byte_offset` can be arbitrarily stale after a crash, making the replay window
  unbounded and resume unsafe.
- **Widen the Resume Batch tolerance window to N batches instead:** rejected —
  tolerating `1062`/`1050` across many batches would mask genuinely duplicate data or
  a real schema clash outside the replay window, which is exactly what the narrow
  two-code / one-phase Tolerated Error contract refuses to do. Fix durability at the
  store, not by loosening the error contract.
- **Checkpoint less often (every K batches) to save fsyncs:** rejected — it enlarges
  the replay window to K batches by construction, reintroducing the same problem the
  invariant avoids; and it is unnecessary because the per-batch fsync is already cheap
  relative to the batch COMMIT. Tune throughput via batch size, not checkpoint spacing.

## Consequences

- The store's connection wiring pins `PRAGMA synchronous=FULL` at open time; this is
  an implementation constant, not an operator knob.
- The Resume Batch's one-batch tolerance window remains sound: after any crash the
  persisted offset lags the last durable MariaDB COMMIT by at most one batch.
- Fast Mode's speed story stays entirely in Batch size (ADR-0009); no fast path may
  lower `synchronous` or skip checkpoints, and a reviewer seeing such a change should
  treat it as a correctness regression, not an optimisation.
- The fsync-per-batch cost is bounded by batch count and shrinks as batches grow, so
  it never dominates a large-dump restore.
