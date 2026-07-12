# Statement splitting is a state-aware streaming lexer, not a naïve delimiter split

Every other decision in this project — constant-memory streaming, the **Batch** as the
unit of atomic progress, the **Byte Offset** the **Checkpoint Store** persists, the
one-batch **Resume Batch** invariant (ADR-0018) — rests on one primitive that has never
been pinned: *how the tool decides where one SQL statement ends and the next begins*
while reading a 9GB+ dump it can never hold in memory. This ADR pins that the splitter
is a **state-aware streaming lexer**, not a scan for a literal delimiter, because the
naïve alternative is silently wrong on real dumps and corrupts the checkpoint offset in
a way that only surfaces after a crash.

## Why a naïve `;\n` split is wrong, not just crude

The tempting implementation is "read until `;\n`, that's a statement." A `mariadb-dump`
file is not a text stream where that holds:

- **A `;` inside string data is not a terminator.** `INSERT INTO audit_log VALUES
  ('...); DROP TABLE x; --')` carries `;` and newlines *inside* the quoted payload.
  Splitting on `;\n` cuts mid-string, producing two fragments that are each a syntax
  error. Because `--extended-insert` is on by default (confirmed via context7
  `/mariadb-corporation/mariadb-docs`: "The `--extended-insert` option creates `INSERT`
  statements that include multiple rows … often enabled by default"), a single INSERT
  can be megabytes of multi-row data — a huge surface for a stray delimiter to hide in.
- **Quoting and escaping hide false delimiters.** `'it\'s'`, doubled `''`, backslash
  escapes, and backtick-quoted identifiers (`` `weird;col` ``) all contain characters
  that look like delimiters but are not. Deciding correctly requires tracking lexer
  state: inside a single-quoted string? a backtick identifier? just saw a backslash?
- **`DELIMITER` redefines the terminator — and is a client command, not SQL.** context7
  confirms `DELIMITER` is a command of the **mariadb command-line client**, not a server
  statement ("Use the DELIMITER command to change the statement delimiter when creating
  stored routines that contain semicolons within their body"). A dump taken with
  `--routines`/`--events` (or the always-included triggers) switches the delimiter to
  e.g. `//` around a `BEGIN … ; … END` body, where the interior `;` are **not**
  statement ends. A splitter that does not itself interpret `DELIMITER` shreds every
  stored-program body into invalid fragments.
- **Executable comments are code, and `;` inside them is not a terminator.** MariaDB
  runs version-gated comments `/*!40101 … */` (MySQL-compat) and `/*M!100100 … */`
  (MariaDB-specific). context7 shows `/*M!100100 select 1 ; */` is itself a **syntax
  error** because a delimiter inside an executable comment is not permitted — so the
  lexer must know it is inside such a comment and must not treat the enclosed `;` as a
  boundary, while still distinguishing these from ordinary `-- ` / `/* */` comments it
  skips.

None of these are exotic; `--routines` dumps, escaped strings, and multi-row INSERTs are
the *normal* shape of a production dump. A naïve split is not "good enough for the common
case" — the common case is exactly where it breaks.

## Why this is a correctness precondition for the checkpoint, not just a parsing detail

The splitter defines two things the rest of the system trusts as ground truth:

1. **What counts as a statement** — the unit the Batch threshold counts toward (~1000
   statements secondary bound, ADR-0009 / CONTEXT.md **Batch**). A miscount here makes
   batches the wrong size, but that is the benign failure.
2. **Where a statement boundary falls in the byte stream** — and the **Byte Offset** is
   only ever written on a boundary. If the lexer mis-identifies a boundary inside a
   quoted string, the persisted offset points into the middle of a string literal. On
   resume the tool `Seek()`s there and begins executing mid-literal — the same
   silent-corruption class ADR-0019 refuses for a stale offset, except here it is
   *self-inflicted* on a perfectly matching file.

So lexer correctness is upstream of checkpoint correctness: a wrong boundary is not a
parse error you notice immediately, it is a poisoned resume point that only detonates
after a crash. This is why it is worth an ADR and not left to "obvious implementation."

## The `DELIMITER` consequence: it is consumed, never forwarded

Because `DELIMITER` is a client-side command the server does not understand, the lexer
does not just *tolerate* it — it must **interpret and consume** it:

- A `DELIMITER //` line changes the lexer's current terminator to `//` and is **not**
  emitted to the server.
- The stored-program body up to the next `//` is emitted as **one** statement, with the
  custom terminator stripped (the server is sent the `CREATE TRIGGER … END`, not the
  trailing `//`).
- `DELIMITER ;` restores the default and is likewise consumed.

This mirrors exactly what the `mariadb` client does when you pipe a dump into it, and it
is the behaviour a dump author assumes. A tool that executes statements over a Go driver
connection (not by shelling out to the `mariadb` client) has no free lunch here — it
*is* the client for parsing purposes and must own this logic.

## Decision

- Statement splitting is performed by a **state-aware streaming lexer** that reads the
  dump in bounded chunks (never buffering more than the current statement/Batch) and
  emits complete statements on true boundaries. It is **not** a scan for a literal
  `;` / `;\n`.
- The lexer tracks at minimum: single-quoted strings (with `\`-escape and doubled-quote
  `''`), backtick-quoted identifiers, `-- ` and `#` line comments, `/* */` block
  comments, **executable** comments `/*!…*/` and `/*M!…*/` (contents are code; interior
  `;` is not a boundary), and the **current delimiter** as (re)defined by `DELIMITER`.
- `DELIMITER` is a **client command**: the lexer interprets it, changes its active
  terminator, and does **not** forward the `DELIMITER` line or the custom terminator to
  the server. Stored-program bodies are emitted as single statements.
- A "statement" for **Batch** counting and for **Byte Offset** placement is defined
  solely by this lexer; the Byte Offset is only ever persisted on a lexer-confirmed
  statement boundary, so a resume `Seek()` always lands between statements, never inside
  a literal.
- What `mariadb-dump` actually emits (extended multi-row INSERTs by default; `DELIMITER`
  around routines/triggers with `--routines`/`--events`; executable comments) is treated
  as the design's ground truth, verified against context7 `/mariadb-corporation/mariadb-docs`,
  not assumed.

## Considered Options

- **Naïve split on `;` or `;\n`:** rejected — silently produces invalid fragments on
  quoted `;`, escaped strings, `DELIMITER` blocks, and executable comments, all of which
  are normal in production dumps. Worse than a parse bug: it can place a Byte Offset
  inside a string literal, poisoning resume on an otherwise-matching file.
- **Line-oriented split (statement = one line):** rejected — an extended-insert INSERT is
  one logical statement spanning many lines *and* a multi-line `BEGIN … END` body is one
  statement; lines correspond to neither statements nor boundaries.
- **Split naïvely but re-drive `DELIMITER` by forwarding it to the server:** rejected —
  the server does not understand `DELIMITER` (it is a client command per context7), so
  forwarding it is itself a syntax error, and it does nothing to fix the quoted-`;`
  problem.
- **Shell out to the `mariadb` client and pipe the dump into it:** rejected here as the
  splitting strategy — it would hand off parsing correctly but forfeits everything this
  project is built on: per-statement/Batch control, the byte-accurate **Byte Offset**,
  resume, progress, and Tolerated-Error handling all require the tool to see statement
  boundaries itself. (Interop with the native client's *format* is honoured; delegating
  execution to it is a different tool.)
- **Full SQL grammar parser (parse each statement into an AST):** rejected — far more
  than boundary detection needs, slow on 9GB+, and pointless because the tool re-sends
  statement text verbatim to the server rather than interpreting it. The lexer needs to
  find boundaries, not understand semantics.

## Consequences

- The lexer's state set is a fixed contract: string/identifier/comment/executable-comment
  states plus a mutable current-delimiter. A reviewer adding a new dump feature (a new
  quoting or comment form) must extend the lexer, not the callers, because boundary
  correctness lives in exactly one place.
- The **Byte Offset** invariant now has a named upstream dependency: it is sound only if
  every persisted offset sits on a lexer-confirmed boundary. This ties directly to
  ADR-0018 (one-batch replay) and ADR-0019 (no seek into a stale/!matching stream) — all
  three assume boundaries are real.
- `DELIMITER` handling means the bytes the tool sends to the server are **not** always a
  verbatim byte-for-byte copy of the dump slice (the `DELIMITER` lines and custom
  terminators are stripped). The Byte Offset still advances over the *dump's* bytes
  (including the consumed `DELIMITER` lines), so resume accounting stays in dump-file
  coordinates; only what is forwarded to the server differs.
- Because the lexer defines the statement count, the Batch ~1000-statement secondary
  bound (ADR-0009) is measured in lexer-statements; a single extended INSERT is one such
  statement regardless of row count, and Batch sizing must keep using the ~64MB primary
  byte bound so one giant INSERT does not blow the batch.
- For **boundary detection** the lexer need not tell an executable comment (`/*!…*/`,
  `/*M!…*/`) apart from a plain `/* */` comment: both are regions where an interior `;`
  cannot terminate a statement, and both close on `*/`. The tool forwards all comment
  bytes **verbatim** — it skips/strips nothing, so the server (not this tool) decides
  whether a version-gated comment runs. The distinction matters to the *server*, not to
  the splitter; conflating them for boundary purposes is correct, not a bug. (A bare `;`
  inside an executable comment is itself a syntax error per context7, so `mariadb-dump`
  never emits one there — the splitter is never asked to make that call.)
