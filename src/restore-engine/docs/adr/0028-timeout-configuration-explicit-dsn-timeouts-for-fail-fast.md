# Timeout configuration: explicit DSN timeouts for fail-fast

**Konteks:** Tool restore berjalan lama (10GB dump = 8-15 menit, 50GB dump = 40-75 menit) dengan long transactions (Batch = ~64MB atau ~1000 statements per COMMIT). Connection issues (network partition, server crash, server overload) bisa membuat tool hang tanpa feedback, dan user tidak tahu apakah tool stuck atau masih processing. ADR-0006 menetapkan fail-fast error handling (no auto-retry), ADR-0011 menetapkan single pinned connection untuk entire restore. Tanpa explicit timeout configuration, tool bergantung pada driver/server defaults yang mungkin terlalu permissive (MySQL `--connect-timeout` default = 43200 seconds / 12 jam) atau terlalu aggressive untuk long batch operations.

**Masalah:** Apakah tool harus set explicit connection/read/write timeouts di DSN, atau rely on driver/server defaults? Jika explicit, berapa timeout value yang balance antara fail-fast (detect issues cepat) dan long transaction support (allow batch COMMIT selesai)?

**Opsi yang dievaluasi:**

1. **Driver defaults (no explicit timeout di DSN)** — rejected: MySQL default `--connect-timeout` adalah 43200 seconds (12 jam) per Context7 research, terlalu lama untuk detect connection issues; tool akan hang 12 jam sebelum report error, defeating fail-fast principle.

2. **Explicit DSN timeouts (timeout, readTimeout, writeTimeout)** — SELECTED: Set `timeout=30s` (connection establishment), `readTimeout=5m` (read operations), `writeTimeout=10m` (write operations) di DSN. Ini matching MariaDB Connector/J default (30s connect timeout), allow long batch COMMIT (10 menit cukup untuk 64MB batch), tapi surface connection/network issues cepat (tidak hang jam-jaman). Context7 research menunjukkan `go-mysql-org/go-mysql` driver supports DSN parameters `timeout`, `readTimeout`, `writeTimeout` dengan Go `time.ParseDuration` format. User bisa override via flags jika needed.

3. **Context-based per-statement deadlines** — rejected: overhead untuk set context deadline di setiap `db.ExecContext()` call, require goroutine coordination untuk cancel, complexity tidak seimbang dengan benefit; DSN timeouts sudah built-in di driver level dan simpler (KISS principle).

4. **Infinite timeout (or very high like 24 hours)** — rejected: jika batch genuine stuck (deadlock, server hang, network partition), tool akan wait indefinitely atau 24 jam, user blind; fail-fast principle menuntut error surface cepat untuk user bisa diagnose dan resume.

## Keputusan

**Explicit DSN timeouts**: Tool set `timeout=30s&readTimeout=5m&writeTimeout=10m` di MariaDB connection DSN. Format:

```go
dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=30s&readTimeout=5m&writeTimeout=10m", user, pass, host, port, db)
```

**Timeout values:**
- `timeout=30s` — connection establishment timeout. Matching MariaDB Connector/J default (30000ms). Cukup untuk establish connection ke server yang healthy, tapi fail fast jika server unreachable atau network partition (tidak wait 12 jam).
- `readTimeout=5m` — I/O read timeout. Allow server response lambat saat heavy load (misalnya server processing large transaction dari client lain), tapi detect jika server truly hang atau network stall (5 menit tanpa data = problem).
- `writeTimeout=10m` — I/O write timeout. Allow batch COMMIT selesai untuk large batch (64MB SQL bisa memakan waktu di server dengan heavy write load atau slow disk), tapi fail jika batch genuine stuck (>10 menit untuk satu COMMIT = clearly ada problem, bukan "just slow").

