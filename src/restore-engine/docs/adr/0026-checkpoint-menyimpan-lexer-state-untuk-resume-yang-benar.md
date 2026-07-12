# Checkpoint menyimpan lexer state untuk resume yang benar

**Konteks:** Saat resume dari checkpoint di byte offset tertentu, lexer harus tahu state yang benar — apakah `\` adalah escape (default) atau literal (`NO_BACKSLASH_ESCAPES`), apakah `"` adalah string (default) atau identifier (`ANSI_QUOTES`), apa delimiter saat ini (default `;`), dan charset apa yang aktif — agar boundary `;` berikutnya ditempatkan dengan benar. ADR-0023 menetapkan lexer adalah stateful (delimiter, escape mode), ADR-0024 menetapkan byte offset sebagai checkpoint boundary, ADR-0025 menetapkan escape rule di-mirror dari `SET sql_mode` in-stream (termasuk `ANSI_QUOTES`). Tapi Checkpoint Store saat ini (CONTEXT.md lines 101-126) hanya menyimpan `byte_offset` dan `statements_done` — tidak menyimpan lexer state yang dibutuhkan untuk parse dari byte tersebut.

**Masalah:** Jika tool resume dari byte offset X tanpa tahu lexer state di titik itu, ia akan salah menempatkan boundary — misalnya, jika dump punya `SET sql_mode='NO_BACKSLASH_ESCAPES'` di header tapi lexer resume dengan default (escape ON), maka `'it\'s'` akan di-parse salah dan `;` akan muncul di tempat yang salah, merusak batch dan checkpoint offset.

**Opsi yang dievaluasi:**

1. **Asumsi semua `SET` statement ada di header, scan ulang header saat resume** — fragile; Context7 tidak konfirmasi mariadb-dump guarantees ini, dan jika asumsi salah (ada `SET sql_mode` di tengah dump), lexer state jadi salah setelah titik itu.

2. **Simpan lexer state di checkpoint (4 kolom tambahan)** — robust, ~32 bytes overhead per checkpoint, lexer bisa langsung resume dengan state yang benar tanpa scan.

3. **Scan dari byte 0 setiap resume untuk rebuild state** — correct tapi O(N) lambat, mengalahkan tujuan fast resume, dan membebani I/O untuk dump besar.

## Keputusan

**Checkpoint Store menyimpan lexer state lengkap**. Schema ditambah 4 kolom:

- `no_backslash_escapes` BOOLEAN — default FALSE (escape ON, MariaDB default)
- `ansi_quotes` BOOLEAN — default FALSE (`"` adalah string literal)
- `current_delimiter` TEXT — default `';'`
- `charset` TEXT — default NULL, diisi dari `SET NAMES` terakhir

Saat tool menulis checkpoint (setelah COMMIT atau sebelum implicit-commit statement), ia snapshot lexer state saat itu ke 4 kolom ini. Saat resume, tool membaca checkpoint, restore byte offset DAN lexer state, lalu lanjutkan parsing.

**Overhead:** ~32 bytes per checkpoint row (2 boolean @ 1 byte, 2 TEXT — delimiter biasanya 1-10 karakter, charset biasanya 5-10 karakter seperti `utf8mb4`). Dengan checkpoint per-Batch (~64MB/~1000 statements), ini negligible dibanding ukuran dump.

**Konsistensi:** Lexer sudah observe dan mirror `SET sql_mode` dan `DELIMITER` in-stream (ADR-0023, ADR-0025); charset sudah dibutuhkan untuk byte-to-rune decoding (ADR-0024). Keputusan ini hanya menambah persistence dari state yang sudah di-track, bukan introduce state tracking baru.

## Alasan

Context7 research untuk mariadb-dump tidak konfirmasi bahwa semua `SET` statement guaranteed ada di header — dokumentasi hanya mengatakan dump "manages sql_mode as part of session state" dan "sets it in the dump file", tapi tidak specify *dimana*. Tanpa jaminan header-only, Option 1 (asumsi + scan header) adalah fragile — jika ada `SET sql_mode` di tengah dump, lexer state salah setelah titik itu dan boundary corruption muncul, yang adalah exact failure yang ADR-0023/0024 exist untuk prevent.

Option 3 (scan from byte 0) adalah correct tapi defeats fast resume — untuk dump 50GB, resume dari byte 40GB akan scan 40GB setiap kali, menghabiskan I/O dan waktu. Fast resume adalah core requirement (CONTEXT.md: "jump to the exact byte and continue").

Option 2 (save state in checkpoint) adalah only approach yang both correct dan fast: lexer state di checkpoint X adalah state yang benar untuk parse dari byte offset X, karena state itu adalah hasil dari observe semua `SET`/`DELIMITER` statement sampai titik X. Tool tidak perlu guess, tidak perlu scan — tinggal restore dan continue.

Overhead 32 bytes per checkpoint adalah acceptable: dengan checkpoint granularity per-Batch (~64MB), untuk dump 10GB ada ~160 checkpoint, total overhead ~5KB — negligible.

## Konsekuensi

- Checkpoint Store schema bertambah 4 kolom; existing checkpoint records (jika ada) perlu migration untuk populate kolom baru dengan default values (safe karena jika resume dari old checkpoint, tool bisa fallback scan header sekali untuk populate state, lalu update checkpoint row — migration one-time cost).
- Lexer's checkpoint-write code path harus snapshot 4 state bits selain byte offset dan statements count.
- Lexer's resume code path harus restore 4 state bits dari checkpoint sebelum scan lanjutan.
- CONTEXT.md **Checkpoint Store** term (lines 101-126) harus di-update untuk reflect schema baru.
- Testing harus cover: resume setelah `SET sql_mode`, resume setelah `DELIMITER`, resume dengan charset non-default, verify boundary tetap correct after resume.
- Ini adalah final piece untuk guarantee "byte offset + lexer state = correct boundary placement on resume", closing the loop dari ADR-0023 (stateful lexer), ADR-0024 (byte offset), ADR-0025 (mirror sql_mode).
