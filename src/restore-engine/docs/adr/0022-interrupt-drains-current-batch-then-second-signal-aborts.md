# An interrupt drains the current batch and stops clean; a second signal aborts to the crash path

A 9GB+ restore is long enough that an operator will routinely stop it deliberately —
Ctrl-C at a terminal, `SIGTERM` from a scheduler tearing down a job. That is *not* a
crash: it is a handleable signal, and the tool can choose what to do with it. The
question this ADR pins is whether a deliberate interrupt is treated exactly like a
power loss (die instantly, lean on crash-resume) or caught and turned into a clean
stop at a batch boundary — and what a *second* signal then means. This is a contract
operators script against (exit codes, "is it safe to Ctrl-C?"), so it is recorded.

## Why a clean drain, not "treat Ctrl-C like kill -9"

Crash-resume already makes an abrupt death *safe*: after any crash the persisted
`byte_offset` lags the last durable MariaDB COMMIT by at most one batch, and the
**Resume Batch** absorbs that one replayed batch's `1062`/`1050` (ADR-0018). So
treating an interrupt as an instant kill would be *correct*. But it wastes the one
thing an interrupt has that a power loss does not: **warning**. The signal arrives
while the process is still alive and in control of its loop, so it can finish the
batch it is in, let that batch's COMMIT + checkpoint write complete, and exit with the
stored offset sitting **exactly** on a batch boundary. The next `restore <file>` then
resumes with **zero** replay, not even the one batch a crash would cost. Same-safe,
strictly cleaner — the drain simply spends the warning the operator gave.

## The two-signal contract

- **First signal** (`os.Interrupt`; also `SIGTERM` on Unix) → enter *drain*: set a
  stop flag / cancel the batch loop's context at the next safe boundary. The batch
  currently executing runs to its COMMIT and checkpoint write, then the tool exits
  clean. A notice is printed: "interrupt received — finishing the current batch, then
  stopping; run `restore <file>` again to continue." Nothing is left half-committed;
  the offset is on a boundary.
- **Second signal** (operator is impatient, or the current batch is a huge
  minimum-one-rule statement that will take a while) → **abort immediately**. This
  drops straight onto the proven crash path: whatever batch was mid-flight is
  abandoned, and resume costs at most the one batch the Resume Batch already tolerates.
  The second signal is therefore **never unsafe** — it only trades zero-replay
  cleanliness for a faster stop.

## Why `signal.NotifyContext` alone does not arm the second signal — and what does

The idiomatic first half is `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`:
the first signal cancels the context, which the batch loop checks at its safe
boundary. But context7 (`/golang/go`, `os/signal`) makes explicit that this **also
suppresses the default exit for every later signal** until the returned `stop` is
called: *"Future interrupts received will not trigger the default (exit) behavior
until the returned stop function is called."* So a naïve `NotifyContext` would make
the operator's second Ctrl-C **do nothing** — the opposite of the "second signal
aborts" contract.

The docs say `stop` *"may restore the default behavior"* — but "may" is
platform-conditional, and this tool is portable across Linux and Windows, so relying
on it is not deterministic. The chosen mechanism is therefore an explicit **counting
handler** rather than bare `NotifyContext`: a `signal.Notify` channel where the first
delivery triggers the drain and the second calls `os.Exit` directly with the interrupt
exit code. The abort does not depend on "may restore default" doing anything.

## Portability of the signal set (stated honestly)

`os.Interrupt` (Ctrl-C) is delivered on **both** Linux and Windows, so the
cross-platform contract rests on it. `SIGTERM` is a Unix concept and is **not**
delivered the same way on Windows; it is handled as an additional trigger on Unix
(schedulers, `kill`) and is **not** claimed as equivalent on Windows. The tool does
not pretend a uniform `SIGTERM` story across platforms it does not have.

## Exit codes for a deliberate stop

