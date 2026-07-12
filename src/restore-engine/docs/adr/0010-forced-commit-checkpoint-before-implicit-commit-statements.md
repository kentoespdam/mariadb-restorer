# A forced COMMIT + Checkpoint precedes every implicit-commit statement (DDL, LOCK TABLES, …)

ADR-0001 makes the **Checkpoint** the byte offset of the last COMMITted
**Statement Boundary**, written *after* MariaDB confirms the COMMIT, so a crash
can never make the tool re-run already-durable data. ADR-0009 decides *when* the
tool itself COMMITs (bytes-first ~64 MB, statements-second ~1000). Both assume the
tool controls when a transaction ends. MariaDB does not honour that assumption:
certain statements **implicitly commit the open transaction before they execute**,
at a boundary the tool did not choose.

## The problem

Verified via context7 (MariaDB docs): "Certain SQL statements, primarily DDL
statements, cause an implicit commit of the current transaction before they are
executed… Even if the statement fails with an error, the transaction is
committed." So a `mariadb-dump` restore like:

```sql
INSERT INTO `orders` VALUES (...),(...),(...);   -- 50 MB accumulated in open Batch
DROP TABLE IF EXISTS `customers`;                -- server implicit-COMMITs the 50 MB first
```

The `DROP TABLE` silently COMMITs the 50 MB of `orders` **before** the executor
has written a Checkpoint for it. If the process crashes between that implicit
COMMIT and the tool noticing, resume seeks back to the last Checkpoint — *before*
the 50 MB — and re-runs it. Under **Fast Mode** (`unique_checks=0`,
`foreign_key_checks=0`) outside the narrow **Resume Batch** window, replayed
`INSERT`s are **not** tolerated: `1062` duplicate-entry is Fatal there (ADR-0006).
The invariant "Checkpoint is never behind durable data" is broken.

This also sharpens ADR-0008: when a `CREATE VIEW` fails `1227` and we **defer**
it, the implicit COMMIT *still happened* — "even if the statement fails, the
transaction is committed." The open Batch is committed by the server regardless of
the DDL's success. That is exactly why deferral is safe (no open Batch to poison),
**and** exactly why the Checkpoint must be written *before* the DDL, not after.

## Decision

The executor recognises an **Implicit-Commit Statement** and treats it as a
**forced Batch boundary**. On match, before executing it:

1. COMMIT the currently open Batch (if any statements are buffered).
2. Write the **Checkpoint** at the **Statement Boundary** just before this
   statement — i.e. the position the operator would resume from.
3. Then execute the implicit-commit statement as its own unit.

The server's implicit COMMIT now lands on a boundary we have already recorded, so
durability never outruns the Checkpoint. This composes with ADR-0008: a deferred
DDL still gets its pre-statement Checkpoint written first; whether the DDL then
succeeds or is deferred, the preceding data is already durably checkpointed.

## The detection set (an explicit prefix allowlist, not a "looks like DDL" heuristic)

Detection is a small **prefix allowlist**, matched only at the start of a
Statement while the **Statement Splitter** is in its `default` state. It is *not*
"the first keyword is CREATE/ALTER/DROP" — that heuristic is wrong in both
directions (below). Recognised (context7-verified as implicit-committing):

- `CREATE` / `ALTER` / `DROP` on tables, indexes, views, routines, events,
  databases, users, servers, tablespaces
- `RENAME TABLE`
- `TRUNCATE [TABLE]`
- `LOCK TABLES` / `UNLOCK TABLES`
- `LOAD DATA` (`INFILE`)
- `FLUSH` / `RESET` / `OPTIMIZE` / `ANALYZE` / `CHECK` / `REPAIR` (admin/table)
- `START TRANSACTION` / `BEGIN` (begins a new txn → commits the prior one)
- `GRANT` / `REVOKE` / `SET PASSWORD` (account-management DDL)

**Exceptions honoured (context7):**