**Error handling:** Jika timeout tercapai, Go driver return error (`context deadline exceeded`, `i/o timeout`), tool exit dengan **Exit Code 1** (Fatal Error). User lihat error message, diagnose root cause (network? server overload? batch terlalu besar?), lalu resume dengan checkpoint atau fix underlying issue. Resume path adalah retry mechanism (ADR-0006), bukan auto-retry di tool.

**User override:** Tool expose flags `--timeout`, `--read-timeout`, `--write-timeout` (optional, default ke values di atas). Jika user tahu batch mereka butuh >10 menit, mereka bisa pass `--write-timeout=30m`. Flag ini construct DSN dengan user-provided values.

**TTY-adaptive:** Tidak ada. Timeout values sama untuk TTY vs non-TTY — timeout adalah network/server concern, bukan UI concern.

## Alasan

Context7 research untuk MySQL dan MariaDB menunjukkan:
- MySQL default `--connect-timeout` adalah 43200 seconds (12 jam) — clearly terlalu permissive untuk operational tool.
- MariaDB Connector/J default `connectTimeout` adalah 30 seconds (30000ms).
- MariaDB Connector Python example menunjukkan `connect_timeout=5.0` dan `socket_timeout=60.0` untuk production use.
- `go-mysql-org/go-mysql` driver supports DSN parameters `timeout`, `readTimeout`, `writeTimeout` dengan Go `time.ParseDuration` format (e.g., `30s`, `5m`, `10m`).

Industry practice adalah explicit timeouts untuk production tools, bukan rely on defaults yang bervariasi antar driver/server version. MariaDB ecosystem tools (Connector/J, Connector/Python, Connector/Node.js) semua expose timeout configuration dan set reasonable defaults (30s-60s untuk connect, minutes untuk long operations).

Fail-fast principle (ADR-0006) menuntut tool detect issues cepat dan exit dengan clear error, bukan hang indefinitely. Explicit timeout 30s untuk connection establishment surface network/server unreachable problems dalam setengah menit, bukan 12 jam. User bisa immediately diagnose (ping server? check firewall? server down?) dan retry.

Long transaction support: `writeTimeout=10m` adalah balance — 64MB batch COMMIT bisa memakan waktu di server dengan heavy load, tapi jika >10 menit clearly ada problem (deadlock? server disk full? batch terlalu besar?). Ini adalah signal untuk user bahwa setup perlu adjustment (increase server resources, decrease batch size, check server health), bukan "biarkan tool hang indefinitely dan hope it completes".

Single pinned connection (ADR-0011) berarti timeout di DSN apply ke satu connection yang di-reuse entire restore — no per-query overhead, no goroutine coordination, simple dan reliable.

KISS principle: DSN timeouts adalah built-in driver feature, tinggal set string, no additional machinery. Context-based deadlines (Option 3) require `context.WithTimeout()` di setiap `db.ExecContext()` call, goroutine coordination untuk handle cancellation, dan state tracking — complexity tidak worth the benefit.

## Konsekuensi

- Connection DSN construction harus include `timeout`, `readTimeout`, `writeTimeout` parameters dengan default values `30s`, `5m`, `10m`.
- Tool expose optional flags `--timeout`, `--read-timeout`, `--write-timeout` untuk user override (document di `--help` dan README).
- Jika timeout tercapai, tool exit dengan **Exit Code 1** (Fatal Error), error message include timeout type dan value (e.g., "write timeout after 10m0s").
- CONTEXT.md harus document timeout behavior di **Connection Profile** atau new term (currently no timeout documentation exists, verified via grep).
- Testing harus cover: connection timeout (server unreachable), read timeout (server hang mid-response), write timeout (batch COMMIT stuck), user override via flags.
- User expectation: jika batch genuine butuh >10 menit, tool akan fail dengan timeout error — user harus diagnose (server under-resourced? batch terlalu besar?) dan adjust timeout atau setup, bukan expect tool wait indefinitely.
- No auto-retry: timeout error adalah **Fatal Error**, tool exit immediately, user resume via checkpoint (resume path adalah retry mechanism).
