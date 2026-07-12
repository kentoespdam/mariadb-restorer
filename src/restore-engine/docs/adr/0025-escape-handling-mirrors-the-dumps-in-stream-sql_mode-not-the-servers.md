# Escape handling mirrors the dump's in-stream `sql_mode`, not a hardcoded assumption or the live server's

ADR-0023 said the lexer tracks a "backslash-escape" state inside quoted strings. ADR-0024
pinned that the lexer scans raw bytes. Neither pinned *whether `\` is even an escape* —
and that is not a constant in MariaDB. The `NO_BACKSLASH_ESCAPES` sql_mode flips it: with
the mode off (the default) `\` escapes the next character; with it on, `\` is an ordinary
literal byte. The same bytes `'it\'s'` parse to **two different string boundaries**
depending on the mode, so if the lexer guesses the escape rule wrong it mis-places a `;`
and poisons a resume offset — the exact failure ADR-0023/0024 exist to prevent. This ADR
pins how the lexer decides.

## Why `\` cannot be treated as a constant

context7 `/mariadb-corporation/mariadb-docs` (*String Literals*):

> "The backslash character (`\`) is used to escape characters, **unless the
> `NO_BACKSLASH_ESCAPES` SQL_MODE is enabled**."

So there are two legal readings of a backslash inside a single-quoted string:

- **Backslash-escapes on (default):** `'it\'s ok'` — the `\'` is an escaped quote, the
  string is **still open** past it, and a `;` after the closing quote is the real
  boundary.
- **`NO_BACKSLASH_ESCAPES` on:** `'it\'` — the `\` is a literal byte, the second `'`
  **closes** the string, and `s ok'` is outside it. A `;` lands in a completely
  different place.

A lexer with a fixed escape rule is therefore correct for exactly one of the two server
configurations and silently wrong for the other.

## Why "let the server decide" cannot be taken literally for boundaries

The instinct — let the server, which owns `sql_mode`, decide — is right in spirit but
cannot be literal here. The server decides how to *decode a string during execution*, but
the lexer must know where a `'` closes **before it sends anything**, because it needs the
`;` boundary to cut a **Batch** and place a **Checkpoint**. The tool cannot send bytes and
then ask the server "was that string closed yet?" — boundary-finding happens upstream of
execution, so the lexer has to decide by itself. The server cannot help.

## Why NOT read the live server's `@@sql_mode` either

Reading `@@SESSION.sql_mode` off the target connection would decide the escape rule from
the *server's current* configuration — but that is the wrong authority. What matters is
the mode under which **the dump's string payloads were escaped**, which is whatever the
dump itself declares, not whatever the restore target happens to be set to. A dump written
with `NO_BACKSLASH_ESCAPES` and restored into a default server (or vice-versa) would be
mis-parsed if the lexer trusted the live server. It also repeats the coupling ADR-0003
rejected for `max_allowed_packet` — deriving parsing behaviour from live server state
rather than from the dump. The dump is the source of truth for how the dump is escaped.

## The mechanism: observe `SET sql_mode` in-stream, mirror it

