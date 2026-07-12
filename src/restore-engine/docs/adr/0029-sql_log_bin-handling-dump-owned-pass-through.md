# sql_log_bin handling: dump-owned pass-through

**Konteks:** Tool restore operations bisa affect binary log — jika `sql_log_bin=1` (server default), semua `INSERT`/`CREATE`/`ALTER` statements masuk binlog dan replicate ke downstream servers. Context7 research menunjukkan `sql_log_bin` adalah session variable yang requires `BINLOG ADMIN` privilege; `SET sql_log_bin=0` disable binary logging, `SET sql_log_bin=1` enable. CONTEXT.md line 184 menyebut `SQL_LOG_BIN` sebagai "Dump-owned session settings... passed through verbatim" alongside `TIME_ZONE`/`SQL_MODE`/`charset`, tapi tidak ada penjelasan apa itu sql_log_bin, kenapa distinction ini penting, atau implikasi operasional untuk replication scenarios. ADR-0025 established tool forwards `SET sql_mode` verbatim (dump-owned), berbeda dari `autocommit`/`unique_checks`/`foreign_key_checks` yang tool actively manage (tool-owned).

**Masalah:** Apakah tool harus actively manage `sql_log_bin` (set 0 saat restore, reset 1 saat exit — treat as tool-owned seperti Fast Mode variables), atau pass through verbatim (honor apapun `SET sql_log_bin` statement yang ada di dump — treat as dump-owned)?

**Opsi yang dievaluasi:**

1. **Pass-through only (dump-owned)** — SELECTED: Tool tidak pernah touch `sql_log_bin`, forward apapun `SET sql_log_bin` statement yang ada di dump ke server verbatim. Jika dump tidak punya statement ini, server default (biasanya `sql_log_bin=1`) apply. Tool tidak inject atau modify `sql_log_bin`.

2. **Tool-managed (tool-owned)** — rejected: Tool actively set `SET sql_log_bin=0` saat establish connection, reset ke `1` saat exit. Guarantee restore operations tidak masuk binlog. Tapi requires `BINLOG ADMIN` privilege (fails di environment tanpa privilege), breaks operator control (jika operator explicitly want binlog ON untuk propagate restore downstream), introduces coupling ke server state (violates ADR-0003).

3. **Hybrid (check/warn)** — rejected: Tool check `@@sql_log_bin` di pre-flight, print warning jika `sql_log_bin=1`, tapi tidak modify. Visibility tapi no control. Warning noise (banyak legitimate use case untuk binlog ON), requires privilege untuk query, introduces coupling ke server state, operator action required (adds friction).

## Keputusan

**sql_log_bin adalah dump-owned session setting, passed through verbatim**. Tool tidak query `@@sql_log_bin`, tidak set `sql_log_bin`, tidak warn. Jika dump contain `SET sql_log_bin=0` statement (misalnya di header), tool forward ke server — operations tidak masuk binlog. Jika dump tidak contain statement ini, server default apply (biasanya `sql_log_bin=1`) — operations masuk binlog dan replicate downstream jika replication configured.

**Classification:** `sql_log_bin` joins `TIME_ZONE`, `SQL_MODE`, `charset/NAMES` sebagai dump-owned session settings (CONTEXT.md line 184 sudah list ini, ADR ini formalize reasoning). Berbeda dari tool-owned variables (`autocommit`, `unique_checks`, `foreign_key_checks`) yang tool actively manage untuk optimize restore execution.

**No privilege requirement:** Tool tidak butuh `BINLOG ADMIN` privilege karena tidak modify `sql_log_bin`. Restore user cukup punya `INSERT`/`CREATE`/`ALTER` privileges (principle of least privilege).

**Operator control:** Replication implications adalah operational decision. Jika operator want restore operations tidak masuk binlog (misalnya restore ke replica untuk local testing), mereka bisa either (1) modify dump header untuk include `SET sql_log_bin=0`, atau (2) manually set di session sebelum restore, atau (3) set server default `sql_log_bin=0` di my.cnf. Jika operator want restore operations masuk binlog (misalnya restore ke primary server untuk propagate ke downstream replica), biarkan server default atau dump declaration handle.

**No flag:** Tool tidak expose `--sql-log-bin` flag. Dump's in-stream declaration adalah sole authority, consistent dengan ADR-0003 principle (no coupling to live server state).

