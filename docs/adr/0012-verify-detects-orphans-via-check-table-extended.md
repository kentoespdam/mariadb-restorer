# Verify detects orphan rows via `CHECK TABLE … EXTENDED` per FK-bearing table, not a self-built anti-join

**Fast Mode** (ADR-0003) loads data with `foreign_key_checks=0`, so a `mariadb-dump`
whose tables are not in dependency order — or that references a parent row an
earlier `INSERT` never wrote — lands **orphan rows** the server accepted silently.
The optional **Verify** phase (`--verify`) exists to surface them. This ADR fixes
*how* Verify finds them, and rejects the tempting "just turn the check back on"
non-answer.

## Why re-enabling the check is not detection

The obvious first instinct — `SET foreign_key_checks=1` at the end and trust the
server — does nothing. Verified via context7 (MariaDB docs): disabling the check
*"does not retrospectively validate data consistency"*; re-enabling it only governs
**future** DML. Rows loaded at `fk=0` are never re-examined by the toggle. So Verify
cannot be a flag flip; it needs an explicit, active scan of already-resident data.

## Decision

Verify enumerates every child table that actually carries a foreign key — reading
`information_schema.REFERENTIAL_CONSTRAINTS` (and skipping tables with none) — and
runs **`CHECK TABLE <table> EXTENDED`** on each. Detection is delegated to the
**server**, not reimplemented in the tool. Verified via context7 (MariaDB docs),
`CHECK TABLE … EXTENDED` reports FK violations **per record, with the constraint
name**, e.g.:

```
Cannot add or update a child row: a foreign key constraint fails
(Key: t2_ibfk_1, record: '2')
```

Verify parses these rows out of the command's result set and reports them (offending
table, constraint, record) to the operator.

This is the same philosophy as ADR-0002: **the tool is not a SQL engine.** It does
not build referential-integrity logic; it drives the engine that already has one.

## The accepted trade-off: Verify reports more than FK violations

`CHECK TABLE … EXTENDED` runs a **full re-scan** of each table — it checks index
consistency and structural corruption, not only foreign keys. So Verify's report can
contain findings beyond orphan rows (a corrupt secondary index, say). This is
**accepted, not a defect**: `--verify` is an opt-in "is this restore sound?" phase,
and surfacing broader corruption a Fast-Mode bulk load could have masked is *more*
useful there, not noise. The cost — a full scan per FK-bearing table — is why Verify
is off by default and scoped to tables that have FKs.

## Considered Options

- **Re-enable `foreign_key_checks=1` and trust it (Path 0):** rejected — the toggle
  does not retro-validate (context7); it detects nothing already loaded.
- **Enumerate FKs and run a self-built anti-join (Path B):** rejected. It would give
  a *narrower* report (FK only, no index/structure noise), but the tool would have to
  reconstruct, per constraint, a `SELECT … FROM child LEFT JOIN parent … WHERE
  parent.key IS NULL AND child.fk IS NOT NULL` from `KEY_COLUMN_USAGE` — handling
  **composite keys**, **`MATCH SIMPLE` NULL semantics** (a partially-NULL composite FK
  is *not* checked), and **schema qualification** ourselves. That is exactly the
  SQL-authoring role ADR-0002 keeps the tool out of, for a marginal reporting-scope
  gain. If narrow-FK-only reporting ever becomes a hard requirement, this is the
  fallback — hence recording it here.
- **Skip detection, just warn "FK integrity unguaranteed":** retained as the
  *default* (Verify off) behavior, but rejected as the answer *when the operator opts
  in* — `--verify` must actually find the orphans, not restate the risk.

## Consequences

- Verify issues one `information_schema.REFERENTIAL_CONSTRAINTS` query to build the
  table list, then one `CHECK TABLE … EXTENDED` per FK-bearing child table — bounded
  by the number of such tables, not by row count in statements.
- Verify runs on the **single pinned connection** (ADR-0011) and re-asserts
  `foreign_key_checks=1` itself before scanning, not trusting the `fk=0` state the
  load left behind.
- The report is intentionally broader than FK violations; operator docs must frame
  `--verify` as a soundness check, not strictly a referential-integrity check.
- Verify is inherently expensive (full scan per table) and therefore stays **off by
  default**.
