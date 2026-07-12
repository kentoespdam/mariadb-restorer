# MariaDB Restorer

A resilient, streaming CLI tool that restores massive MariaDB/MySQL SQL dump
files (9GB+) with constant memory, crash-resume via byte-offset checkpoints,
and speed optimizations that bypass real-time constraint checking.

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

**Fast Mode**:
The session state that trades constraint safety for speed: `autocommit=0`,
`unique_checks=0`, `foreign_key_checks=0`, set with `SET SESSION` during the
**Pre-flight Check**. Because it accepts FK-violating rows permanently (the
server never retro-validates), it makes the optional **Verify** phase meaningful.
The tool **owns** these three variables end to end: it sets them, runs the whole
restore on **one pinned connection** they live on, and **resets all three to `1`
on every exit** — clean completion, **Fatal Error**, or panic — before that
connection returns to Go's pool. It never depends on the **Dump**'s footer
`SET FOREIGN_KEY_CHECKS=@OLD_...` to restore integrity state: that footer runs only
on a clean finish, so a mid-restore failure would otherwise leave `fk=0` on a
pooled connection our own process could reuse (MariaDB isolates sessions from
*other clients*, but Go `database/sql` pools connections *within* this process).
Contrast the **Dump**-owned session settings (TIME_ZONE, SQL_MODE, charset/NAMES,
SQL_LOG_BIN) passed through verbatim.
_Avoid_: bulk mode, turbo (be literal about which checks are off).

**SQL_LOG_BIN**:
The session variable controlling binary logging for replication. `SET sql_log_bin=0`
disables binary logging for the current session — restore operations do not enter
the binlog and will not replicate to downstream servers. `SET sql_log_bin=1`
enables it — operations enter the binlog (if the server has binlog enabled) and
replicate downstream if replication is configured. Requires `BINLOG ADMIN` privilege
to modify. The tool treats `sql_log_bin` as **Dump**-owned (ADR-0029): it forwards
any `SET sql_log_bin` statement the **Dump** contains verbatim, never queries
`@@sql_log_bin`, never sets it itself, never warns about binlog state. If the dump
contains `SET sql_log_bin=0` (common for dumps intended to restore on replicas
without propagating changes), the tool forwards it and restore operations do not
replicate. If the dump does not contain this statement, the server default applies
(usually `sql_log_bin=1`) and restore operations replicate downstream if replication
is configured. The replication implications are the operator's decision: some
operators want restore operations to replicate (restore to primary, downstream
replicas sync automatically), some want restore operations isolated (restore to
replica for local testing, no upstream propagation). The tool does not assume —
the dump's in-stream declaration is the sole authority. No `--sql-log-bin` flag exists.
_Avoid_: binlog mode, replication control (be literal: sql_log_bin session variable).

**Verify**:
The optional post-restore phase (`--verify`, default off) that surfaces the
referential-integrity violations **Fast Mode** let in. It cannot rely on merely
re-enabling `foreign_key_checks=1`: MariaDB (context7) *"does not retrospectively
validate data consistency"* when the check is turned back on, so orphan rows
loaded at `fk=0` sit silently until something reads them. Verify instead **asks
the server to re-scan** — it enumerates each child table carrying a foreign key
(via `information_schema.REFERENTIAL_CONSTRAINTS`) and runs `CHECK TABLE <t>
EXTENDED` on it, parsing the per-record FK failures the server reports (`Cannot
add or update a child row: a foreign key constraint fails (Key: …, record: …)`).
It re-asserts `foreign_key_checks=1` itself before this rather than trusting the
connection's inherited state — it runs on the same pinned connection the load just
left at `fk=0`. Because `CHECK TABLE EXTENDED` re-scans the whole table, Verify
also surfaces index/structural corruption, not only FK orphans — accepted scope
for an opt-in "is this restore sound?" phase. Its report **separates two finding
classes**: FK violations (orphan rows, named by constraint and record) and
structural `Corrupt`, the latter flagged as *possibly* a false positive because
MariaDB (context7) can mis-report MyISAM/Aria tables as corrupt from an inaccurate
column-prefix hash computation. Verify therefore **never escalates `Corrupt` to a
Fatal Error**: both classes yield **Exit Code** `4` (a post-completion state, not
the mid-run fail-fast `1` is for) and leave the judgement to a human. When off,
the tool still warns that FK integrity is unguaranteed.
_Avoid_: validate, check (name the flag).

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
fatal in `replay`; the object stays queued for the next round. Lets an admin fix
a handful of metadata objects in seconds without re-running the 9GB data load.
_Avoid_: retry, resume (resuming continues an interrupted data load and is
automatic inside `restore` — not a verb; `replay` is a real subcommand that drains
deferred metadata — different phases, different stores).

