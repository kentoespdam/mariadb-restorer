# Progress is byte-based and TTY-adaptive, from the same offset the checkpoint stores

A 9GB+ restore runs for a long time with no natural output, so it must report
progress — but *how* is two locked choices, not one. First, what the percentage is
measured against: bytes consumed, or statements executed. Second, how the line is
rendered: an in-place bar that rewrites itself, or append-only lines. Getting either
default wrong is a real, recurring harm — a statement-based percentage cannot exist
without breaking the streaming contract, and an in-place bar written to a file
turns a log into a smear of carriage returns. Both are pinned here.

## Why the percentage is byte-based, not statement-based

The obvious percentage is `statements_done / total_statements`. But **total statement
count is unknowable without reading the whole file first**, and this tool is built to
stream a dump in constant memory (never holding more than the current **Batch**).
Pre-scanning a 9GB dump to count statements would read every byte before executing the
first one — the exact whole-file pass the streaming design exists to avoid, paid on
every run.

The dump's **total size in bytes is already known at startup**, for free: it is
`dump_size_bytes`, captured into the **Checkpoint Store** row as part of **Dump
Identity** (ADR-0019). So the honest, O(1) denominator is bytes:

```
percent = byte_offset / dump_size_bytes
```

The numerator, `byte_offset`, is the count of dump bytes consumed so far — the very
same value the checkpoint persists. `statements_done` is still tracked and shown as a
secondary counter (it is a real number the tool has), but it is **not** the basis of
the percentage, because its denominator can't be had cheaply.

## Single source of truth: the bar and the resume point cannot diverge

The percentage's numerator **is** the checkpoint's `byte_offset` — not a parallel
counter that happens to track it. This is deliberate. If progress display kept its own
"bytes processed" tally separate from what the checkpoint writes, the two could drift
(display counting bytes *read* ahead of the batch that has actually COMMITted, say),
and the operator would watch "73%" while a crash resumes from 68%. Binding the bar to
the persisted offset means **what you see is exactly where a crash resumes from** —
the progress bar is a live readout of the resume point, never an optimistic estimate
of it. Display reads the offset; it never invents one.

## Why rendering is TTY-adaptive

The tool is portable and runs anywhere on Linux and Windows — sometimes at an
interactive terminal, sometimes with stdout redirected to a file, a pipe, or a
scheduler's log. One rendering cannot serve both:

- **Interactive** (`term.IsTerminal(fd)` on stdout reports a terminal): a single
  status line rewritten in place with a carriage return (`\r`) — percent, throughput
  (MB/s), and ETA, refreshed at a throttled cadence (~once per second, not per batch).
  One line that updates is the right density for a human watching.
- **Non-interactive** (stdout is a pipe, file, or scheduler): **no `\r`**. A carriage
  return written to a file produces a single physical line overwritten thousands of
  times — useless in `less`, mangled in most log viewers, and it defeats `tail -f`.
  Instead the tool emits **append-only, timestamped progress lines** at a coarse
  cadence (every N seconds or M percent), so a redirected log stays greppable and
  `tail -f`-able without ballooning to one line per batch.

The decision to switch is made by detecting the terminal, not by a flag the operator
must remember to set for cron. `term.IsTerminal(fd int) bool` is the same
cross-platform primitive already relied on in ADR-0017 (Unix `IoctlGetTermios`,
Windows `GetConsoleMode` — re-confirmed via context7 `/golang/term`). ADR-0017 gates
stdin for the password prompt; here the same function gates **stdout** for progress
rendering. `--progress=plain|auto|none` may override the auto-detection, but the
default is `auto` and needs no configuration to do the right thing in either
environment.

## Decision

- The progress percentage is **`byte_offset / dump_size_bytes`**. `dump_size_bytes`
  is known at startup (Dump Identity, ADR-0019); total statement count is not, and
  computing it would require a whole-file pre-scan that violates constant-memory
  streaming.
- The percentage's numerator is the **same `byte_offset` written to the Checkpoint
  Store** — one source of truth, so the displayed progress and the resume point can
  never diverge. `statements_done` is shown as a secondary counter, never as the
  percentage basis.
- Rendering is **TTY-adaptive on stdout**, decided by `term.IsTerminal(fd)`:
  - interactive → one `\r`-updated status line (percent, MB/s, ETA), throttled ~1/sec;
  - non-interactive → append-only timestamped progress lines at a coarse cadence, no
    carriage returns.
- `--progress=auto|plain|none` overrides detection; `auto` is the default.

## Considered Options

- **Statement-based percentage (`statements_done / total_statements`):** rejected —
  the denominator cannot be obtained without reading the entire dump before executing
  it, which breaks the constant-memory streaming contract on every run, including runs
  that would never have needed the count. Bytes give an honest denominator for free.
- **A separate "bytes displayed" counter independent of the checkpoint offset:**
  rejected — invites drift between the bar and the resume point, so the operator sees
  a number a crash won't honour. Bind the bar to the persisted offset instead.
- **Always render the in-place `\r` bar, regardless of destination:** rejected —
  writing carriage returns to a redirected file or pipe produces an unreadable,
  self-overwriting smear and breaks `tail -f`/`grep` on the log. The most common
  unattended destination is exactly where this fails.
- **Always append plain lines, even at an interactive terminal:** rejected — scrolls a
  human's terminal with hundreds of near-identical lines when a single updating line
  is the right density. Detection lets each environment get the rendering it wants.
- **A mandatory `--progress` flag with no auto-detection:** rejected — forces every
  cron/systemd invocation to remember a flag purely to avoid a garbled log; the tool
  can see it has no TTY and should just do the right thing. The flag stays as an
  override, not a requirement.

## Consequences

- `dump_size_bytes` is load-bearing for display, not only for Dump Identity: a zero or
  unknown size (e.g. a dump streamed from a pipe with no seekable length) degrades the
  percentage to "unknown" and the tool must fall back to showing bytes-done and
  throughput without a percentage, rather than dividing by zero or lying.
- Progress rendering reads the checkpoint's `byte_offset`; it must not maintain a
  second progress counter. A reviewer who sees an independent byte tally for display
  should treat it as a divergence bug, not an optimisation.
- The `\r` path is reached only after `term.IsTerminal(stdout)` returns true; every
  other destination gets append-only lines. This mirrors ADR-0017's stdin gating with
  the same package, so the codebase has one TTY-detection story for both streams.
- Terminal-width truncation (fitting the status line to the window) is **not** decided
  here: it would need `term.GetSize`, which was not confirmed in the context7
  `/golang/term` result and must be verified separately before being relied on. The
  status line is kept short enough not to require it for now.