A deliberate interrupt is **not** a **Fatal Error**, so it must not reuse Fatal `1`.
Conflating "the operator chose to stop" with "a statement failed / the connection
dropped" is exactly the mistake the four-value **Exit Code** contract exists to
prevent — a cron that retries on `1` should not retry an operator's Ctrl-C the same
way it retries a broken restore. The tool follows the shell convention of `128 +
signal number` for signal-caused exit — **`130`** for `SIGINT`/Ctrl-C, **`143`** for
`SIGTERM` — for both the clean drain and the second-signal abort (both are
interruptions; the difference is replay cost, not outcome). This is a shell/waitstatus
convention (e.g. bash), **not** a Go stdlib guarantee, and is recorded as a deliberate
choice, not an inherited default. These codes sit outside the `0`/`3`/`4`/`1` range,
so they extend the Exit Code contract without colliding with it. In every case the
checkpoint row survives and `restore <file>` resumes.

## Decision

- A first `os.Interrupt` (and `SIGTERM` on Unix) triggers a **graceful drain**: finish
  the in-flight batch through its COMMIT + checkpoint write, print a resume notice,
  exit clean with the offset on a batch boundary (zero replay on the next run).
- A **second** signal **aborts immediately** onto the crash-resume path (replay ≤ one
  batch, absorbed by the Resume Batch — never unsafe, only less clean).
- The second signal is armed by an explicit counting `signal.Notify` handler, **not**
  by bare `NotifyContext`, because `NotifyContext` suppresses the default exit for
  later signals (context7 `/golang/go`) and its `stop` only *may* restore it — too
  weak for a portable Linux+Windows guarantee.
- The cross-platform trigger is `os.Interrupt`; `SIGTERM` is Unix-only and not claimed
  equivalent on Windows.
- A deliberate interrupt exits with the `128 + signo` convention (`130` for SIGINT,
  `143` for SIGTERM), **distinct from Fatal `1`**, so scripts can tell an operator
  stop from a restore failure. Shell convention, not a stdlib guarantee.

## Considered Options

- **Treat an interrupt exactly like a crash (die instantly):** rejected — correct but
  wasteful. The signal arrives with the process alive and in control; spending that
  warning on a batch-boundary drain gives zero-replay resume for free. (Kept as the
  *second-signal* behaviour, where a fast stop is what the operator is asking for.)
- **Drain but ignore/swallow the second signal (pure `NotifyContext`):** rejected —
  leaves an operator unable to force a stop during a long in-flight batch (e.g. a
  multi-hundred-MB minimum-one-rule statement). context7 confirms `NotifyContext`
  suppresses later signals' default exit, so this is the *accidental* behaviour of the
  naïve implementation — precisely what the counting handler exists to avoid.
- **Abort on the first signal, no drain:** rejected — throws away the one advantage an
  interrupt has over a crash for no benefit; the common case (operator stops a run
  they will resume) should be the clean one.
- **Exit `1` (Fatal) on interrupt:** rejected — makes a deliberate, blameless stop
  indistinguishable from a real failure, defeating the Exit Code contract's whole
  point. A cron's error-handling for `1` would misfire on an operator's Ctrl-C.
- **Exit `0` on interrupt:** rejected — an interrupted restore is *not* a completed
  one; a `0` would tell a script the dump finished when it did not. The interrupt
  codes say "stopped, resumable," which is the truth.

## Consequences

- The batch loop must check for cancellation only at a **safe boundary** (between
  batches, never mid-COMMIT), so a drain can never leave a half-applied batch; this is
  the same boundary the checkpoint write already uses.
- Signal handling is an explicit counting handler; a reviewer who replaces it with a
  bare `NotifyContext` reintroduces the swallowed-second-signal bug and must be
  stopped.
- The **Exit Code** term gains the interrupt codes (`130`/`143`) alongside `0`/`3`/`4`/
  `1`; they mean "deliberately stopped, resumable," never "failed."
- Windows callers get the contract via `os.Interrupt` only; documentation must not
  imply `SIGTERM` behaves identically there.
- Both drain and abort leave the checkpoint row intact, so the resume story is
  unchanged from ADR-0018/0019/0020 — an interrupt is just a cleaner entry into it.
