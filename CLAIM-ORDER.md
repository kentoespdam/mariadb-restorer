# Claim Order — MariaDB Restorer

> Daftar monitoring progres pengerjaan issue.
> Urutan berdasarkan **prioritas (P0→P3)** lalu **dependensi**.
> Centang checklist saat issue selesai dikerjakan & di-close.

---

## 🏗️ P0 — Foundation (Core Engine + CLI)

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 1 | **mr-h71** | Go project scaffolding — module init, directory structure, tooling | ✓ Closed | ☑ |
| 2 | **mr-96o** | Core restore engine — checkpoint store + statement splitter | ✓ Closed | ☑ |
| 3 | **mr-4d4** | CLI command structure — subcommands & flag parsing | ✓ Closed | ☑ |
| 4 | **mr-oy5** | Credential vault — AES-256-GCM + connection profile CRUD | ✓ Closed | ☑ |
| 5 | **mr-67g** | Restore engine — batching, progress, verify, deferred objects | ✓ Closed | ☑ |

### Dependency Chain P0

```
mr-h71 (scaffolding) ─┬─ mr-96o (checkpoint+splitter) ── mr-67g (full restore)
                      ├─ mr-4d4 (CLI)
                      └─ mr-oy5 (vault)
```

> **Catatan:** `mr-96o`, `mr-4d4`, dan `mr-oy5` bisa dikerjakan paralel setelah `mr-h71` selesai.
> `mr-67g` baru bisa dimulai setelah `mr-96o` selesai.

---

## 🖥️ P1 — TUI Screens (Core UI)

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 6 | **mr-143** | TUI framework layer & screen router (Bubble Tea) | ○ Open | ☐ |
| 7 | **mr-f8u** | Home screen — restore history viewer | ○ Open | ☐ |
| 8 | **mr-9l6** | Profile manager screen — CRUD UI | ○ Open | ☐ |
| 9 | **mr-ixu** | Restore launcher wizard screen | ○ Open | ☐ |

### Dependency Chain P1

```
mr-143 (TUI framework) ─┬─ mr-f8u (home) ← juga butuh mr-67g
                        ├─ mr-9l6 (profiles) ← juga butuh mr-oy5
                        └─ mr-ixu (launcher) ← juga butuh mr-67g
```

> **Catatan:** `mr-143` adalah prerequisite untuk **semua** TUI screens.
> `mr-f8u`, `mr-9l6`, `mr-ixu` bisa dikerjakan paralel setelah `mr-143`, `mr-67g`, dan `mr-oy5` selesai.

---

## 📊 P2 — Advanced TUI Features

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 10 | **mr-23z** | Live progress monitor screen | ○ Open | ☐ |
| 11 | **mr-i1e** | Post-restore report screen | ○ Open | ☐ |
| 12 | **mr-6o7** | Help & glossary screens | ○ Open | ☐ |

### Dependency Chain P2

```
mr-143 (TUI framework) ─┬─ mr-23z (progress) ← juga butuh mr-67g
                        ├─ mr-i1e (report) ← juga butuh mr-67g
                        └─ mr-6o7 (help/glossary)
```

> **Catatan:** Semua P2 bisa dikerjakan paralel setelah `mr-143` dan `mr-67g` selesai.

---

## 🎨 P3 — Demo Mode & Final Polish

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 13 | **mr-t9h** | Demo mode & final polish | ○ Open | ☐ |

### Dependency Chain P3

```
mr-t9h (demo mode) ← butuh SEMUA screens (mr-f8u, mr-9l6, mr-ixu, mr-23z, mr-i1e, mr-6o7)
```

> **Catatan:** `mr-t9h` adalah issue **paling akhir** — butuh semua TUI screens selesai terlebih dahulu.

---

## 📈 Ringkasan Progres

| Priority | Total | Open | In Progress | Closed |
|----------|-------|------|-------------|--------|
| **P0** | 5 | 0 | 0 | 5 |
| **P1** | 4 | 4 | 0 | 0 |
| **P2** | 3 | 3 | 0 | 0 |
| **P3** | 1 | 1 | 0 | 0 |
| **Total** | **13** | **8** | **0** | **5** |

**Progres keseluruhan:** ▰▰▰▰▰▰▰▰▰▰▰▰▰ 38% (5/13 selesai)

> ✅ **P0 selesai!** Semua foundation (scaffolding, engine, CLI, vault) sudah di-closed.

---

## ⚡ Quick Reference — bd Commands

```bash
# Lihat semua issue
bd list --status=all

# Lihat issue siap kerja (tidak terblokir)
bd ready

# Ambil issue baru
bd update <id> --claim

# Detail issue
bd show <id>

# Tutup issue setelah selesai
bd close <id>

# Push ke remote
git pull --rebase && bd dolt push && git push
```

---

> **Last updated:** 2026-07-13
> **Coding rules:** `docs/agents/coding-rules.md`
