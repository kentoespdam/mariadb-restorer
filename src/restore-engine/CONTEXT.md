# Context: restore-engine

Core restore execution logic — parsing SQL dumps, managing checkpoints, handling batches and transactions, and coordinating the restore lifecycle.

## Language

**Dump**:
A `mariadb-dump`-generated `.sql` file being restored. Machine-generated and
regular in structure — not arbitrary hand-written SQL.
_Avoid_: backup, export, file.

**Statement**:
A single executable SQL unit terminated by the **current delimiter** (default
`;`, redefinable mid-stream by a `DELIMITER` client command) that sits outside
any string literal or comment. The atomic unit of execution and checkpointing.
_Avoid_: line, query, command.

**Statement Boundary**:
The byte position immediately after a terminating `;`. The only position a
**Checkpoint** may legally record — a boundary is valid only when the splitter
is in its `default` state (not inside a quote, backtick, or comment).

**Statement Splitter**:
The state-aware streaming lexer that consumes the **Dump** stream in bounded
chunks (never buffering beyond the current **Statement**/**Batch**) and emits
**Statements** on true boundaries — NOT a scan for a literal `;`/`;\n`. Tracks:
default, single-quote, double-quote, backtick, backslash-escape, line-comment
(`--`, `#`), block-comment (`/* */`), **Executable Comment** (`/*! */`,
`/*M! */`), and the **current delimiter** as redefined by `DELIMITER`. Most
state exists to suppress false `;` termination, and the splitter sends every
**Statement** to the server verbatim — with ONE exception: `DELIMITER` is a
client command (not server SQL), so the splitter interprets and *consumes* it,
changing its active terminator and emitting a stored-program body as one
**Statement** without forwarding the `DELIMITER` line or custom terminator.
Two boundary-affecting rules are likewise not fixed and are mirrored from the
same source: whether `\` escapes inside a quoted string (`no_backslash_escapes`,
default off = escapes on, the MariaDB default) and whether `"` is a string
literal or a quoted identifier (`ansi_quotes`, default off = `"` is a string).
The splitter updates both bits by *observing* `SET sql_mode` statements in the
stream — those are **forwarded to the server AND observed** by the splitter,
never consumed. Under `ansi_quotes`, `"` closes by the same doubled-quote rule
the splitter already runs for backtick identifiers. Both rules are taken from the
**Dump**'s own in-stream `SET sql_mode`, never from the live server's
`@@sql_mode` (mirrors ADR-0003's no-live-coupling and ADR-0024's
charset-from-`SET NAMES`). See ADR-0025.
The splitter alone defines what a **Statement** is (for **Batch** counting) and
where a **Byte Offset** may fall — so its correctness is upstream of checkpoint
correctness (a boundary mis-placed inside a literal poisons resume). It scans
raw bytes, never decoded runes: every structural token is ASCII (<0x80), and no
byte of a UTF-8 multibyte character is <0x80, so byte-scanning cannot mistake a
multibyte fragment for a delimiter and needs no encoding detection. This is
sound for every restorable **Dump** because MariaDB forbids the
ASCII-incompatible charsets (`utf16`/`utf16le`/`ucs2`/`utf32`) as client charsets
— a dump in one of those cannot be restored by any client, so it is outside the
valid-input set. See ADR-0023, ADR-0024.
_Avoid_: naive splitter, parser, tokenizer (it is a boundary lexer, not a full
SQL grammar and not a `;` scan).

**Executable Comment**:
A versioned comment the server may run: `/*!NNNNN ... */` (MySQL-compat, e.g.
`/*!40101 SET ... */`, `/*!50001 ... VIEW ... */`) or `/*M!NNNNN ... */`
(MariaDB-specific). The server, not this tool, decides whether to run it based
on its own version — we pass it verbatim, never unwrap it. One **Statement** may
contain several segments (e.g. `50001` + `50013`). Note: a bare `;` inside an
executable comment is a syntax error, so `mariadb-dump` never emits one there —
which is why the **Statement Splitter** need not distinguish executable comments
from plain `/* */` comments; both are just regions where `;` cannot terminate.

**View Stub**:
The placeholder table `mariadb-dump` emits for a view (real column names, dummy
types) so view-on-view references resolve during restore, later dropped and
replaced by the real `CREATE VIEW`. Produced by the dump tool — NOT by us.
_Avoid_: dummy table (we do not create our own; §5.2's custom stubbing is dropped).

**Pre-flight Check**:
The startup phase that validates the environment before any **Statement** runs:
it reads (never sets) `@@global.max_allowed_packet` and aborts with an actionable
message if the server cannot accept the largest **Statements** a **Dump** may
contain. It also sets the speed-tuning session variables (`autocommit=0`,
`unique_checks=0`, `foreign_key_checks=0`) on the **single pinned connection**
that runs the entire restore — a shared pooled connection would break **Batch**
correctness, since `autocommit=0` on one connection does not carry to another. We
*read, never set*
`max_allowed_packet` deliberately: it is the server admin's dial (managed hosts
may forbid raising it), and setting it silently would hide a real capacity limit.
When it is too small the tool tells the operator exactly how to tune it — see the
actionable-tuning contract on **Fatal Error**.
_Avoid_: setup, init (those are generic); this is specifically the guard phase.

**Checkpoint**:
The **Byte Offset** of the last **COMMIT**ted **Statement Boundary**, persisted
to the local **Checkpoint Store** so a crashed restore resumes via `file.Seek()`.
Written only *after* MariaDB confirms the COMMIT — never per-statement. Besides
the byte/statement thresholds of a **Batch**, a Checkpoint is also forced right
before every **Implicit-Commit Statement**, so the server's own implicit COMMIT
never advances durability past a boundary we have not yet recorded.
_Avoid_: bookmark, savepoint (savepoint means something specific in SQL).

**Checkpoint Store**:
The local pure-Go SQLite file (`modernc.org/sqlite`, no cgo) holding one row per
**Dump**, keyed by `dump_identity`: `dump_path`, `dump_size_bytes`,
`dump_identity`, `byte_offset`, `statements_done`, `no_backslash_escapes`,
`ansi_quotes`, `current_delimiter`, `charset`, `updated_at`. Each COMMIT
overwrites *that dump's* row (an UPSERT on `dump_identity`) — no history is kept
per dump, only "where do we resume *this dump*". The four lexer-state columns
(`no_backslash_escapes`, `ansi_quotes`, `current_delimiter`, `charset`) snapshot
the **Statement Splitter**'s state at `byte_offset`, ensuring correct boundary
detection on resume (ADR-0026): the tool restores not just *where* to resume but
*how to parse* from that byte. Keying by `dump_identity` (not a single global row)
is what lets a crashed **dump A** stay resumable after the operator starts an
unrelated `restore dump-B.sql`: B gets its own row and never clobbers A's resume
point. Lives on local disk, never in the target MariaDB. A dump's row is
**deleted on full successful completion** — the store holds only *unfinished*
restores, so its size is bounded by the number of restores currently pending
(normally zero). This gives resume its vocabulary as a *behaviour*, not a
command (there is no `resume` verb — ADR-0020): a row present means the next
`restore <file>` continues it, a row absent means "nothing to continue — already
completed or never started" (the two are distinguished by whether the tool has ever
seen that `dump_identity`). A crash in the narrow window *after* the final COMMIT but
*before* the DELETE leaves a benign idle row whose `byte_offset` already equals
EOF; the next `restore <file>` seeks to EOF, finds no work, and deletes the row — the
cleanup is idempotent, so the row never outlives one further run.
Opened with `PRAGMA synchronous=FULL`: each checkpoint write is fsync-durable
before the next batch is COMMITted to MariaDB, which is what keeps the resume
replay window at most one batch — the precondition the **Resume Batch** tolerance
depends on (ADR-0018). Fast Mode is tuned by **Batch** size, never by lowering
`synchronous` or checkpointing less often.
_Avoid_: journal, log (it keeps no per-dump history — each row is a single
mutable position for its dump).

