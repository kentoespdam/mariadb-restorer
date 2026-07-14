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

---

## 🖥️ P1 — TUI Screens (Core UI)

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 6 | **mr-143** | TUI framework layer & screen router (Bubble Tea) | ✓ Closed | ☑ |
| 7 | **mr-f8u** | Home screen — restore history viewer | ✓ Closed | ☑ |
| 8 | **mr-9l6** | Profile manager screen — CRUD UI | ✓ Closed | ☑ |
| 9 | **mr-ixu** | Restore launcher wizard screen | ✓ Closed | ☑ |

### Dependency Chain P1

```
mr-143 (TUI framework) ─┬─ mr-f8u (home)
                        ├─ mr-9l6 (profiles)
                        └─ mr-ixu (launcher)
```

---

## 📊 P2 — Advanced TUI Features

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 10 | **mr-23z** | Live progress monitor screen | ✓ Closed | ☑ |
| 11 | **mr-i1e** | Post-restore report screen | ✓ Closed | ☑ |
| 12 | **mr-6o7** | Help & glossary screens | ✓ Closed | ☑ |

### Dependency Chain P2

```
mr-143 ─┬─ mr-23z (progress)
        ├─ mr-i1e (report)
        └─ mr-6o7 (help/glossary)
```

---

## 🎨 P3 — Demo Mode & Final Polish

| # | ID | Issue | Status | Selesai |
|---|----|-------|--------|---------|
| 13 | **mr-t9h** | Demo mode & final polish | ✓ Closed | ☑ |

### Dependency Chain P3

```
mr-t9h (demo mode) ← butuh SEMUA screens (mr-f8u, mr-9l6, mr-ixu, mr-23z, mr-i1e, mr-6o7)
```

---

## 📈 Ringkasan Progres

| Priority | Total | Open | In Progress | Closed |
|----------|-------|------|-------------|--------|
| **P0** | 5 | 0 | 0 | 5 |
| **P1** | 4 | 0 | 0 | 4 |
| **P2** | 3 | 0 | 0 | 3 |
| **P3** | 1 | 0 | 0 | 1 |
| **Total** | **13** | **0** | **0** | **13** |

**Progres keseluruhan:** ▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰ 100% (13/13 selesai)

> ✅ **Semua issue sudah closed!** Proyek sudah selesai dikerjakan — tinggal integrasi & pengujian.

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

> **Last updated:** 2026-07-14
> **Coding rules:** `docs/agents/coding-rules.md`
