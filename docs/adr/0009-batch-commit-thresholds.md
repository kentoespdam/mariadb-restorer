# Batches commit on bytes-first (~64 MB), statements-second (~1000); oversized statements commit alone

A **Batch** (ADR-0001) is the transaction between two **COMMIT**s and the unit of
atomic resume progress. Something has to decide *when* to COMMIT. This ADR fixes
that trigger, and why it is byte-driven rather than row-driven.

## The trigger: whichever crosses first

- **Primary — bytes accumulated ≥ `--commit-bytes` (default 64 MB).**
- **Secondary — Statements accumulated ≥ `--commit-statements` (default 1000).**

Both are defaults, both overridable by flag, both sensible with zero
configuration.

## Why bytes are primary, not row/statement count

A `mariadb-dump` **Dump** uses `--extended-insert` (`-e`), on by default, so one
`INSERT` **Statement** carries many rows and can be *hundreds of MB*. Counting
statements as the primary trigger is therefore meaningless for sizing a
transaction: "1000 statements" could be 1 MB or 100 GB depending on how the dump
was tuned (its `--net-buffer-length` controls per-INSERT size — verified via
context7, but that value lives in the dump, not in our control). Only bytes track
the real cost that matters here: the size of the transaction MariaDB must hold and
the amount of work a rollback-on-resume throws away.

**64 MB** is chosen to sit well above the common single extended-INSERT size (~1 MB
class) so a Batch amortises COMMIT overhead across many statements, yet small
enough that a crash-resume rolls back at most tens of MB of redo — not gigabytes.

The statement count stays as a **secondary** cap for the opposite dump shape: a
dump of millions of tiny single-row `INSERT`s (extended-insert disabled) would
otherwise pack hundreds of thousands of statements into one transaction before the
byte threshold trips. ~1000 bounds that.

## The minimum-one rule (why we never split a Statement)

The byte threshold means **"COMMIT after crossing"**, not **"never exceed"**. A
single **Statement** larger than 64 MB — a 300 MB extended INSERT — is executed
*whole, alone*, as a one-statement Batch, then committed immediately. We do **not**
split it.

Splitting is not on the table: ADR-0002 and the **Statement Splitter**'s contract
send every Statement to the server *verbatim*. The tool is not a SQL rewriter; it
cannot break one `INSERT (...),(...),(...)` into several without parsing and
re-emitting SQL, which is exactly what the splitter is designed *not* to do. So the
only knob we own is *when to COMMIT*, never *how big a Statement is*. A Statement's
size is fixed in the Dump; our job is to wrap it in a transaction, not reshape it.

The one hard ceiling on a single Statement is the server's `max_allowed_packet`,
not our Batch threshold — and that is enforced elsewhere (Pre-flight Check + fatal
`1153`), see below.

## Interaction with `max_allowed_packet` — actionable tuning, we never set it

Our Batch byte threshold and the server's `max_allowed_packet` are different
limits: ours groups statements into a transaction; the server's caps a *single*
packet/Statement. A Batch can be 64 MB of small statements with no packet anywhere
near the ceiling; conversely a lone 300 MB Statement can breach
`max_allowed_packet` while being a perfectly ordinary one-statement Batch.

We **read, never set** `@@global.max_allowed_packet` (Pre-flight Check, ADR-0003):
it is the server admin's dial, a managed host may forbid raising it, and setting it
silently would mask a real capacity limit. When a Statement exceeds it the server
returns `1153`, which stays **Fatal** (ADR-0006) — but the message is
**actionable**. Verified via context7 (MariaDB docs), `max_allowed_packet` is a
*dynamic* variable, and a runtime change is *lost on restart unless persisted*.
So the tool prints the complete fix, not a bare code:

```
Statement at byte offset <N> (~<size>) exceeds the server's max_allowed_packet
(<current>). Raise it and resume:
  SET GLOBAL max_allowed_packet = <needed>;      -- runtime, admin privilege
  # and add to my.cnf so it survives a restart:
  [mysqld]
  max_allowed_packet = <needed>
Then re-run with `resume`.
```

`resume` opens a fresh connection; its Pre-flight Check re-reads the now-larger
ceiling. Telling the operator to `SET GLOBAL` *only* would be a trap — the next
restart silently reverts it. This is why the advice always pairs runtime + `my.cnf`.

## Considered Options

- **Statement/row count as the primary trigger:** rejected — with extended-insert
  on (the dump default), statement count is decoupled from transaction size by
  orders of magnitude; it cannot bound rollback cost or server memory.
- **Split oversized statements to fit a target Batch size:** rejected — violates
  ADR-0002 (verbatim statements) and the Statement Splitter's no-rewrite contract;
  would require a real SQL parser the tool deliberately does not have.
- **Have the tool raise `max_allowed_packet` automatically:** rejected — it is the
  admin's dial; managed hosts may deny it, and auto-setting hides a genuine
  capacity signal. We surface an actionable message instead and stay read-only.
- **A single fixed threshold, no flags:** rejected — 64 MB / 1000 are good
  defaults but the right transaction size is workload-dependent (disk, redo log,
  latency to server); exposing `--commit-bytes` / `--commit-statements` costs
  almost nothing and avoids a re-release to re-tune.
- **Much larger byte threshold (e.g. 1 GB) for fewer COMMITs:** rejected for MVP —
  fewer COMMITs marginally speeds a clean run but multiplies the redo a
  crash-resume discards and the memory the server holds open; 64 MB is the safer
  default for a *resumable* restore, and the flag is there for those who want more.

## Consequences

- The executor tracks two running counters per Batch (bytes, statements) and
  COMMITs on the first to trip — cheap, no buffering of statement text beyond the
  current one.
- Default Batch size is bounded (~64 MB), so worst-case resume rollback is bounded
  too — a property the resume UX can state honestly.
- A single Statement can still exceed the byte threshold; the executor must handle
  the "already over budget with one statement" case by committing immediately after
  it, never by attempting to trim it.
- Two capacity limits coexist and must not be conflated in docs or messages: the
  Batch byte threshold (ours, a COMMIT cadence) and `max_allowed_packet` (the
  server's, a hard per-Statement wall). Only the latter can make a Statement fail.
- The `1153` path carries a fuller remediation string than other Fatal errors;
  the fatal-error printer needs a per-code hook for that actionable text rather
  than one generic format.
