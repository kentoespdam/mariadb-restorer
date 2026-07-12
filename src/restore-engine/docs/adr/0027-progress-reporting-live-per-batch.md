# Progress reporting: live updates per batch

**Konteks:** Tool restore berjalan lama (10GB dump = 8-15 menit, 50GB dump = 40-75 menit), tanpa feedback user tidak tahu apakah tool hang, seberapa banyak progress, atau berapa lama lagi. ADR-0020 (one-shot mode) memilih fail-fast over retry, jadi user tidak punya recovery loop otomatis — jika tool hang, user perlu detect dan SIGINT manual. Progress visibility adalah critical untuk operability. Q12 design interview mengevaluasi tiga opsi: (1) silent output, (2) periodic updates setiap 30 detik, (3) live updates setiap batch. **User feedback: "sepertinya lebih informatif Opsi 3"** — user value increased information over reduced noise.

**Masalah:** Tanpa progress updates, user tidak tahu restore masih berjalan atau stuck; tidak bisa estimate completion time; tidak bisa judge apakah performance reasonable atau ada bottleneck.

**Opsi yang dievaluasi:**

1. **Silent output (hanya final summary)** — rejected: tidak ada feedback selama restore berjalan, user blind untuk 10+ menit, tidak bisa detect hang vs slow progress, opsi ini eliminated early.

2. **Periodic updates setiap 30 detik** — initially locked, later revised: background goroutine dengan `time.Ticker` 30s mencetak progress ke stderr; update line di-overwrite dengan carriage return (`\r`) di TTY interactive, append dengan newline (`\n`) di non-TTY (redirect to file, CI); format one-line human-readable showing bytes processed, statement count, elapsed time, ETA; update pertama 30 detik setelah start, lalu setiap 30 detik; final summary saat completion. Overhead minimal (one goroutine, mutex-protected snapshot read setiap 30s). Noise rendah (untuk dump 10GB dengan 160 batch, hanya ~20 update lines dalam 10 menit). Inspired by MariaDB's `MYSQL_PROGRESS_CALLBACK` pattern.

3. **Live updates setiap batch** — SELECTED: print progress ke stderr setelah setiap batch COMMIT; tidak perlu background goroutine atau ticker, langsung print setelah batch transaction complete; format one-line sama seperti Option 2 (bytes processed, statements, elapsed, ETA), TTY-adaptive rendering sama (`\r` for TTY overwrite, `\n` for non-TTY append); lebih verbose dari Option 2 (untuk dump 10GB dengan 160 batch ada 160 update lines), tapi **lebih informatif** karena user lihat progress segera setelah setiap batch complete, tidak perlu tunggu 30 detik; user bisa detect per-batch throughput variance (misalnya batch pertama 2s, batch kedua 10s — mungkin ada indexing overhead). Implementation lebih sederhana (no goroutine coordination, no ticker cleanup, progress logic langsung di batch loop). User feedback: **"sepertinya lebih informatif Opsi 3"**.

## Keputusan

**Live progress updates setiap batch**. Setelah setiap batch COMMIT berhasil, tool mencetak one-line progress ke stderr dengan format:

```
[12:34:56] 64.2MB / 10.0GB (0.6%) | 1,024 statements | 2s elapsed | ~5m32s remaining
```

Format:
- `[HH:MM:SS]` — current wall-clock time (memudahkan timestamp untuk log)
- `64.2MB / 10.0GB (0.6%)` — bytes processed vs total file size, percentage
- `1,024 statements` — cumulative statement count sejak start
- `2s elapsed` — total wall-clock time sejak restore start
- `~5m32s remaining` — estimated time to completion based on throughput (bytes/second); skip ETA jika elapsed < 5 detik (data insufficient untuk estimate akurat)

**TTY-adaptive rendering**: detects `term.IsTerminal(os.Stderr.Fd())` — jika interactive TTY, gunakan carriage return (`\r`) untuk overwrite line (visual fixed-line update); jika non-TTY (redirect to file, CI), gunakan newline (`\n`) untuk append (setiap update adalah line baru di log file). Stderr never interferes dengan stdout (reserved untuk future machine-readable output).

