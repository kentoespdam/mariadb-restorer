# `restore` auto-resumes from the checkpoint; there is no separate `resume` verb

Re-running `restore big.sql` after a crash must continue from the stored
**Byte Offset**, not start over. The open question was *how* the operator asks for
that continuation: does `restore` resume on its own, or must the operator type a
distinct verb (`resume big.sql`) to opt into continuation? This ADR pins that
`restore` auto-resumes and that no `resume` subcommand exists, because the word
"resume" had crept through CONTEXT.md as if it were a command, and left unpinned it
would harden into a ghost verb nobody actually decided to build.

## Why the verb carries no information the tool doesn't already have

The resume decision is already fully determined by data the tool holds, not by which
word the operator typed. On any `restore <file>` the tool recomputes the **Dump
Identity** (ADR-0019) and consults the **Checkpoint Store**:

- row present + fingerprint **match** → seek to the stored Byte Offset and continue;
- row present + **mismatch** → auto-restart from byte 0 with a notice (ADR-0019);
- row **absent** → fresh restore from byte 0.

All three branches are decided from the store plus the file's own bytes. A `resume`
verb would only restate a conclusion the tool has already reached — it adds no input.
Worse, requiring it creates a new ambiguity: an operator who types `restore big.sql`
while a checkpoint exists would need a *fourth* meaning ("did they mean ignore the
checkpoint?"), splitting one deterministic outcome into two verbs with near-identical
semantics. That is exactly the "two paths for one thing" this project rejected in
ADR-0017 (a single credential precedence chain) and ADR-0019 (exactly two outcomes,
never a third). Resolution stays data-driven, not verb-driven.

## The command model, kept to two shapes

- `restore <file>` — "bring this dump to completion; continue from the checkpoint if
  one exists for it." The default, and the whole happy path.
- `restore <file> --restart` — "ignore any checkpoint; reload from byte 0." The one
  conscious escape hatch, already locked in ADR-0019 (its sole non-mismatch job is the
  false-positive same-size in-place edit the fingerprint cannot see).

There is no third verb. This also matches the mental model operators bring from the
native client: `mariadb < dump.sql` has no notion of resume at all — every invocation
is one stream from the beginning — so an operator's instinct after an interrupted load
is to *run the same command again*, not to reach for a different word. Auto-resume
honours that instinct; a mandatory `resume` verb would violate it.

## Auto-resume is never silent

The mirror risk of auto-restart (ADR-0019) applies here too: an operator who believes
they are starting fresh must not silently continue an old checkpoint. So resuming is
**never silent** — when a `restore <file>` continues from a stored offset it always
prints a notice naming the dump, how far the checkpoint had progressed, the byte it is
resuming from, and that `--restart` forces a clean reload. Symmetric with the mismatch
notice: automatic action, mandatory disclosure.

## Decision

- `restore <file>` automatically resumes from the Checkpoint Store when a matching
  row exists; the operator does not type a different verb to continue.
- There is **no** `resume` subcommand. The command surface for a data load is exactly
  two shapes: `restore <file>` (auto-resume) and `restore <file> --restart` (force
  from byte 0).
- A resume is **never silent**: continuing from a stored offset always prints a notice
  (dump, percent progressed, resume byte, and that `--restart` reloads from zero),
  symmetric with the ADR-0019 mismatch notice.
- "resume" survives only as vocabulary for a *phase/behaviour* (the **Resume Batch**,
  the resume path, a restore *resuming*), never as a command an operator invokes.

## Considered Options

- **Explicit `resume <file>` verb, `restore <file>` always starts fresh:** rejected —
  makes the destructive action (`restore` = wipe and reload) the shorter, more obvious
  command and the safe action (continue) the one you must remember to ask for. An
  operator re-running the interrupted command from history would silently discard hours
  of progress. Inverts the safe default.
- **`resume <file>` as an alias that behaves identically to `restore <file>`:**
  rejected — two verbs for one behaviour is documentation debt with no gain; operators
  would wonder how they differ and invent distinctions that don't exist.
- **Auto-resume silently (no notice):** rejected for the same reason ADR-0019 rejected
  a silent restart — the operator may believe they are starting clean; continuation
  onto a partially loaded target is disclosed, even though it is not gated.
- **Prompt "resume or restart?" interactively on every re-run with a checkpoint:**
  rejected — breaks unattended/cron use (no TTY to answer), and the answer is already
  determined for the common case. `--restart` covers the rare override without a prompt.

## Consequences

- CONTEXT.md must demote every "the next `resume`" / "then `resume`s" phrasing from an
  implied command to a phase/behaviour; a reviewer must not read those as licence to
  build a `resume` subcommand.
- The resume notice is part of the tool's UX contract, not incidental logging —
  removing it turns a disclosed continuation into a silent one (same standing as the
  ADR-0019 mismatch notice).
- `--restart` is the *only* way to override auto-resume; there is no verb-level opt-out,
  keeping the command surface to two shapes.
- The idempotent EOF cleanup (a benign row whose offset already equals EOF) is reached
  by re-running `restore <file>`, not a distinct `resume` — the same auto-resume path
  seeks to EOF, finds no work, and deletes the row.