## Alasan

Context7 research untuk MariaDB (`/mariadb-corporation/mariadb-docs`) menunjukkan:
- `sql_log_bin` requires `BINLOG ADMIN` privilege untuk modify.
- `SET sql_log_bin=0` disable binary logging for current session; operations tidak masuk binlog dan tidak replicate downstream.
- `SET sql_log_bin=1` enable binary logging; operations masuk binlog (jika server has binlog enabled) dan replicate downstream jika replication configured.
- Common use case: `SET sql_log_bin=0` during `ALTER TABLE` operations di replica to prevent propagating change upstream.

Design alignment:
1. **Privilege consideration:** Restore user di production environment mungkin dibatasi ke `INSERT`/`CREATE`/`ALTER` privileges only, tanpa `BINLOG ADMIN` (principle of least privilege). Jika tool actively manage `sql_log_bin`, restore akan fail dengan permission error — adding operational friction yang tidak necessary. Pass-through (Option 1) require zero privileges untuk `sql_log_bin`.

2. **Operator control:** Replication topology dan binlog decision adalah operational concern, bukan tool concern. Some operators want restore operations masuk binlog untuk propagate ke downstream replica (restore ke primary server, downstream sync automatically). Some operators want restore tidak masuk binlog (restore ke replica untuk local testing, tidak propagate upstream). Tool tidak bisa assume which scenario — ini adalah operator's decision yang depend on their infra topology.

3. **ADR-0003 consistency:** ADR-0003 established principle "do not couple tool behavior to live server state" (no `max_allowed_packet` derivation from server). Option 2 dan 3 require query `@@sql_log_bin` atau set `sql_log_bin`, introducing coupling. Option 1 (pass-through) align dengan ini — tool honor dump's declaration, tidak query atau modify server state.

4. **ADR-0025 pattern:** ADR-0025 established `SET sql_mode` adalah dump-owned (forwarded verbatim), berbeda dari `DELIMITER` yang tool-owned (consumed, not forwarded). `sql_log_bin` follow same pattern sebagai `sql_mode` — both are session variables yang dump might declare, both forwarded verbatim, tool observe tapi tidak modify.

5. **Simplicity (KISS):** Pass-through require zero additional code — tool sudah forward `SET` statements verbatim (per ADR-0025). `SET sql_log_bin` is just another `SET` statement, forward dan done. Option 2 require privilege check, state management, exit reset. Option 3 require query, warning logic, false positive handling.

Existing CONTEXT.md classification (line 184: "Dump-owned session settings... TIME_ZONE, SQL_MODE, charset/NAMES, SQL_LOG_BIN") adalah correct — ADR ini hanya formalize reasoning dan document implications.

## Konsekuensi

- Tool tidak query `@@sql_log_bin`, tidak set `sql_log_bin`, tidak warn tentang binlog state.
- Jika dump contain `SET sql_log_bin=...` statement, tool forward ke server verbatim (same treatment sebagai `SET sql_mode`).
- Jika dump tidak contain `SET sql_log_bin` statement, server default apply (biasanya `sql_log_bin=1` — binlog ON).
- **Operator responsibility:** Jika operator want restore operations tidak masuk binlog, mereka harus explicitly ensure `sql_log_bin=0` via dump header modification, manual session set, atau server default config. Tool tidak assume.
- **No privilege requirement:** Tool tidak butuh `BINLOG ADMIN` privilege. Restore user cukup punya data manipulation privileges.
- **Replication implications:** Jika restore berjalan dengan `sql_log_bin=1` dan replication configured, restore operations akan replicate downstream — this is by design, bukan bug. Operator yang control replication topology harus aware dan configure accordingly.
- CONTEXT.md harus expand `SQL_LOG_BIN` explanation di Fast Mode term atau create dedicated term untuk clarify what sql_log_bin does dan why tool-owned vs dump-owned distinction matters.
- Testing harus cover: restore dengan dump yang contain `SET sql_log_bin=0`, restore dengan dump yang tidak contain statement ini (verify server default apply), verify tool tidak query atau set sql_log_bin.
- No new code required — existing `SET` statement forwarding (ADR-0025) already handle `sql_log_bin`.
