# Drop custom CREATE VIEW interception; execute the dump linearly and verbatim

The PRD (§5.2) proposed intercepting `CREATE VIEW` statements and substituting a
hand-built placeholder table `(dummy INT)` to break view-on-view dependency
cycles during restore. We are **dropping this entirely**. `mariadb-dump` (10.11,
the confirmed source version) already emits a superior two-phase form: a
placeholder `/*!50001 CREATE TABLE ... */` stub with the *real* column names and
types, later `DROP`ped and replaced by `/*!50001 ... VIEW ... */`. Reproducing
this ourselves would be strictly worse (dummy column names break view-on-view
references that select named columns) and is redundant.

Consequently the restorer executes the dump as a **linear stream of statements,
sent verbatim** to the server. It never parses SQL semantically, never
rewrites bytes, and never decides what is "executable" — the server interprets
`/*!NNNNN ... */` and `/*M!NNNNN ... */` version gates itself. The
**Statement Splitter**'s only job is boundary detection: find the `;` that sits
in the `default` state (outside quotes/backticks/comments).

## Considered Options

- **Custom view interception (PRD §5.2):** rejected — inferior to the dump
  tool's native stubbing (wrong placeholder column names) and adds a semantic
  SQL-parsing burden the linear executor otherwise never needs.
- **Special-casing executable comments in the splitter (an earlier draft of
  this decision):** rejected — unnecessary. MariaDB docs confirm a bare `;` is
  illegal *inside* an executable comment, so `mariadb-dump` never places a
  terminator there. The splitter can treat `/*!`, `/*M!`, and `/*` identically.

## Consequences

- MVP features #4 (view interception) and #7 (placeholder substitution) are
  removed from scope.
- Scope assumption: input is always `mariadb-dump` output. Hand-written dumps
  with plain `CREATE VIEW` in dependency-wrong order are out of scope — no
  fallback is provided.
- `DEFINER=user@host` clauses on views/routines are passed through verbatim;
  restore assumes those users exist on the target. Handling missing definers is
  deferred (not part of this decision).
- Known limitation: a `*/` byte sequence *inside* a string literal that is
  itself inside a block comment could close the comment early in the byte-level
  state machine. Vanishingly rare in `mariadb-dump` view output; flagged as an
  MVP boundary rather than handled.