**Exit Code**:
The tool's process exit status, distinct values so scripts/cron never
mistake an unsound result for a clean one: `0` = clean restore, nothing deferred,
nothing flagged; `3` = restore completed but ≥1 **Deferred Object** remains
(operator fixes DEFINER users and `replay`s — a state with an *automated* next
step); `4` = restore completed but **Verify** surfaced ≥1 integrity finding
(orphan rows and/or structural `Corrupt`) — a state whose next step is a *human*
inspecting data, with **no** automated path, which is exactly why it is not
folded into `3` (a cron that answers `3` by running `replay` would do the wrong
thing here); non-zero-and-not-3-not-4 (`1`) = **Fatal Error**, fail-fast, restore
stopped mid-run and resumable (re-run `restore <file>`). `3` and `4` are both "succeeded with
reservations", kept distinct because their remediations differ (mechanical replay
vs. manual data repair). When both a **Deferred Object** and a **Verify** finding
are present, `4` takes precedence — unsound *data* outranks deferred *metadata* —
and the printed report names both.
A deliberate **Interrupt** exits with the shell `128 + signo` convention —
`130` (SIGINT/Ctrl-C) or `143` (SIGTERM, Unix) — kept **outside** the
`0`/`3`/`4`/`1` range so an operator stop is never confused with a failure or a
clean finish. `130`/`143` mean "deliberately stopped, resumable" (re-run
`restore <file>`), never "failed": a cron that retries on `1` must not treat an
operator's Ctrl-C the same way. Shell/waitstatus convention (e.g. bash), a
deliberate choice, not a Go stdlib guarantee.
_Avoid_: return code, status (be specific: 0 clean / 3 deferred / 4 verify-flagged
/ 1 fatal / 130·143 interrupted).

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

**Progress**:
Live visibility updates emitted to stderr after every batch completes, format:
`[12:34:56] 64.2MB / 10.0GB (0.6%) | 1,024 statements | 2s elapsed | ~5m32s
remaining`. One-line human-readable showing bytes processed vs total file size
(percentage), cumulative statement count, wall-clock elapsed time since restore
start, and estimated time remaining based on current throughput (bytes/second).
Update printed immediately after each batch COMMIT; for 10GB dump with 160
batches, 160 progress lines (more verbose than periodic updates, but more
informative — user sees per-batch throughput and detects hang faster). ETA
skipped if elapsed < 5 seconds (insufficient data for accurate estimate). Final
summary printed on completion (total bytes, statements, time). **TTY-adaptive
rendering**: detects `term.IsTerminal(os.Stderr.Fd())` — if interactive TTY,
uses carriage return (`\r`) to overwrite the line; if non-TTY (redirect to file,
CI), uses newline (`\n`) to append. Stderr never interferes with stdout
(reserved for future machine-readable output). Implementation: inline in batch
loop after COMMIT, no background goroutine or ticker — state (bytes read,
statements count, start time) already available, just format and print. See
ADR-0027.
_Avoid_: status, log, periodic (be specific: live per-batch updates, stderr,
TTY-adaptive).

**Timeout**:
Network and I/O time limits set explicitly in the MariaDB connection DSN to
surface connection and execution failures quickly rather than hanging
indefinitely. Three timeouts are configured: `timeout=30s` (connection
establishment — matching MariaDB Connector/J default), `readTimeout=5m` (I/O
read operations — allows slow server responses during heavy load),
`writeTimeout=10m` (I/O write operations — allows large batch COMMIT to
complete, ~64MB could take minutes). These align with fail-fast error handling
(ADR-0006): MySQL default `--connect-timeout` is 43200 seconds (12 hours), far
too permissive for operational tool — explicit 30s timeout surfaces
unreachable server or network partition within half a minute, not hours. If
timeout reached, Go driver returns error (`context deadline exceeded`, `i/o
timeout`), tool exits with **Exit Code 1** (Fatal Error), user diagnoses
root cause (network issue? server overload? batch too large?) and resumes via
checkpoint or fixes underlying problem. User can override via optional flags
`--timeout`, `--read-timeout`, `--write-timeout` if workload needs different
values (e.g., `--write-timeout=30m` for very large batches). Trade-off: if
batch COMMIT genuinely needs >10 minutes, tool times out and exits — this is
signal that batch size too large or server under-resourced, not "let it hang
indefinitely". See ADR-0028.
_Avoid_: deadline, TTL (be specific: connection timeout, read/write timeout,
DSN parameters).

