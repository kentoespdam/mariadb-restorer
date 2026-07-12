# Error classification: Exit Code 1 for all Fatal Errors

**Konteks:** ADR-0028 (timeout configuration) introduce inconsistency dengan existing exit code contract (ADR-0013, CONTEXT.md Exit Code term). ADR-0028 lines 30, 58 state "tool exit dengan **Exit Code 3** (Database Error)" untuk timeout errors, tapi line 62 state "timeout error adalah **Fatal Error**". Ini contradictory karena per ADR-0013 dan CONTEXT.md:
- **Exit Code 1** = Fatal Error, fail-fast, restore stopped mid-run, resumable
- **Exit Code 3** = Deferred Object remains, restore completed, automated next step (replay)

Timeout adalah Fatal Error (restore stopped mid-run), bukan "succeeded with reservations" (restore completed). Exit Code 3 adalah load-bearing contract (ADR-0013) — scripts/cron branch on `[ $? -eq 3 ] && replay`. Jika timeout return Exit Code 3, automation akan mishandle dengan run replay (wrong action).

**Masalah:** Apakah tool harus introduce new exit code class untuk "Database Error" (timeout, connection failure, query error), atau map semua Fatal Errors (timeout, connection, parse, disk, dll.) ke Exit Code 1?

**Opsi yang dievaluasi:**

1. **Exit Code 1 untuk semua Fatal Errors** — SELECTED: Timeout, connection failure, parse error, disk I/O error, file not found, permission denied, constraint violation, deadlock, dll. adalah Fatal Errors yang stop restore mid-run. Semua return **Exit Code 1**. Exit Code 3 reserved exclusively untuk "Deferred Object remains" (ADR-0013 contract). Simple classification: completed with reservations (3, 4) vs stopped mid-run (1) vs clean (0) vs interrupted (130, 143). Checkpoint-resume mechanism make ALL Fatal Errors recoverable — re-run `restore <file>` jump ke checkpoint dan continue.

2. **Exit Code 3 untuk "Database Error" class (timeout, connection, query), Exit Code 1 untuk "Client Error" (parse, disk, file not found)** — rejected: Break ADR-0013 contract yang already lock Exit Code 3 = "Deferred Object remains — automated `replay` path". Scripts/cron yang branch on `[ $? -eq 3 ] && replay` akan mishandle database error dengan run `replay` command (wrong automation — `replay` drains deferred metadata, tidak fix connection timeout). Exit Code 3 sudah load-bearing sebagai "automated next step exists" signal, bukan generic error category.

3. **Exit Code 3 untuk recoverable errors (timeout, transient connection), Exit Code 1 untuk unrecoverable (parse error, file not found)** — rejected: "Recoverable vs unrecoverable" adalah subjective distinction yang tidak robust. Timeout bisa transient (network blip, server momentarily overloaded) atau permanent (firewall block, server down) — tool tidak bisa distinguish at error time. Checkpoint-resume mechanism already make ALL Fatal Errors recoverable (re-run `restore <file>` resume dari checkpoint, regardless of error type). Distinction tidak add operational value. Break ADR-0013 contract untuk Exit Code 3.

## Keputusan

**Exit Code 1 untuk semua Fatal Errors**. Classification by **execution state**, bukan error type:

- **Exit Code 0**: Clean restore — completed, no deferred objects, no verify findings
- **Exit Code 1**: **Fatal Error** — restore stopped mid-run, resumable — ANY error yang stop execution: timeout, connection error, parse error, disk I/O error, file not found, permission denied, SQL constraint violation, deadlock, server crash, out of memory, etc.
- **Exit Code 3**: **Deferred Object** — restore completed but ≥1 metadata object couldn't be applied (DEFINER mismatch, missing user) — automated next step: `replay`
- **Exit Code 4**: **Verify finding** — restore completed but integrity check flagged orphan rows or structural Corrupt — manual inspection, no automated path
- **Exit Code 130/143**: **Interrupt** — SIGINT (Ctrl-C) or SIGTERM, deliberately stopped by operator, resumable

