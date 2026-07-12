# Dump Identity mismatch auto-restarts from byte 0 (with a notice), never seeks a stale offset

On `resume`, the recomputed **Dump Identity** (head-prefix + size, ADR from the
Checkpoint Store design) either matches the stored fingerprint or it does not.
The match case is obvious — seek to the stored **Byte Offset** and continue. The
mismatch case is the one that needs a locked, surprising decision: the file at
this path is *not* the file that was checkpointed, so the stored offset points into
a byte stream that no longer exists. This ADR pins what happens then, because the
wrong choice here silently corrupts a restore or silently discards a valid one.

## Why there is no third outcome

A stale **Byte Offset** is only meaningful for the exact byte stream it was recorded
against. If the operator regenerated the dump (`mariadb-dump` again) or swapped the
file, offset 4.2 GB in the new file lands in the middle of some unrelated statement.
Seeking there does not "resume" — it begins executing mid-statement, producing a
syntax error at best and a valid-but-wrong statement at worst. So "force-seek the
old offset despite the mismatch" is never correct and is not offered. Resolution is
strictly two-valued: **match → resume**, **mismatch → restart from byte 0**.

## Why the restart is automatic, not gated behind a flag

The mismatch is path-specific: same path, different content. That means the *old*
file has already been overwritten or replaced — the checkpoint it produced is
already orphaned, and keeping it resumes nothing. There is no live progress to
protect by refusing, so gating the restart behind a mandatory `--restart` flag adds
friction without protecting anything. The tool restarts automatically and treats the
new file as a fresh restore.

The one thing the operator must not lose is *awareness*, because a restart discards a
checkpoint and reloads onto a possibly non-empty target. So the auto-restart is
**never silent**: it always prints a notice naming the path, how far the discarded
checkpoint had progressed, and that it is starting over. Awareness is served by the
message, not by a blocking flag.

## Why restart-onto-a-dirty-target is safe on the default path

After a mismatch the target MariaDB may already hold rows from the *old* dump. A
restart from byte 0 is only clean if something drops that stale data first.
`mariadb-dump`'s `--opt` group is **enabled by default** and includes
`--add-drop-table`, which emits `DROP TABLE IF EXISTS` before each `CREATE TABLE`
(confirmed via context7 `/mariadb-corporation/mariadb-docs`: "`--opt` … enabled by
default … `--add-drop-table`"). So a default-shaped dump drops and recreates every
table on reload — the dirty target is cleaned by the dump's own prolog.

The exception is a `--no-create-info` / `--skip-opt` **data-only** dump: it carries
neither `CREATE TABLE` nor `DROP TABLE` (context7, same source: `--no-create-info`
"prevents including the CREATE TABLE statement", and `--skip-opt` disables
`--add-drop-table`). Reloading such a dump from byte 0 onto existing rows will hit
`1062` duplicate-key on the first `INSERT` — and that `INSERT` is a *fresh* batch,
not the first batch after a resume, so the **Resume Batch** tolerance does not apply
and the restore fails loudly. That is the correct outcome: the tool does not silently
merge data-only dumps into a dirty target, and this remains the operator's concern.

## Decision

- Dump Identity is the resume session key with exactly two outcomes: match → resume
  from the stored Byte Offset; mismatch → restart from byte 0.
- Mismatch triggers an **automatic** restart: the old `dump_identity` row is deleted
  and the file is loaded as a fresh restore. No `--restart` flag is required.
- The auto-restart **always** prints a notice (path, percent progressed on the
  discarded checkpoint, "restarting from the beginning; old checkpoint discarded").
- There is no "force-seek the stale offset" path — it is never correct.
- Restart safety relies on the dump's own `DROP TABLE` prolog (`mariadb-dump --opt`
  default); data-only dumps without it fail loudly on duplicate keys rather than
  merging silently. This is documented, not worked around.
- `--restart` exists but is **not** the gate for a mismatch (that restart is
  automatic). Its sole role is the one case the fingerprint cannot see — the
  **false-positive match** from a same-size in-place edit — where the operator
  knowingly forces a from-byte-0 reload even though head + size still match.

## The fingerprint's detection contract (and its one blind spot)

Dump Identity is deliberately cheap: head-prefix + total size, O(1) at startup, so a
9GB+ run pays nothing to compute it. That buys detection of the **common** changes —
regeneration and swap almost always alter the size or the head bytes — at the cost of
one **pathological** blind spot: a **same-size in-place edit**. If the operator hand-
edits a statement mid-file without changing the total byte count, head + size still
match, the tool treats it as the same dump, and `Seek()`s past the edited region on
resume. An edit located *before* the stored offset is then skipped entirely, and the
restore completes without ever applying the fix the operator believed they made.

This blind spot is accepted, not closed, because the alternatives cost too much for
how rare it is (editing a multi-gigabyte dump by hand is outside the normal flow):

- **Skip-region rolling hash (hash bytes `[0..offset)` at checkpoint, re-verify on
  resume):** would catch exactly this case, but forces reading the very bytes
  `Seek()` exists to skip — an 8 GB resume would read 8 GB before its first
  statement, erasing the seek advantage precisely on the largest restores.
- **Full-file hash:** catches every edit anywhere, but reads all 9GB+ on *every*
  startup, including first runs that will never resume. Punishes every run for a rare
  case.

So the contract is stated plainly rather than silently assumed: the fingerprint
**detects regeneration and swap; it does NOT guarantee detection of a same-size
in-place edit.** An operator who edits a dump in place is directed to `--restart` to
force a clean reload. Strengthening the fingerprint remains a future trade-off if
in-place editing ever becomes a real workflow, but is explicitly out of scope now.

## Considered Options

- **Refuse on mismatch, require explicit `--restart`:** rejected — the orphaned
  checkpoint protects nothing (the old file is gone), so the flag is friction, not
  safety. Awareness is fully served by the notice. (Kept the notice; dropped the
  mandatory flag.)
- **Auto-restart silently (no notice):** rejected — a restart discards a checkpoint
  and reloads onto a possibly non-empty target; the operator must be told, even
  though they are not asked. Automatic action, mandatory disclosure.
- **Force-seek the stored offset on mismatch (optional flag):** rejected outright —
  seeking a stale offset into a different byte stream is silent corruption; no flag
  makes it correct.
- **Full-file hash to catch same-size in-place edits:** out of scope here (touches
  the Dump Identity cost decision), and unnecessary for *this* choice: because the
  only mismatch action is restart-from-zero, an undetected same-size edit degrades to
  a clean restart, never a mid-stream seek. Strengthening the fingerprint is a
  separate trade-off (startup cost on every 9GB+ run).

## Consequences

- `resume` has a two-branch contract operators can rely on: same file → continues;
  different file at the same path → starts over with a printed notice.
- The notice output is part of the tool's UX contract, not incidental logging; a
  reviewer removing it turns a disclosed restart into a silent one.
- Restart correctness is inherited from the dump's `DROP TABLE` prolog; the docs must
  keep stating that data-only (`--no-create-info` / `--skip-opt`) dumps are the
  operator's responsibility on a non-empty target.
- No code path may reseek a stale Byte Offset after a fingerprint mismatch.