**Data Directory**:
Where the tool keeps its local state (the **Checkpoint Store** and any saved
**Connection Profiles**). Default: the directory of the running executable
(`os.Executable()` → `filepath.EvalSymlinks` → `filepath.Dir`), so the whole tool
is *portable* — copy the folder and its state travels with it. If that directory
is not writable (e.g. installed under `/usr/local/bin`), it falls back to
`os.UserConfigDir()` with a warning; `--data-dir` overrides both.
_Avoid_: install dir, home dir (be specific: executable-adjacent by default).

**Connection Profile**:
A named, reusable set of connection settings — `name`, `host`, `port`, `user`,
`database`, plus non-secret options — stored as one row in the local SQLite file
in the **Data Directory**. Lets an operator run `restore --profile prod` instead
of re-typing flags. A profile NEVER stores a plaintext password; if the operator
saves one it is sealed by the **Credential Vault** and kept *on this same row* as
a ciphertext column (or, if none is saved, the password is supplied at run time).
A profile is created **password-less**: `save` writes only the non-secret
settings and never touches the **Master Passphrase**. `save` is an **upsert** —
re-running it on an existing `name` updates the fields given, keeps the rest, and
leaves `sealed_password` untouched (there is no separate `edit` verb); it prints
`created` vs `updated` so a mistyped name is visible. This is what makes
editing `host`/`port` leave the vaulted password valid (below): the update never
touches the seal, and the AAD still matches on `name`. Sealing a password is
always the separate `set-password` step — the one command that opens the vault —
so `save` stays non-interactive and script-safe, and a profile that never gets a
`set-password` is a first-class state (its password comes from a run-time
**Credential Source**). Deletion is the mirror: `delete` is a metadata
operation that drops the row (and, with it, any `sealed_password`) and NEVER
requires the Master Passphrase — you need not open a secret to discard it, so a
profile whose passphrase is lost is still removable rather than an
undeletable zombie. Its only gate is an interactive `y/N` confirmation (`--yes`
to skip for unattended use). Listing is likewise passphrase-free and touches no
secret: `list` shows the non-secret settings plus a presence-only password
column derived from `sealed_password IS NOT NULL` (`vaulted` / `—`) — never a
length, prefix, or any value from the plaintext, and it does **not** verify the
seal (an AAD/`vault-id` mismatch is only detectable by attempting `Open`, which
needs the passphrase, so mismatch detection is deferred to restore time,
fail-closed). The `name` is the profile's *unique identity*: it keys
the profile record and is
the `profile-id` bound into that row's sealed-password AAD (Credential Vault) —
there is no separate internal ID. A consequence: because only `name` is
identity, editing a profile's `host`/`port` does **not** invalidate its vaulted
password (the AAD still matches on `name`), so redirecting `prod` to a new address
reuses the existing credential rather than forcing a conscious re-entry — the
accepted trade-off for treating the name, not the destination, as identity.
Renaming a profile changes its AAD and therefore re-seals its vault entry.
_Avoid_: connection string, DSN (a profile is non-secret settings only).

**Credential Vault**:
The at-rest storage for every **Connection Profile**'s DB password, encrypted with
AES-256-GCM. The vault is **not** a separate file: it is realized *inline* in the
same **Data Directory** SQLite store, each profile's sealed password kept as a column
on its own **Connection Profile** row (the model proven in the sibling
`mariadb-magic` tool, where `password_ciphertext` lives on the `connections` row). A
profile and its secret are therefore one record — there is no second file to drift
out of sync, and deleting a profile deletes its ciphertext with it. A single
vault-level **Key Encryption Key (KEK)** is derived once from the **Master
Passphrase** via Argon2id; the salt (`crypto/rand`) and KDF parameters live in the
vault's single **settings row**, recorded as one PHC string
(`$argon2id$v=19$m=65536,t=3,p=4$<salt>$…`) — not per profile. Each profile's password
is then sealed directly under that KEK with a fresh random 96-bit nonce per seal
(no per-profile KDF, no intermediate data-encryption-key layer — a KEK sealing a few
dozen passwords stays far under GCM's 2³²-message-per-key limit, so envelope wrapping
buys nothing here but complexity). That column therefore holds only the sealed
`nonce ‖ ciphertext ‖ tag`; salt and parameters are read from the settings row. Each
seal also binds **Additional Authenticated Data** — the profile's identity
concatenated with the vault's own random ID (`profile-id ‖ vault-id`, `vault-id` a
UUID in the settings row) — which GCM authenticates but does not encrypt. The AAD is
not stored on the profile row; it is reconstructed at open time from the row's own
identity and the settings row. This makes a *ciphertext-swap* fail loudly instead of
silently: a sealed value moved to a different profile's row (both under the same KEK)
breaks on the profile mismatch, and one transplanted into a *different* vault breaks
on the `vault-id` mismatch — `Open` returns an authentication error rather than the
wrong host's password. Copying the whole **Data Directory** is unaffected because the
`vault-id` travels in the same SQLite store. The KDF parameters are fixed at the
RFC 9106 §4 second option — `time=3, memory=64 MiB, threads=4`, deriving a 32-byte
(AES-256) key from a 16-byte salt — chosen over the first option's 2 GiB memory cost
because a 2 GiB transient allocation would betray this tool's constant-memory promise
when restoring a 9 GB+ dump. The parameters are **not** a user-facing knob, but
because they live in the settings row the hard-coded default may be raised in a future
version without stranding vaults written under the old cost. Because the KEK is never on disk, copying the
**Data Directory** yields only ciphertext — real encryption, not obfuscation. GCM
also detects tampering. Contrast the rejected `.mylogin.cnf`, which *obfuscates* (a
false sense of security). The vault is only one link in the **Credential Source**
precedence: an explicit `--password-file` or `--password` supplied in the invocation
outranks it, and it in turn outranks the ambient `MYSQL_PWD` env var and the
interactive prompt.
_Avoid_: keystore, keyring (the vault is our own AES file, not an OS keyring).

