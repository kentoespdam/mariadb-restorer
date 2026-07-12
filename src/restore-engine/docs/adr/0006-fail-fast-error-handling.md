# Any non-tolerated Batch error stops the restore; no auto-retry in MVP

The restore runs with `foreign_key_checks=0`, `unique_checks=0`, `autocommit=0`
(ADR-0004). Under those settings the only integrity contract we can offer is
**"stop the moment anything unexpected happens."** Continuing past an unknown
error — skipping a Statement, trying the next one — would bury silent corruption,
exactly what ADR-0001's checkpoint ordering exists to prevent.

So: when a **Batch** returns an error outside the tolerated class, we **roll back
the open transaction** (it has not COMMITted, so the **Checkpoint** never advances)
and **exit non-zero**. We do not skip, we do not best-effort. The last saved
**Checkpoint** stays valid; the operator fixes the root cause and `resume`s from
the last safe **Statement Boundary**.

## Error classification (verified via context7, MariaDB docs)

Server error numbers (from `*mysql.MySQLError.Number` in go-sql-driver):

- **Tolerated — only inside the Resume Batch** (ADR-0001's crash window):
  - `1062` `ER_DUP_ENTRY` (SQLSTATE 23000) — "Duplicate entry '%s' for key '%s'".
  - `1050` `ER_TABLE_EXISTS_ERROR` — "Table '%s' already exists".
- **Fatal — always fail-fast**, including inside the Resume Batch:
  - `1146` `ER_NO_SUCH_TABLE` — ordering broken / dump truncated.
  - `1153` `ER_NET_PACKET_TOO_LARGE` — a Statement exceeded the provisioned
    `max_allowed_packet` despite the Pre-flight Check (ADR-0003).
  - `1064` syntax error — a signal our own **Statement Splitter** mis-cut a
    boundary; must surface loudly, never be swallowed.
  - every other server error not in the tolerated list.

Connection-drop is a **client-side** condition, not a server error number:
`2006` `CR_SERVER_GONE_ERROR` / `2013` `CR_SERVER_LOST` surface in Go as
`driver.ErrBadConn`, on a different code path than `*mysql.MySQLError.Number`.
In the MVP it is treated as fatal — no automatic reconnect-and-retry.

## On failure, the tool prints

The raw server error, the failing Statement's **Byte Offset**, its statement
number, and a truncated snippet (≈first 1KB) of the Statement — enough for the
operator to locate the problem and resume after fixing it.

## Considered Options

- **Auto-retry transient (connection-drop) Batches with backoff (option 7b):**
  rejected for MVP — the resume path *is* our retry mechanism, only manual and
  safer (the Resume Batch already tolerates the replay duplicates). Adding a
  retry loop before that path is proven risks masking a real problem (a server
  that OOMs repeatedly) for no MVP benefit.
- **Skip the failing Statement and continue:** rejected — turns an unknown error
  into silent partial data, the exact failure ADR-0001 is built to avoid.
- **Treat 1146 / 1064 as tolerable:** rejected — they signal ordering damage or a
  splitter bug, not the benign COMMIT-then-crash replay window.

## Consequences

- The tolerated-error set is scoped **both** by code (1062/1050) **and** by phase
  (Resume Batch only). A duplicate-entry during a *normal* Batch is fatal — it
  means something is genuinely wrong, not a replay.
- Error classification lives in two places by necessity: server-number matching
  (`*mysql.MySQLError`) and connection-error matching (`driver.ErrBadConn`).
- No retry/backoff config surface in the MVP; if operational experience shows
  transient drops are common, 7b can be revisited without changing the resume
  contract.
