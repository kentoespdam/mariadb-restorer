# Fast-mode variables are SESSION-scoped; final FK integrity is opt-in via --verify

Restore speed comes from disabling real-time constraint checks
(`autocommit=0`, `unique_checks=0`, `foreign_key_checks=0`). We set these with
`SET SESSION`, never `SET GLOBAL`: they affect only our connection, and a crash
that drops the connection discards them automatically — so there is no "reset on
crash" step. A resumed restore opens a fresh connection and re-applies them in
the **Pre-flight Check**.

Because `foreign_key_checks=0` permanently accepts constraint-violating rows —
MariaDB docs (context7): *"Disabling checks does not retrospectively validate
data consistency"* — re-enabling the variable does NOT re-validate existing data.
A restore can therefore leave the database silently violating referential
integrity if the dump was inconsistent. We do not pay a full-scan validation cost
on every restore, but we are honest about the gap: an optional `--verify` flag
(default off) runs referential-integrity checks after a successful restore, and
the tool always prints a warning that FK integrity is not guaranteed unless
`--verify` was run.

## Considered Options

- **GLOBAL scope for the fast-mode variables:** rejected — mutates shared server
  state affecting other connections, and needs privilege (same reasoning as
  ADR-0003).
- **Explicit "reset variables on crash":** rejected — impossible and unnecessary;
  a dropped connection already discards session state.
- **Always run full FK verification (no flag):** rejected — a full referential
  scan on a 9GB+ restore is prohibitively expensive for the common case of a
  known-consistent `mariadb-dump` source.
- **Never verify, stay silent:** rejected — leaves silent integrity violations
  with no signal to the operator.

## Consequences

- `--verify` is off by default; the post-restore warning about unverified FK
  integrity is always emitted so the trade-off is never hidden.
- Verification strategy (anti-join per FK vs `CHECK TABLE`) is an implementation
  detail chosen later; both are acceptable behind the flag.
