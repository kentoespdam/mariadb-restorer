# MariaDB Restorer

A resilient, streaming CLI tool that restores massive MariaDB/MySQL SQL dump
files (9GB+) with constant memory, crash-resume via byte-offset checkpoints,
and speed optimizations that bypass real-time constraint checking.

## Language

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
