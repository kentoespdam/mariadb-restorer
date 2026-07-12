# The restore owns its Fast Mode session vars on one pinned connection, resetting all three on every exit

**Fast Mode** (ADR-0003, CONTEXT.md) turns off three session variables for speed:
`autocommit=0`, `unique_checks=0`, `foreign_key_checks=0`, set with `SET SESSION`
during the **Pre-flight Check**. Two questions were left open: *which connection*
runs these statements, and *who is responsible for turning the checks back on*.
This ADR answers both, and rejects the tempting-but-wrong answer to the second.

## Why one pinned connection is not optional

MariaDB session variables are strictly per-connection. Verified via context7
(MariaDB docs): *"session variables affect only the current connection"*, and
*"when a user connects, the current global value is copied to their session"* — a
`SET SESSION` never reaches any other connection, existing or future.

That guarantee is a floor, not a convenience. `autocommit=0` is what makes a
**Batch** a transaction at all: if the executor spread its statements across a
pooled `*sql.DB`, statement N could land on connection A (autocommit off) and
statement N+1 on connection B (autocommit *on*, because B never ran our `SET`).
The **Batch** boundary, the COMMIT cadence of ADR-0009, and the implicit-commit
handling of ADR-0010 all assume one continuous session. So the restore **pins a
single connection** via `db.Conn(ctx)` and runs the *entire* job on it —
Pre-flight Check, every Batch, the Resume Batch, and Verify. This is required for
correctness first; the Fast Mode variables riding on that same connection is a
consequence, not the motivation.

## The trap: MariaDB isolates sessions, but Go pools connections

It is tempting to reason: "session vars can't leak to other clients (context7
confirms it), so if we crash with `foreign_key_checks=0` nothing is harmed —
skip the reset." That is half right and dangerously incomplete.

- **True at the server:** `fk=0` on our connection cannot affect any *other
  client's* connection. Another application connecting to the same MariaDB is
  unaffected. The fear "fk=0 disturbs other connections" does not exist at the
  server level — MariaDB's session isolation forecloses it.
- **False inside our own process:** Go's `database/sql` **pools** connections.
  Verified via context7 (Go docs): `Conn.Close()` *"returns the connection to the
  connection pool"* — it does not close the socket. A driver *may* implement
  `SessionResetter`/`ResetSession`, called *"prior to executing a query on the
  connection if the connection has been used before"* — but that resets
  driver-level state, **not** arbitrary application `SET SESSION` variables. The
  Go MySQL driver does not issue a `RESET` that clears our `foreign_key_checks=0`.

So the real hazard is intra-process: a connection left at `fk=0` and returned to
the pool can be borrowed by a *later phase of our own run* — most concretely the
**Verify** phase, whose entire job is to check the referential integrity Fast
Mode skipped. A Verify running on an inherited `fk=0` session would validate
nothing and report a false clean.

## Decision

1. **Pin one connection.** The whole restore runs on a single `db.Conn(ctx)`.
   Fast Mode's three `SET SESSION` statements are issued once, during Pre-flight
   Check, on that connection.
2. **Reset all three on every exit path.** A `defer` on the pinned connection
   re-asserts `foreign_key_checks=1`, `unique_checks=1`, `autocommit=1` before the
   connection is released back to the pool or the process exits — on clean
   completion, on **Fatal Error**, and on panic. All three, not just
   `foreign_key_checks`: leaving `unique_checks=0` or `autocommit=0` on a pooled
   connection is the same class of intra-process bug.
3. **We own these three; the dump owns the rest.** The tool never depends on the
   dump footer's `SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS` (or the header's
   save) to restore integrity state. Dump-owned session settings (TIME_ZONE,
   SQL_MODE, charset/NAMES, SQL_LOG_BIN, NO_AUTO_VALUE_ON_ZERO) still pass through
   verbatim per ADR-0002 — those are the dump's contract. The three Fast Mode
   variables are *ours*: we set them, and we reset them, regardless of what the
   dump's header or footer does.
4. **Verify re-asserts `foreign_key_checks=1` itself.** Because Verify runs on the
   same pinned connection immediately after the data load turned FK off, it does
   not trust that state — it sets `foreign_key_checks=1` before running its
   referential-integrity checks. This is belt-and-suspenders with the `defer`
   reset, and it makes Verify correct even if a future refactor moves it off the
   pinned connection.

## Why not "SET foreign_key_checks=1 on exit" as the whole story

Rejected as *reasoning*, even though the code line is similar. Framing the reset
as "so we don't disturb other connections" is factually wrong (server isolation
already guarantees that) and would mislead a future reader into thinking the reset
is cosmetic and removable. The reset exists for one precise reason: **our own
process's connection pool may reuse the connection.** Recording the correct
rationale here prevents someone from later deleting the `defer` after "verifying"
that session vars can't leak to other clients — a true fact that does not license
removing the reset.

## Considered Options

- **Run on the pooled `*sql.DB`, reset nothing:** rejected — breaks Batch
  correctness (autocommit could differ per statement across pooled connections)
  and leaves `fk=0`/`unique_checks=0` connections in the pool for Verify to
  inherit, producing a false-clean integrity check.
- **Depend on the dump footer's `SET FOREIGN_KEY_CHECKS=@OLD_...` to restore
  state:** rejected — that footer runs only if the restore reaches the end
  cleanly; a Fatal Error mid-restore never executes it, leaving `fk=0` on a pooled
  connection. Ownership of a safety-critical variable cannot be delegated to a line
  we might never reach. (We also could not context7-confirm the exact header/footer
  text for MariaDB 10.11; independent of that, delegating the reset is unsound.)
- **Reset only `foreign_key_checks`, not the other two:** rejected — `unique_checks`
  and `autocommit` are the same intra-process pool-reuse hazard; a half-reset
  connection is a subtle-bug source. Cost is two extra `SET`s, once, at exit.
- **Rely on the driver's `ResetSession` to clear our SET vars:** rejected —
  `ResetSession` is driver-defined and does not clear arbitrary application session
  variables (context7); depending on it is depending on unspecified behavior.

## Consequences

- The executor acquires and holds one `*sql.Conn` for the job's lifetime and must
  thread it through every phase (Pre-flight, Batches, Resume Batch, Verify, the
  final reset). No phase may open its own pooled connection for statement
  execution.
- A single `defer` on that connection performs the three-variable reset, reached
  on success, Fatal Error, and panic alike — the one place FK/unique/autocommit are
  restored.
- **Verify** carries its own `SET foreign_key_checks=1` before its checks; it does
  not assume the load left FK in any particular state.
- The dump's footer `SET FOREIGN_KEY_CHECKS=@OLD_...` becomes a harmless no-op from
  our safety's perspective — if present it simply re-affirms what our `defer`
  already guarantees; if absent or unreached, our reset still holds.
- Fast Mode's meaning is now precise: the tool *owns* the three variables end to
  end — sets them, runs on the one connection they live on, and resets them itself.