**Implementation**: setelah batch loop meng-COMMIT transaction, hitung current progress (bytes read dari file, statements count dari lexer, elapsed time dari start timestamp), calculate percentage dan ETA (bytes_processed / elapsed_seconds untuk throughput, remaining_bytes / throughput untuk ETA), format string, print ke stderr dengan `\r` atau `\n` depending TTY detection. Final summary printed on completion showing total bytes, statements, elapsed time.

**Overhead**: setiap batch completion melakukan progress calculation dan stderr write — negligible dibanding batch execution cost (64MB SQL parse + network send + MariaDB transaction). Untuk dump 10GB dengan 160 batch, ada 160 progress updates (~160 lines di non-TTY, 1 overwritten line di TTY).

**Trade-off accepted**: lebih verbose dari periodic updates (160 lines vs ~20 lines untuk 10GB dump), tapi user value increased information — bisa lihat per-batch throughput, detect hang lebih cepat (jika no update untuk >10s, clearly stuck; dengan 30s periodic, hang vs slow batch ambiguous sampai next update).

## Alasan

Context7 research untuk MariaDB C API menunjukkan `MYSQL_PROGRESS_CALLBACK` pattern: callback function dipanggil secara periodic oleh client library during long-running operations, passing stage info dan progress percentage. ADR ini mengadopsi spirit dari pattern tersebut — periodic visibility updates, percentage-based progress, human-readable format — tapi simplified untuk single-stage linear restore (no multi-stage complexity).

User feedback **"sepertinya lebih informatif Opsi 3"** adalah deciding factor: user explicitly value increased information over reduced noise. Option 2 (periodic 30s) adalah reasonable default untuk many tools, tapi untuk restore use case dimana user actively monitor progress (terutama di production incident recovery), live per-batch updates memberikan confidence bahwa setiap batch completing dan tool making forward progress. Per-batch updates juga memudahkan detect per-batch performance variance — jika sebagian batch fast (2s) dan sebagian slow (10s), user bisa correlate dengan dump content (misalnya large INSERTs vs many small statements) atau server load.

Implementation simplicity adalah bonus: tidak perlu goroutine coordination, tidak perlu ticker lifecycle management, tidak perlu mutex-protected snapshot reads — progress logic langsung inline di batch loop, state already available (bytes read, statements count, start time), tinggal format dan print. Ini align dengan KISS principle.

Noise concern (160 lines di non-TTY untuk 10GB dump) adalah acceptable karena: (1) di TTY interactive, hanya 1 line di-overwrite jadi visual noise minimal; (2) di non-TTY (log file, CI), 160 lines dalam 10 menit adalah ~0.27 lines/second, reasonable untuk observability log; (3) user bisa redirect stderr ke `/dev/null` jika benar-benar tidak mau progress updates, tapi default adalah "show me everything".

## Konsekuensi

- Progress updates dicetak ke stderr setelah setiap batch COMMIT, format one-line human-readable dengan timestamp, bytes, statements, elapsed, ETA.
- TTY detection via `term.IsTerminal(os.Stderr.Fd())`: `\r` overwrite untuk interactive, `\n` append untuk non-interactive.
- ETA calculation skipped jika elapsed < 5 seconds (insufficient data).
- Final summary printed on completion (total bytes, statements, elapsed time).
- Implementation di batch loop, tidak perlu background goroutine atau ticker.
- CONTEXT.md **Progress** term harus didefinisikan untuk reflect live per-batch updates (bukan periodic 30s).
- Testing harus cover: TTY vs non-TTY rendering, ETA calculation accuracy, edge case (very fast restore < 5s, very large dump > 1 hour).
- User expectation: restore output akan lebih verbose dari typical CLI tool, tapi increased visibility adalah intentional trade-off untuk operability.
