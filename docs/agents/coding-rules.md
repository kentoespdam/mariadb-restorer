# Coding Rules untuk MariaDB Restorer

> Berlaku untuk semua AI agent yang menulis kode pada proyek ini.
> Dipatuhi **sebelum** menulis kode apa pun.

---

## 1. Pahami Plan/Issue Secara Mendalam

- **WAJIB** baca issue description, design, notes, dan acceptance criteria sebelum menulis kode.
- **WAJIB** baca `CONTEXT.md` dan `docs/adr/` yang relevan dengan area yang dikerjakan.
- **WAJIB** panggil `bd show <id>` untuk melihat detail issue, dependensi, dan blocker.
- JANGAN pernah menebak-nebak spesifikasi. Jika ada yang kurang jelas, tanyakan ke user.
- Jika mengerjakan issue yang bergantung pada issue lain, pastikan dependensi sudah selesai.

## 2. Utamakan GitNexus & Context7

- **GitNexus**: Sebelum mengedit simbol apa pun (fungsi, struct, method), jalankan:
  - `gitnexus_impact({target: "symbolName", direction: "upstream"})` — untuk tahu blast radius.
  - `gitnexus_detect_changes()` — sebelum commit untuk verifikasi perubahan.
  - Jika impact analysis mengembalikan risiko HIGH/CRITICAL, **WAJIB** laporkan ke user.
  - JANGAN rename simbol dengan find-and-replace. Gunakan `gitnexus_rename`.
  - Jangan edit fungsi/class/method tanpa `gitnexus_impact` terlebih dahulu.

- **Context7**: Untuk mencari dokumentasi library/framework terbaru (Bubble Tea, Lip Gloss, modernc/sqlite, dll):
  - Gunakan skill `context7` untuk lookup API yang tepat.
  - Jangan mengandalkan ingatan saja — dokumentasi bisa berubah.

## 3. Terapkan KISS & DRY

- **KISS** (Keep It Simple, Stupid):
  - Pilih solusi paling sederhana yang memenuhi acceptance criteria.
  - Jangan tambahkan abstraksi yang belum diperlukan.
  - Satu fungsi melakukan satu hal (SRP — Single Responsibility Principle).
  - Hindari over-engineering: "kita mungkin butuh ini nanti" bukan alasan valid.

- **DRY** (Don't Repeat Yourself):
  - Jika pola yang sama muncul 2+ kali, ekstrak ke fungsi/type bersama.
  - Reuse helper functions dari package yang sudah ada.
  - JANGAN reimplementasi yang sudah ada di codebase.
  - Gunakan constant untuk magic values, jangan string/number literal berulang.

## 4. Batasi Baris per File

- **Maksimum 150 baris per file** (termasuk package declaration, imports, comments, dan blank lines).
  - File yang melebihi 150 baris **WAJIB** dipecah.
  - Exception: file auto-generated, file konfigurasi, atau data fixture.
  - Test file juga terikat aturan ini — pisahkan test ke file terpisah jika perlu.

- **Mengapa 150 baris?**
  - Memaksa fokus — satu file, satu concern.
  - Memudahkan code review — perubahan yang terisolasi lebih cepat diverifikasi.
  - Mengurangi merge conflict — beberapa developer bisa kerja di file berbeda.
  - AI agent bisa memahami file kecil lebih akurat.

- **Strategi pemecahan file di Go:**
  - `types.go` — type definitions, constants
  - `init.go` — konstruktor, initialization logic
  - `<feature>.go` — implementasi fitur
  - `<feature>_test.go` — test untuk fitur tersebut
  - `options.go` — functional options pattern

## 5. Siklus Kerja: Issue → Code → Close → Push

Setelah selesai mengerjakan sebuah issue, ikuti protokol ini:

```
1. bd close <id>           — tutup issue setelah kode selesai
2. git status              — periksa perubahan
3. git add <files>         — stage file yang relevan
4. git commit -m "..."     — commit dengan pesan deskriptif
5. git pull --rebase       — tarik perubahan terbaru
6. bd dolt push            — push beads ke remote
7. git push                — push ke GitHub
```

- **Jangan skip:** Work tidak selesai sampai `git push` sukses.
- **Jika push gagal:** retry sampai berhasil.
- **Jangan bilang "ready to push when you are"** — agent yang melakukan push.
- **Sebelum commit:** jalankan `gitnexus_detect_changes()` untuk verifikasi scope perubahan.

## 6. Kualitas Kode

- **Error handling:** Semua error harus di-handle. Jangan diamkan dengan `_`.
  - Di Go: `if err != nil { return fmt.Errorf("context: %w", err) }`
  - Jangan panik kecuali untuk hal yang benar-benar fatal (misal: init config error).
- **Logging:** Gunakan structured logging. Jangan `fmt.Println` untuk debug di production code.
- **Testing:**
  - Tulis test sesuai area yang diubah (unit test untuk logic, integration test untuk boundary).
  - Test harus bisa jalan tanpa koneksi eksternal (mock dependencies).
  - Coverage minimal 70% untuk package baru.
- **Linting:** Pastikan `go vet` dan `golangci-lint` (jika tersedia) tidak mengeluarkan error.

## 7. Konvensi Go Spesifik Proyek

- **Package naming:** Single word, lowercase (`checkpoint`, `profile`, `vault`).
- **File naming:** Snake case (`checkpoint_store.go`, `profile_manager.go`).
- **Error wrapping:** Gunakan `fmt.Errorf("context: %w", err)` — jangan `errors.Wrap` atau `errors.New` tanpa konteks.
- **Imports:** Kelompokkan: std → eksternal → internal. Pisahkan dengan blank line.
- **Exported vs unexported:** Ekspor hanya yang digunakan dari package lain.
- **Interfaces:** Definisi di package pengguna, bukan di package implementasi (Go idiom).
- **Context:** Pass context sebagai parameter pertama pada fungsi yang bisa block.

## 8. Dokumentasi & Comments

- **Setiap exported symbol** harus punya Go doc comment.
- **Setiap file** harus punya file-level comment: `// Package x melakukan y.`
- **Tidak ada comment yang sudah basi** — update atau hapus jika kode berubah.
- **Kenapa, bukan apa:** Comment jelaskan *mengapa* kode ditulis begini, bukan *apa* yang dilakukan kode (kode sudah menjelaskan apa).
- Untuk implementasi yang tidak intuitif, refer ke ADR: `// ADR-0014: Argon2id memory cost 64MiB, bukan 2GiB, untuk menjaga constant-memory promise.`

---

> Aturan ini bisa berubah seiring waktu. Jika ada aturan yang menghambat produktivitas tanpa alasan jelas, diskusikan dengan tim.
