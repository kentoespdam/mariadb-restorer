# Verify integrity findings get their own exit code `4`, distinct from deferred-object `3`

The tool already returned three process exit codes: `0` clean, `3` when ≥1
**Deferred Object** remained (ADR-0008), `1` for a **Fatal Error**. Adding the
optional **Verify** phase (ADR-0012) created a fourth possible outcome — the
restore finished, but Verify surfaced an integrity finding (orphan FK rows and/or
a structural `Corrupt` report). This ADR decides how that outcome is signalled:
a new exit code `4` rather than folding it into the existing `3`.

## The question

Both `3` and `4` describe "the restore *completed* but the result is not fully
sound." The tempting simplification is to reuse `3` for any "succeeded with
reservations" outcome and keep the exit-code surface small. We reject that.

## Why `3` and `4` cannot share a code: the remediations diverge

An exit code is a *contract read by machines* — cron jobs, CI steps, restore
runbooks. The whole point of a non-zero-but-not-fatal code is to let an operator
script the next step. But the next step differs fundamentally:

- **`3` (Deferred Object) has an *automated* next step.** The operator fixes the
  DEFINER users (or grants `SET USER`) and runs `replay` (ADR-0008/Replay). A
  cron can plausibly encode this: `restore … ; [ $? -eq 3 ] && replay`.
- **`4` (Verify finding) has *no* automated next step.** Orphan rows and a
  possibly-corrupt table require a *human* to inspect the data and decide what is
  correct. There is no command that "fixes" an orphan row the way `replay` fixes a
  deferred view.

If both returned `3`, a cron written to answer `3` by running `replay` would do
the **wrong thing** for a Verify finding: `replay` drains deferred metadata and
would report "nothing to replay," silently masking unsound *data*. The two codes
exist precisely so an operator's automation can branch correctly. Collapsing them
trades a real safety property for a cosmetically smaller exit-code set.

## Precedence when both hold: `4` wins over `3`

A single run can both defer an object **and** have Verify flag a finding. The exit
code can only be one number, and it must signal the **worst** condition. `4` wins:

- A **Deferred Object** is *metadata* with a known, automated remediation path —
  the least-bad reservation.
- A **Verify** finding is *unsound data* with no automated fix — the more serious
  condition, the one that most needs a human to notice.

So the code reports the worse of the two, and the printed report names **both** so
the operator does not lose sight of the deferred object. (During the grill I first
recommended the reverse — "`3` wins" — and corrected it: the exit code's job is to
surface the condition least likely to be safely automated away, which is `4`.)

## Why `4`, and not escalating Verify findings to Fatal `1`

`1`/**Fatal Error** has a precise, load-bearing meaning: fail-fast, the restore
stopped *mid-run*, the transaction rolled back, and the job is `resume`-able. A
Verify finding is none of those — it is discovered *after* the load completed, so
there is nothing to `resume`. Routing it to `1` would violate the `1` contract.

This matters most for structural `Corrupt`. `CHECK TABLE … EXTENDED` (ADR-0012)
re-scans the whole table and can report corruption beyond FK orphans. We
deliberately do **not** escalate that to Fatal, for a reason confirmed via
context7 (MariaDB release note 10.6.17-12): `CHECK TABLE` can *incorrectly* report
MyISAM/Aria tables as `Corrupt` due to an inaccurate unique-hash-key computation
on column prefixes. Letting a signal that is known to false-positive hard-stop the
tool would be wrong. Instead, both Verify finding classes — FK orphans and
structural `Corrupt` — yield `4`, and Verify's report separates them, tagging
`Corrupt` as *possibly* a false positive for MyISAM/Aria so the human knows to
verify manually rather than trust it as ground truth.

## Decision

- Add exit code **`4`**: the restore completed but **Verify** surfaced ≥1
  integrity finding (FK orphan rows and/or structural `Corrupt`).
- Keep `0` (clean), `3` (Deferred Object remains — automated `replay` path), and
  `1` (Fatal Error — mid-run, resumable) unchanged.
- When both a Deferred Object and a Verify finding are present, return **`4`**
  (unsound data outranks deferred metadata) and name **both** in the report.
- Verify findings **never** escalate to Fatal `1`; structural `Corrupt` in
  particular stays `4`, flagged as possibly-false-positive per context7.

## Considered Options

- **Reuse `3` for any "completed with reservations":** rejected — the
  remediations diverge (automated `replay` vs. manual data inspection); a cron
  branching on `3` would mis-handle a Verify finding by running `replay` on
  unsound data.
- **Escalate Verify findings (esp. `Corrupt`) to Fatal `1`:** rejected — `1`
  means mid-run, resumable fail-fast; a post-completion finding is neither, and
  `Corrupt` is known to false-positive on MyISAM/Aria (context7), so it must not
  hard-stop.
- **`3` wins when both hold:** rejected (a mid-grill self-correction) — the exit
  code must signal the worst, least-automatable condition, which is the Verify
  finding.

## Consequences

- The exit-code surface is four values; scripts/cron can branch precisely:
  `0` proceed, `3` run `replay`, `4` summon a human, `1` fix-and-`resume`.
- The report printer must render a combined `3`+`4` case (name both the deferred
  object and the Verify finding) while returning `4`.
- Verify's report must distinguish FK-orphan findings from structural `Corrupt`,
  and annotate `Corrupt` with the MyISAM/Aria false-positive caveat.
- Operator docs must state the exit-code contract explicitly, since downstream
  automation will encode it and it is thereafter expensive to change.