**Dump Identity**:
A fast fingerprint of the **Dump** — a hash of its first few KB plus its total
size — stored in the **Checkpoint Store**. On resume the tool recomputes it for
the file it was handed; it is the **session key** deciding one of exactly two
outcomes, never a third "seek anyway":
- **match** → resume from the stored **Byte Offset**. Auto-resume is never silent:
  a plain `restore <file>` that finds a matching row always prints a notice ("found
  checkpoint for this dump (N% done) — resuming from byte X; use `--restart` to reload
  from the beginning"), symmetric with the mismatch notice below (ADR-0020). There is
  no separate `resume` verb — `restore <file>` resumes on its own, and `--restart` is
  the only override.
- **mismatch** → the file at this path differs from the one checkpointed (the
  operator regenerated or swapped it), so the stale offset is meaningless for this
  byte stream. The tool **automatically restarts from byte 0**, discarding the old
  `dump_identity` row, and **always prints a notice** ("dump at this path differs
  from the previous checkpoint (N% done) — restarting from the beginning; the old
  checkpoint is discarded"). Auto-restart is safe on the default path because
  `mariadb-dump --opt` (on by default) emits `DROP TABLE IF EXISTS` before each
  `CREATE TABLE`, so a dirty target is dropped and reloaded cleanly; a
  `--no-create-info`/`--skip-opt` data-only dump carries no DROP and reloading it
  onto existing rows is the operator's own concern (ADR-0019).
It is NOT a hash of the whole 9GB+ file (too slow to compute at every startup),
so it cannot catch a **same-size in-place content edit** (operator fixes one
statement mid-file, total byte count unchanged): head + size still match, so the
tool resumes and `Seek()`s past the edited region — an edit *before* the offset is
never executed, and the restore "succeeds" without the fix. This is a documented
limit, not a silent one: the fingerprint's contract is **"detects regeneration and
swap, does NOT guarantee detection of a same-size in-place edit."** The operator's
conscious escape hatch for that case is `--restart` (below) — the fingerprint is
kept cheap (head-prefix + size, O(1) startup) rather than paying a full-file or
skip-region hash on every 9GB+ run to close a pathological case (ADR-0019).
`--restart` has one job across both cases: force a from-byte-0 restart. On a
**mismatch** it is not needed (restart is automatic); its real purpose is the
**false-positive match** the fingerprint cannot see — "I edited this file, discard
the checkpoint and reload even though the fingerprint matches."
_Avoid_: checksum, digest (be specific: head-prefix + size, not full-file).

**Batch**:
The set of **Statements** executed inside one MariaDB transaction between two
**COMMIT**s. The unit of atomic progress. A **COMMIT** is triggered by whichever
threshold is crossed first: **~64 MB** of statement bytes accumulated (primary,
byte-driven — a single extended INSERT can be hundreds of MB, so counting rows is
meaningless) or **~1000 Statements** (secondary — caps dumps of many tiny
single-row INSERTs). Both are defaults exposed as `--commit-bytes` /
`--commit-statements`, overridable but sensible without configuration.
**Minimum-one rule:** a single **Statement** larger than the byte threshold is
executed *alone* and committed as a one-statement Batch — never split (splitting
would violate the **Statement Splitter**'s verbatim contract). The threshold means
"COMMIT *after* crossing", not "never exceed"; the huge Statement still runs whole,
bounded only by the server's `max_allowed_packet`, then commits immediately.
_Avoid_: chunk, page (a Batch is a transaction boundary, not a fixed-size slice).

**Progress**:
What the tool reports during a long restore. The percentage is **byte-based** —
`byte_offset / dump_size_bytes` — because `dump_size_bytes` is known for free at
startup (**Dump Identity**) while the total **Statement** count is not, and getting
it would mean pre-scanning the whole 9GB+ file, breaking the constant-memory stream
(ADR-0021). `statements_done` is shown as a secondary counter, never as the
percentage's basis. The numerator is the **same `byte_offset` the Checkpoint Store
persists**, not a parallel tally: the bar is a live readout of the resume point, so
what the operator sees is exactly where a crash would resume from — the two cannot
diverge. Rendering is **TTY-adaptive** on stdout, decided by `term.IsTerminal`
(the same primitive ADR-0017 uses to gate the stdin prompt): an interactive terminal
gets one status line rewritten in place (`\r`) with percent, MB/s, and ETA, throttled
~1/sec; a pipe/file/scheduler gets **no `\r`** but append-only, timestamped progress
lines at a coarse cadence, so a redirected log stays greppable and `tail -f`-able
instead of smearing into one overwritten line. `--progress=auto|plain|none` overrides
the detection; `auto` is the default and needs no configuration to do the right thing
in either environment. If `dump_size_bytes` is unknown (e.g. a non-seekable pipe), the
percentage degrades to "unknown" and the tool shows bytes-done plus throughput rather
than dividing by zero.
_Avoid_: bar, spinner (name the number and its source, not the widget); the display
is byte-driven and checkpoint-bound, not a cosmetic animation.

**Implicit-Commit Statement**:
A **Statement** the server force-commits the current transaction *before*
executing — primarily DDL. MariaDB (context7) does this for `CREATE`/`ALTER`/
`DROP` (tables, indexes, views, routines, events, databases), `RENAME`,
`TRUNCATE`, `LOCK TABLES`, `UNLOCK TABLES`, `LOAD DATA`, `FLUSH`, `RESET`,
`OPTIMIZE`, and `START TRANSACTION`/`BEGIN` — and **commits even if the statement
then fails**. `mariadb-dump` emits several per table (`LOCK TABLES … WRITE`,
`DROP TABLE IF EXISTS`, `CREATE TABLE`, `ALTER TABLE … DISABLE KEYS`,
`UNLOCK TABLES`). The danger: the server's implicit COMMIT lands at a **Statement
Boundary** we did *not* choose, so an open **Batch**'s data becomes durable before
our **Checkpoint** records it — a crash there would replay already-committed data.
The executor therefore treats each one as a **forced Batch boundary**: it COMMITs
the open Batch and writes the **Checkpoint** *immediately before* running the
statement, so the server's implicit COMMIT can never precede our Checkpoint.
Exceptions honoured (context7): `CREATE/DROP TEMPORARY TABLE` do **not** implicit-
commit (excluded from the match); `TRUNCATE` and `CREATE/DROP INDEX` always do,
even on temp tables. Detection is an explicit **prefix allowlist** checked only at
a Statement's start in the splitter's `default` state — never a "first word looks
like DDL" heuristic, which would both miss `LOCK TABLES` and mis-hit `TEMPORARY`.
_Avoid_: DDL (too narrow — `LOCK TABLES`/`LOAD DATA` also commit and are not DDL).

**Resume Batch**:
The first **Batch** executed after a crash-resume. Run with error tolerance
(only the **Tolerated Errors** ignored) to absorb the narrow window where a
COMMIT succeeded but its **Checkpoint** was not yet written.

**Tolerated Error**:
The exact, narrow set of MariaDB server errors ignored — and *only* inside the
**Resume Batch**: `1062` (duplicate entry) and `1050` (table already exists).
Everything else is **Fatal**. A `1062`/`1050` during a normal **Batch** is NOT
tolerated — outside the replay window it means something is genuinely wrong.
_Avoid_: recoverable error, warning (be exact: two codes, one phase).

**Fatal Error**:
Any Batch error that is not a **Tolerated Error** in the **Resume Batch**. It
triggers fail-fast: roll back the open transaction (the **Checkpoint** never
advanced), print the server error + failing **Byte Offset** + statement number +
truncated snippet, and exit non-zero so the operator fixes the cause and re-runs
`restore <file>` (which auto-resumes — ADR-0020). Includes `1146` (no such table),
`1153` (packet too large), `1064` (syntax error — a **Statement Splitter** bug
signal), and connection drops (`2006`/`2013`, surfaced in Go as `driver.ErrBadConn`).
No auto-retry in MVP — the resume path is the retry mechanism.
**Actionable-tuning contract:** when the Fatal Error is a capacity limit the
operator can lift — chiefly `1153` (a **Statement** exceeds
`max_allowed_packet`) — the message does not just report the code, it prints the
fix: raise the ceiling *both* at runtime and persistently (`SET GLOBAL
max_allowed_packet=<bytes>;` **and** add it to `my.cnf`, because `SET GLOBAL`
alone is lost on restart — MariaDB docs, context7), then re-run `restore <file>`.
Re-running opens a fresh connection whose **Pre-flight Check** re-reads the
now-larger ceiling. The tool never raises `max_allowed_packet` itself.
_Avoid_: crash, exception (it is a controlled, resumable stop).

**Deferred Object**:
A single, deliberately narrow exception to **Fatal Error**: a `CREATE
VIEW/TRIGGER/ROUTINE/EVENT` **Statement** that fails *at create time* because of
its `DEFINER=` clause. The dominant, common cause is `1227`
(ER_SPECIFIC_ACCESS_DENIED_ERROR) — the restoring account lacks `SET USER`/
`SUPER`, so it may not assume another account as DEFINER (the managed-DB /
restore-as-app-user case). `1449` (ER_NO_SUCH_USER) is also caught as a
create-time net, though per MariaDB docs (context7) a missing DEFINER *account*
is normally checked when the object is **referenced** (view queried, trigger
fires) — i.e. at runtime, outside this tool — not at create. So when the restore
runs with full privilege (e.g. as `root`), these statements simply SUCCEED and
nothing is deferred; deferral is the restricted-privilege path. On a match the
tool records the *verbatim* statement text plus its **Byte Offset**, error code,
and object identity into the **Deferred Store**, prints a warning, and continues.
Safe to defer for two server-guaranteed reasons: these are DDL, so each one
implicit-COMMITs on its own (no open **Batch** to poison), and a failed statement
does not roll back a transaction. Scope is exact: ONLY `1227`/`1449`, ONLY on
object-creating DDL. Those codes in any other context, and every other error,
stay **Fatal**. Never used for `INSERT` data — only small metadata statements
are worth storing to replay.
_Avoid_: skipped statement, ignored error (it is captured for **Replay**, not
dropped — and it changes the exit code).

**Deferred Store**:
The table (in the same **Data Directory** SQLite file as the **Checkpoint
Store**) holding one row per **Deferred Object**: verbatim statement text,
**Byte Offset**, error code, object identity (`db`, name, type), and
`deferred_at`. Unlike the single-row **Checkpoint Store**, this is an
append-many queue drained by **Replay**.
_Avoid_: dead-letter queue (fine as an analogy, but name it Deferred Store here).

**Replay**:
The `replay` subcommand that re-executes every **Deferred Object** from the
**Deferred Store** after the operator has fixed the cause (created the missing
DEFINER users, or granted the restoring account `SET USER`). Runs as a
**fixed-point iteration**, not one pass: each round executes all remaining rows;
a row that succeeds is deleted; if a round completes ≥1 object it loops again;
it stops when a full round makes no progress (the rest are genuinely blocked).
This drains deferred *view-on-view* chains automatically — `v_summary`
depending on a still-deferred `v_report` succeeds on the round after `v_report`
does — without the tool having to parse view dependencies. Reference-time errors
during replay (`1146` no such table, `1356` view-references-invalid) are not
fatal in `replay`; the object stays queued for the next round. **Interrupt-safe**:
because each successful object is immediately deleted from the store, an interrupt
(Ctrl-C) in the middle of replay only leaves objects that haven't been tried — 
re-run `replay` continues from the remainder without repeating objects that already
succeeded. Follows the same **Interrupt** two-signal pattern: first signal graceful
drain (complete the object currently executing, delete its row if successful, then
exit), second signal abort immediately. Lets an admin fix a handful of metadata objects in seconds
without re-running the 9GB data load.
_Avoid_: retry, resume (resuming continues an interrupted data load and is
automatic inside `restore` — not a verb; `replay` is a real subcommand that drains
deferred metadata — different phases, different stores).

**Interrupt**:
An operator deliberately stopping a running restore — Ctrl-C at a terminal
(`os.Interrupt`, portable on Linux and Windows) or `SIGTERM` from a scheduler
(Unix only). Unlike a crash it arrives with the process alive and in control, so
the tool spends that warning: the **first** signal triggers a *graceful drain* —
finish the in-flight **Batch** through its COMMIT + checkpoint write, print a
resume notice, exit clean with the **Byte Offset** exactly on a batch boundary, so
the next `restore <file>` replays **zero** batches. A **second** signal *aborts
immediately* onto the crash-resume path (replay ≤ one batch, absorbed by the
**Resume Batch** — never unsafe, only less clean). Both leave the checkpoint row
intact; an interrupt is just a cleaner entry into the same resume story. Armed by
an explicit counting `signal.Notify` handler, not bare `signal.NotifyContext`
(which suppresses the default exit for later signals and would swallow the second
Ctrl-C). Exits with the interrupt **Exit Code** (`130`/`143`).
_Avoid_: cancel, kill, abort (reserve *abort* for the second-signal fast stop;
*Interrupt* is the whole two-signal contract).
