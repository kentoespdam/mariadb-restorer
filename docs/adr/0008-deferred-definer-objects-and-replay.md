# Missing-DEFINER object DDL is deferred to a replay queue, not fatal; the run exits 3

ADR-0006 makes any non-tolerated **Batch** error fail-fast, and ADR-0002 keeps
every **Statement** verbatim (the tool is not a SQL rewriter). Both stand. But a
common cross-host `mariadb-dump` restore failure is a `CREATE
VIEW/TRIGGER/ROUTINE/EVENT` carrying `DEFINER='someuser'@'somehost'` that the
target cannot accept.

**When this actually fails at create time (verified via context7, MariaDB
docs).** The two DEFINER clauses are checked at different times:

- **Create-time — privilege check.** Setting `DEFINER=` to *another* account
  requires the `SET USER` privilege (≥10.5) / `SUPER`. Without it, `CREATE`
  fails immediately with `1227` `ER_SPECIFIC_ACCESS_DENIED_ERROR`. This is the
  **dominant** deferral case: restoring as a limited app-user on a managed DB.
- **Reference-time — existence check.** A *missing DEFINER account* is normally
  detected "when the view is referenced" (queried) / when the trigger fires —
  i.e. at runtime, **outside this tool**. `1449` `ER_NO_SUCH_USER` therefore
  usually does NOT occur during restore; if the restore runs with full privilege
  (as `root`), the `CREATE` simply SUCCEEDS and nothing is deferred. We still
  catch `1449` at create time as a net for the paths that do surface it, but it
  is the exception, not the rule.

So deferral is specifically the **restricted-privilege** path (`1227`). Under
pure ADR-0006, one such object 8 hours into a 9GB restore would kill the whole
run for a metadata problem the data load itself does not care about — the wrong
trade-off for this error class, so ADR-0008 carves a deliberately narrow
exception.

## Why deferral is safe (verified via context7, MariaDB docs)

Two server facts make deferral safe rather than a silent-corruption risk — the
precise objection that sank "skip and continue" in ADR-0006:

1. **A failed statement does not roll back its transaction.** MariaDB docs: an
   error in one statement aborts only that statement; a following statement in
   the same transaction still runs. So capturing the error and moving on does not
   poison surrounding work.
2. **`CREATE VIEW/TRIGGER/ROUTINE/EVENT` is DDL, and DDL forces an implicit
   COMMIT.** Each of these objects commits on its own. A `1449`/`1227` here has
   no open **Batch** to unwind and no half-written data to leave behind.

Because these DDL statements are small (KB, not the 300MB of an extended
`INSERT`), storing the **verbatim statement text** to replay later is cheap and
bounded. This is why the exception is restricted to object-creating DDL and is
**never** extended to data.

## Behavior

- **Scope, exact:** ONLY `1227`/`1449`, ONLY on `CREATE VIEW/TRIGGER/ROUTINE/
  EVENT`. Those codes in any other context, and every other error anywhere,
  remain **Fatal** per ADR-0006.
- **Capture, don't rewrite:** on match, write a row to the **Deferred Store**
  (verbatim statement, **Byte Offset**, error code, object identity, timestamp),
  print a warning, and continue. The DEFINER clause is never stripped or
  rewritten — ADR-0002 holds.
- **Finish, then report:** the restore runs to completion. At the end, if the
  **Deferred Store** is non-empty, the tool prints a prominent summary: *"Restore
  complete, but N objects were deferred because their DEFINER user is missing.
  Create the users, then run `replay`."*
- **`replay` subcommand (fixed-point iteration):** after the operator fixes the
  cause (creates the DEFINER users, or grants the restoring account `SET USER`),
  `replay` re-executes the deferred statements — not once, but in rounds. Each
  round runs every remaining row; a success deletes its row; if a round clears
  ≥1 object it loops again; it stops when a full round makes no progress. This
  drains deferred **view-on-view** chains automatically (see below) without the
  tool parsing view dependencies. Reference-time errors during replay (`1146`
  no such table, `1356` view-references-invalid) are non-fatal *in replay* — the
  object stays queued for the next round rather than aborting the drain.