**Fix ADR-0028:** Change "Exit Code 3 (Database Error)" → "Exit Code 1 (Fatal Error)" di lines 30, 58. Line 62 already correct ("timeout error adalah **Fatal Error**"), tinggal align exit code number.

**Preserve ADR-0013 contract:** Exit Code 3 tetap exclusively untuk "Deferred Object remains — automated `replay` path". Scripts/cron yang branch on `[ $? -eq 3 ] && replay` remain correct. Exit Code 1 adalah generic Fatal Error catch-all untuk any mid-run failure, resume path adalah re-run `restore <file>` (not `replay`).

**Rationale:** Simple classification by execution state (completed vs stopped) lebih robust dari classification by error type (database vs client, recoverable vs unrecoverable). Operator automation (cron, runbooks) branch on **what to do next**, bukan **why it failed**:
- Exit Code 0 → proceed (success)
- Exit Code 1 → diagnose error, fix root cause, re-run `restore <file>` (resume)
- Exit Code 3 → fix DEFINER users, run `replay` (automated metadata drain)
- Exit Code 4 → inspect data manually, repair orphans (manual investigation)
- Exit Code 130/143 → operator stopped it, re-run when ready (deliberate pause)

## Alasan

ADR-0013 established Exit Code 3 contract: "restore *completed* but the result is not fully sound" (Deferred Object remains). Key property: **automated next step exists** (`replay` command). Contract adalah load-bearing — scripts/cron encode this:

```bash
restore dump.sql
case $? in
  0) echo "Clean restore, proceed" ;;
  3) replay && echo "Metadata replayed" ;;  # automated fix
  4) echo "Verify found issues, inspect manually" ;;  # human required
  1) echo "Fatal error, diagnose and resume" ;;
  *) echo "Interrupted or unknown" ;;
esac
```

Jika timeout return Exit Code 3, automation akan execute `replay` command — wrong action. `replay` drains deferred metadata dari checkpoint store, tidak fix network timeout atau server overload. Exit Code 3 signal "automated fix available", timeout tidak punya automated fix (operator harus diagnose: network down? server overload? increase timeout? decrease batch size?).

Classification by error type (database vs client, recoverable vs unrecoverable) tidak align dengan operator action. Operator tidak care "is this a database error or client error" — operator care "what do I do next?". Execution state classification (completed with reservations vs stopped mid-run) directly map ke operator action (replay vs resume).

Checkpoint-resume mechanism make ALL Fatal Errors recoverable — re-run `restore <file>`, tool read checkpoint, jump ke byte offset terakhir, continue dari situ. Distinction "recoverable vs unrecoverable" tidak relevant — semua Fatal Error resume path sama (re-run `restore <file>`), regardless of error type.

KISS principle: one exit code untuk all Fatal Errors (Exit Code 1) simpler dari multiple exit codes untuk error sub-categories. Tool tidak perlu classify error type (timeout vs parse vs disk) at exit time, tinggal detect "did restore complete or stop mid-run?".

## Konsekuensi

- **All Fatal Errors** (timeout, connection, parse, disk I/O, file not found, permission, constraint violation, deadlock, etc.) return **Exit Code 1**.
- Exit Code 3 exclusively untuk "Deferred Object remains — automated `replay` path" (ADR-0013 contract preserved).
- ADR-0028 harus di-fix: change "Exit Code 3 (Database Error)" → "Exit Code 1 (Fatal Error)" di lines 30, 58 untuk align dengan classification ini.
- CONTEXT.md Exit Code term (lines 368-391) already correct dan comprehensive — no update needed, term already state "non-zero-and-not-3-not-4 (`1`) = **Fatal Error**, fail-fast, restore stopped mid-run and resumable".
- Error handling implementation: catch any error during restore execution, print error message with context (what operation failed, error details), exit with Exit Code 1. No need classify error type — generic Fatal Error handling sufficient.
- Operator runbooks dan automation (cron, CI) branch on exit code per contract: `1` = diagnose and resume, `3` = run replay, `4` = inspect manually, `130`/`143` = operator stopped.
- Testing harus cover: timeout exits with 1, connection error exits with 1, parse error exits with 1, disk error exits with 1, verify checkpoint row intact after Exit Code 1 (resume works).