`mariadb-dump` manages `sql_mode` as part of the session state it writes into the dump —
context7 shows sql_mode is dump-controlled state (MDEV-27816 "Set sql_mode before DROP IF
EXISTS"; the 10.6.8-4 note that "mariadb-dump does not correctly set the sql_mode ... in
the dump file" confirms setting it is the normal, expected behaviour). So the dump *tells
us* which escape rule its own payloads follow, via `SET sql_mode=...` statements in the
stream.

The lexer therefore does what it already does for `DELIMITER` (ADR-0023) and relies on for
`SET NAMES` (ADR-0024): it **reads the dump's own in-stream declaration** and mirrors it.
Concretely, the lexer carries one boolean `no_backslash_escapes` (default **false** —
escapes on, matching MariaDB's default), and updates it whenever it passes a
`SET sql_mode = ...` statement whose value list does / does not contain
`NO_BACKSLASH_ESCAPES`. From that point forward, quoted-string scanning honours the new
rule.

This has one deliberate difference from `DELIMITER` handling: **`SET sql_mode` is real
server SQL and is forwarded to the server verbatim** (the server needs it to decode data
correctly and to apply mode-dependent DDL semantics). The lexer *also observes* it in
passing. So `SET sql_mode` is **forwarded AND observed**, whereas `DELIMITER` is
**consumed and not forwarded**. The lexer never rewrites or suppresses `SET sql_mode`.

Because the dump escapes its payloads consistently with the very `sql_mode` it sets in its
own header, mirroring is guaranteed to stay aligned with the data — the tool does not need
to know the exact header bytes `mariadb-dump` emits (which were not confirmable via
context7); it only needs to react to whatever `SET sql_mode` it actually encounters.

## Decision

- The lexer tracks a boolean escape rule, defaulting to **backslash-escapes ON** (MariaDB
  default; `no_backslash_escapes = false`), and a second boolean `ansi_quotes`
  (default **false** — `"` is a string literal). Both are mirrored from the same
  `SET sql_mode` observation.
- The rule is updated by **observing `SET sql_mode = ...` statements in the dump stream**:
  if the assigned value contains `NO_BACKSLASH_ESCAPES`, `\` becomes a literal byte inside
  quoted strings from that point; if a later `SET sql_mode` drops it, escapes resume.
- `SET sql_mode` statements are **forwarded to the server verbatim** *and* observed by the
  lexer — unlike `DELIMITER`, they are not consumed. The lexer never edits them.
- The escape rule is **never** derived from the live target server's `@@sql_mode`. The
  dump is the sole authority for how the dump is escaped, consistent with ADR-0003's
  refusal to couple behaviour to live server state.
- This applies to the escape semantics inside single- and double-quoted string literals;
  `ANSI_QUOTES` (which changes what `"` *is*) is tracked by the same in-stream
  `SET sql_mode` observation as a second mirrored bit — see below.

## The companion mode: `ANSI_QUOTES` changes what `"` *is*

`NO_BACKSLASH_ESCAPES` changes *whether* `\` escapes; `ANSI_QUOTES` goes deeper and
changes *what the `"` byte means*. context7 `/mariadb-corporation/mariadb-docs`
(*Identifier Names*, error `e1149`):

> By default, double quotes (`"`) are not used for quoting identifiers. However, if the
> `ANSI_QUOTES` SQL mode is enabled, double quotes can be used to quote identifiers …
> Without this mode, double quotes are treated as string literals.

```sql
CREATE TABLE "SELECT" (i int);          -- ERROR 1064 without the mode
SET sql_mode='ANSI_QUOTES';
CREATE TABLE "SELECT" (i int);           -- OK: "SELECT" is now an identifier
```

So the same `"..."` bytes are a **string literal** under the default and a **quoted
identifier** under `ANSI_QUOTES` — and the two obey different in-region rules, which
changes where the region closes and therefore where a `;` boundary lands. `"a\";b"` has
two readings exactly as `'it\'s'` did.

What matters for **boundary-finding** is only *where the quoted region closes*, and that
is decided by the quote character, never by `\`:

- `"` as a **string** (default): closes on an unpaired `"`; a doubled `""` is a literal
  quote inside; `\` may escape the next byte *iff* `no_backslash_escapes` is off (the
  same bit above).
- `"` as an **identifier** (`ANSI_QUOTES` on): behaves like a backtick identifier
  (ADR-0023). context7 shows the only documented identifier escape is **doubling the
  quote character** — `sys.quote_identifier` doubles backticks (`` `a``b` ``) and never
  emits a `\`. So region-close is decided purely by the unpaired/doubled quote; `\` is
  irrelevant to it.

  *Honesty note:* context7 documents doubling as the identifier escape mechanism and
  never lists `\` as an identifier escape, but it contains no sentence that literally
  says "`\` is a literal byte inside an identifier". That `\` is escape-irrelevant inside
  an identifier is an **inference** from doubling being the sole documented mechanism —
  it is not quoted as a direct context7 statement. The design does not rest on it anyway:
  region-close for a `"`-identifier is the doubled-quote rule the lexer already runs for
  backticks, and `\` changes none of it.

Mechanism is identical to the backslash bit: the lexer carries a second boolean
`ansi_quotes` (default **false** — `"` is a string), updated by the **same**
`SET sql_mode` observation. One pass over a `SET sql_mode` value updates both bits. When
`ansi_quotes` is on, the scanner switches `"` from the string ruleset to the
backtick-identifier ruleset; when off, `"` uses the string ruleset (itself governed by
`no_backslash_escapes`).

`ANSI_QUOTES` therefore adds no new machinery and no new directive: it rides the
`SET sql_mode` statement that is already **forwarded to the server verbatim and observed
by the lexer**, and it reuses the doubled-quote region-close the lexer already has for
backtick identifiers.

## Considered Options

- **Hardcode backslash-escapes ON, ignore `sql_mode`:** rejected — correct only for the
  default server config, silently mis-parses every dump produced under
  `NO_BACKSLASH_ESCAPES`, placing boundaries inside string literals. The initial
  recommendation; withdrawn once context7 confirmed the mode genuinely changes boundary
  placement and that dumps carry their own `sql_mode`.
- **Read `@@SESSION.sql_mode` from the target connection:** rejected — decides escaping
  from the restore target's config, not from the mode the dump's payloads were written
  under; a mode mismatch between dump-origin and restore-target then corrupts parsing. Also
  reintroduces the live-server coupling ADR-0003 rejected.
- **Parse the dump's `SET sql_mode` but strip/suppress it (like DELIMITER):** rejected —
  `SET sql_mode` is valid server SQL the server needs for correct decoding and DDL
  semantics; consuming it would change execution behaviour. It must be forwarded; the
  lexer only *additionally* observes it.
- **Ask the server per-string (send bytes, query if the string closed):** rejected —
  impossible in principle; boundary detection is upstream of execution and the tool must
  cut Batches and place checkpoints before sending anything.

## Consequences

- The lexer's state (ADR-0023) gains two mirrored bits — `no_backslash_escapes` and
  `ansi_quotes` — alongside the mutable current-delimiter; all are updated by observing
  in-stream client/session declarations rather than by configuration. A reviewer who
  hardcodes the escape rule, hardcodes `"` as always-a-string, or reads either from the
  live connection reintroduces a boundary-corruption bug.
- `ansi_quotes` adds no code path of its own beyond a mode switch: when on, `"` is scanned
  with the existing backtick-identifier region-close (doubled-quote); when off, with the
  string region-close. The `\` escape logic lives entirely in the string ruleset gated by
  `no_backslash_escapes`, so the two bits compose without special cases.
- `SET sql_mode` joins `SET NAMES` and `DELIMITER` as the set of in-stream directives the
  lexer must recognise. Only `DELIMITER` is consumed; `SET NAMES` and `SET sql_mode` are
  forwarded, and `SET sql_mode` is additionally observed. This distinction is a contract:
  observing ≠ consuming.
- Because the escape rule can change mid-stream, the **Byte Offset** boundary correctness
  (ADR-0024) is sound only if the lexer's mode bit is current at each boundary — i.e. the
  lexer must apply a `SET sql_mode` change before scanning the strings that follow it.
  Ordering matters: observe-then-scan, never scan-then-observe.
- The tool carries **no `--sql-mode` flag** and no live `@@sql_mode` read for parsing. The
  dump's in-stream `SET sql_mode` is the single source of truth, mirroring how ADR-0024
  leaves charset to the dump's `SET NAMES`.