**Master Passphrase**:
The human-held secret that unlocks the **Credential Vault**. Entered at a no-echo
TTY prompt (`term.ReadPassword`) or, for unattended runs, read from an env var —
NEVER written into the **Data Directory** (doing so would collapse the vault back
into obfuscation). It is the one piece of state that deliberately does not travel
with the portable folder; it lives in the operator's head or an operator-managed
`0600` file.
_Avoid_: master password, key (it derives the key; it is not the AES key itself).

**Credential Source**:
Where the DB password for a run comes from, resolved by one fixed precedence chain
applied whether or not the profile has a vaulted password: explicit `--password-file`,
then explicit `--password`/`-p`, then the vaulted `sealed_password`, then the ambient
`MYSQL_PWD` env var, then a no-echo TTY prompt (`term.ReadPassword`, TTY-gated by
`term.IsTerminal`). The first source that yields a password wins; an explicit
invocation source deliberately overrides the vault, and the vault overrides env.
`--password`/`-p` is honored but warned about (MariaDB's own docs note it is visible
in `ps`/shell history). A headless run with no source fails closed rather than hangs.
_Avoid_: fallback chain (it is a full precedence order, not only a no-vault fallback).

## Relationships

- A **Dump** is a stream of **Statements** separated by **Statement Boundaries**
- The **Statement Splitter** turns the **Dump** byte stream into **Statements**
- A **Checkpoint** records exactly one **Statement Boundary** (a byte offset)
- A **Batch** groups **Statements**; one **COMMIT** ends one **Batch**
- An **Implicit-Commit Statement** forces a **Batch** boundary + **Checkpoint**
  right before it, so the server's implicit COMMIT never outruns durability
- A **Checkpoint** is written only after its **Batch**'s **COMMIT** succeeds
- The **Resume Batch** is the one **Batch** allowed to tolerate duplicate errors
- The **Data Directory** holds both the **Checkpoint Store** and the
  **Connection Profiles**, and defaults to the executable's own directory
- A **Connection Profile** holds non-secret settings on its row; if a password is
  saved, the **Credential Vault** seals it into a ciphertext column on that same row
- The **Credential Vault** is unlocked by the **Master Passphrase**, which never
  travels with the **Data Directory**
- A **Deferred Object** is a narrow, captured exception to a **Fatal Error**;
  it lands in the **Deferred Store** and is drained by **Replay**
- A run with any **Deferred Object** ends with **Exit Code** `3`, and a run whose
  **Verify** surfaced a finding ends with `4` (which wins over `3` when both hold);
  both are distinct from `0` (clean) and `1` (**Fatal Error**)
- The whole restore — **Pre-flight Check**, every **Batch**, the **Resume Batch**,
  and **Verify** — runs on one pinned connection; the tool owns **Fast Mode**'s
  three session variables on it and resets all three to `1` on every exit path

## Example dialogue

> **Dev:** "Can we checkpoint after every line we read?"
> **Domain expert:** "No — a **Checkpoint** must land on a **Statement
> Boundary**. A single extended `INSERT` can span hundreds of MB across many
> lines; the byte offset only means something after a `;` in the `default`
> state."

## Flagged ambiguities

- "line" was used to mean both a physical newline-delimited line and a logical
  **Statement** — resolved: these are distinct. We checkpoint on **Statements**,
  never on lines. A **Dump** line can be hundreds of MB (extended INSERT).