- `CREATE TEMPORARY TABLE` / `DROP TEMPORARY TABLE` do **not** implicit-commit —
  excluded from the match by checking for the `TEMPORARY` keyword.
- `TRUNCATE TABLE` **always** commits, even on a temporary table.
- `CREATE INDEX` / `DROP INDEX` **always** commit, even against a temp table.
- `UNLOCK TABLES` commits only if a prior `LOCK TABLES` was on a non-transactional
  table. We still force a boundary on it unconditionally — an unnecessary COMMIT
  in the transactional-only case is harmless (correctness over micro-optimisation),
  and tracking "was any locked table non-transactional" is state we refuse to
  carry for a dump that is almost entirely InnoDB.

## Why not the "first keyword is DDL" shortcut

- **False negative (dangerous):** `LOCK TABLES \`orders\` WRITE;` — which
  `mariadb-dump` emits before every table's INSERT block — does **not** start with
  a DDL verb, yet it implicit-commits. A DDL-verb-only check misses it, leaving the
  very gap this ADR closes. `LOAD DATA`, `FLUSH`, `BEGIN` share this shape.
- **False positive (wasteful):** `CREATE TEMPORARY TABLE` starts with `CREATE` but
  does **not** commit. Forcing a boundary is harmless to correctness but adds a
  needless COMMIT. The allowlist encodes the `TEMPORARY` exception explicitly.

The allowlist is cheap: these are short, regular prefixes from a machine-generated
dump, tested only at a Statement's first token in `default` state — never a
hot-path cost on the millions of `INSERT`s that dominate a 9 GB restore.

## Considered Options

- **Ignore implicit commits (status quo of ADR-0001/0009):** rejected — leaves the
  crash-replay hole above; the "Checkpoint never behind durable data" invariant
  fails at every DDL and every `LOCK TABLES`.
- **Match on the DDL verb only (`CREATE`/`ALTER`/`DROP`):** rejected — misses
  `LOCK TABLES` / `LOAD DATA` / `FLUSH` / `BEGIN` (not DDL-verbs but committing)
  and mis-fires on `CREATE TEMPORARY TABLE` (DDL-verb but non-committing).
- **Ask the server after each statement whether an implicit commit occurred:**
  rejected — no portable, cheap signal for it; a round-trip per statement defeats
  the streaming design. The dump's SQL is deterministic; a prefix allowlist is
  exact and free.
- **Wrap the whole restore in one transaction to avoid the question:** rejected —
  impossible; DDL force-commits regardless, and a single 9 GB transaction defeats
  bounded-resume-rollback (ADR-0009) and constant-memory goals.
- **Checkpoint *after* the DDL instead of before:** rejected — the implicit COMMIT
  happens *before* the DDL executes and *even if it fails*; the only safe place for
  the Checkpoint is before the statement, at the boundary preceding it.

## Consequences

- The executor gains one prefix-allowlist check at each Statement's first token
  (in `default` state only). Cheap and confined to statement starts.
- A restore now COMMITs on three triggers, not two: `--commit-bytes`,
  `--commit-statements` (ADR-0009), **and** "next statement implicitly commits."
  Batches around DDL/`LOCK TABLES` are therefore often smaller than the byte
  threshold — expected and correct, not a bug.
- `mariadb-dump` output naturally produces one forced boundary per table (its
  `LOCK TABLES … WRITE` / `UNLOCK TABLES` framing), giving frequent, well-placed
  Checkpoints for free — resume granularity is roughly per-table without any extra
  logic.
- This closes the loop with ADR-0008: a deferred DDL's preceding data is
  Checkpointed before the DDL runs, so deferral never risks a replay gap.
- The `TEMPORARY` exception and the `UNLOCK TABLES` "non-transactional only" nuance
  are documented but the executor deliberately over-approximates `UNLOCK TABLES`
  (always forces a boundary) rather than tracking lock table engines — a conscious
  correctness-over-optimisation choice recorded here so a future reader does not
  "fix" it into a subtle bug.