- **Exit code 3:** a run leaving any **Deferred Object** exits with a dedicated
  code `3` — distinct from `0` (clean) and from fail-fast `1`. Cron/CI can react
  ("succeeded with reservations") instead of mistaking a partial result for a
  clean restore. This preserves ADR-0006's spirit — never a *silent* partial
  success — while relaxing its fail-fast on this one class.

## The orphaned View Stub, and why replay must iterate

`mariadb-dump` restores a view in two moves: first a **View Stub** (a placeholder
table with real column names, dummy types, so view-on-view references resolve),
then later `DROP TABLE` the stub and `CREATE VIEW` the real thing:

```sql
CREATE TABLE `v_report` (...dummy...);   -- stub, succeeds
DROP TABLE IF EXISTS `v_report`;         -- succeeds
CREATE VIEW `v_report` AS SELECT ...;    -- 1227: DEFINER not permitted → DEFERRED
```

The trap: the `DROP TABLE` has *already succeeded* before the `CREATE VIEW`
fails. Deferring the `CREATE VIEW` therefore leaves `v_report` **gone entirely**
— stub dropped, real view not yet created. Any other view `v_summary AS SELECT
... FROM v_report` will then fail its own replay with `1146`/`1356` until
`v_report` exists — a dependency chain among deferred objects.

This is exactly why `replay` is a **fixed-point iteration**, not a single pass:
`v_report` is created in one round, and `v_summary` succeeds in the next. The
orphaned-stub state is a transient consequence of the dump's own ordering (the
`DROP TABLE` is in the dump, not something we do), fully healed once replay
converges. It does not change the deferral decision; it is the reason the drain
loops until no progress.

## Triggers are deferred too

A deferred trigger opens a window where table data is loaded but the trigger does
not yet exist. This is acceptable — and in fact consistent with the dump's
intent: `mariadb-dump` restores triggers *after* their table's data precisely so
they do NOT fire during the bulk load (firing them would double-count
aggregates/side effects). A trigger is not meant to run against the historical
rows being restored, so deferring its creation changes nothing about the data
already loaded. Triggers are therefore in scope for deferral alongside
views/routines/events.

## Considered Options

- **Keep pure ADR-0006 fail-fast on `1449`/`1227`:** rejected — turns the most
  common env-migration hiccup (a missing metadata user) into a full-restore
  abort, forcing a `resume` for something that isn't a data problem.
- **Strip or rewrite the DEFINER clause (e.g. to `CURRENT_USER`):** rejected —
  violates ADR-0002 (verbatim statements); silently changes ownership/security
  semantics the operator may rely on. The tool reports, it does not edit SQL.
- **Silently skip the failing object and exit 0:** rejected — the exact silent
  partial-success ADR-0006 forbids. Deferral differs: the statement is captured
  for **Replay** and the run advertises itself via exit code 3.
- **Defer data (`INSERT`) failures the same way:** rejected — extended INSERTs
  are huge, and an INSERT error is not the benign missing-metadata-user case;
  it signals real trouble and stays **Fatal**.
- **Keep triggers Fatal, defer only view/routine/event:** rejected as
  unnecessarily conservative — see "Triggers are deferred too"; deferring a
  trigger is consistent with why the dump orders it after the data.

## Consequences

- A second SQLite table (the **Deferred Store**) joins the single-row
  **Checkpoint Store** in the **Data Directory** file — an append-many queue,
  drained by `replay`.
- The CLI grows a `replay` subcommand and a third documented exit code (`3`).
  Operator docs must state all three: 0 clean / 3 deferred / 1 fatal.
- Detecting the DDL class means the executor must know a deferred-eligible
  statement when it sees one. Cheap: the small, regular `CREATE
  VIEW/TRIGGER/…` prefixes from `mariadb-dump`, checked only when the error is
  `1449`/`1227` — never a hot-path cost on normal statements.
- `replay` is a distinct phase from `resume`: `resume` continues an interrupted
  data load from a **Byte Offset**; `replay` drains deferred metadata. They use
  different stores and must not be conflated in the UX.
- `replay` must loop to a fixed point (round with no progress), so it needs a
  per-round progress counter and a stable "no progress → stop" guard. The
  bound is safe because the queue only shrinks or holds — each round either
  removes ≥1 row or terminates. A convergence that leaves rows means the cause
  is still unfixed (user absent / privilege ungranted); those rows and their
  final errors are reported for the operator, still exit `3`.
