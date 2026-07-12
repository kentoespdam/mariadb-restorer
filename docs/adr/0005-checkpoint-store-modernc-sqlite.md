# Checkpoint store is one row in pure-Go SQLite; resume rejects on dump-identity mismatch

The **Checkpoint** must survive a crash and be re-read on resume, so it lives in a
local SQLite file next to the run — not in memory, not in the target MariaDB
(which may be the very thing that crashed). The store holds **one row per restore
job**, not one row per **Batch**: each COMMIT overwrites the single row with the
new byte offset. History is not needed; only "where do we resume".

## Schema

One row, columns:

- `dump_path` — absolute path of the **Dump**, human-readable, for operator sanity.
- `dump_size_bytes` — total file size; cheapest possible change-detection.
- `dump_identity` — a **hash of the first few KB of the Dump plus its size**, NOT a
  hash of the full 9GB+ file. Fast to compute at startup, ~99% effective at
  catching "this is a different or regenerated dump".
- `byte_offset` — the committed **Statement Boundary** to `file.Seek()` to.
- `statements_done` — running count, for progress reporting.
- `updated_at` — last checkpoint write time.

## Resume guard

On resume the tool recomputes `dump_size_bytes` + `dump_identity` for the file it
was given and compares to the stored row. **On any mismatch it aborts by default**
— it never seeks a stale offset into a different byte stream, which would silently
corrupt the restore. An explicit override flag can force resume, but the safe
default is refuse.

## Library: `modernc.org/sqlite` (pure Go), not `mattn/go-sqlite3` (cgo)

Verified via context7:

- `mattn/go-sqlite3` docs: *"you are required to set the environment variable
  `CGO_ENABLED=1` and have a gcc compiler present in your system's PATH"*;
  cross-compilation needs a full cross-compiler toolchain (`CC=...-gcc`).
- `modernc.org/sqlite` is *"a CGo-free port of the C SQLite3 library"* — pure Go,
  no C toolchain, trivial cross-compilation. Its published benchmarks show it
  1.2×–5.8× slower than cgo on write-heavy workloads.

Our SQLite workload is one `UPDATE` per COMMIT (per ADR-0001), so the performance
penalty is irrelevant, while the deployment benefit — a single static binary that
runs on any restore host without dragging a C toolchain — is decisive for an ops
CLI.

## Considered Options

- **`mattn/go-sqlite3` (cgo):** rejected — faster on paper but forces
  `CGO_ENABLED=1` + gcc on every build/cross-build host, breaking the "one static
  binary" distribution story for a workload that gains nothing from the speed.
- **One checkpoint row per Batch (append-only history):** rejected — resume only
  needs the latest offset; a single overwritten row is simpler and bounded.
- **Full-file hash for `dump_identity`:** rejected — hashing 9GB+ at every startup
  is prohibitively slow; the size + head-prefix hash catches regenerated/swapped
  dumps at ~99% for negligible cost.
- **Continue blindly on identity mismatch:** rejected — seeking a stale offset
  into a changed stream silently corrupts data; refuse-by-default is the only safe
  behavior.

## Consequences

- The head-prefix hash is not cryptographically complete: a dump edited only past
  the prefix, keeping the exact same size, could pass the guard. Accepted as
  vanishingly rare; `--force-resume` and full re-run remain available.
- The SQLite file is per-job state; deleting it forces a clean restart from
  offset 0.
- `modernc.org/sqlite` registers under the driver name `sqlite` (not `sqlite3`) —
  an implementation detail to pin at wiring time.
