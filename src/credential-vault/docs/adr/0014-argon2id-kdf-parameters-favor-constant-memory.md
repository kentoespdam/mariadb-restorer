# Argon2id KDF uses RFC 9106's 64 MiB option, not its 2 GiB one, and records its cost once in the vault settings row

The **Credential Vault** (CONTEXT.md) encrypts a **Connection Profile**'s DB
password with AES-256-GCM, deriving the key from the **Master Passphrase** via
Argon2id (`golang.org/x/crypto/argon2` `IDKey`). Two things had to be pinned:
*which* Argon2id cost parameters, and *where* those parameters live. Both are
at-rest format decisions — expensive to change once operators hold real vaults —
so they are recorded here.

## The cost parameters: RFC 9106 §4 second option

Argon2id's `IDKey` documentation cites **RFC 9106 Section 4**, which offers two
recommended parameter sets (confirmed via `pkg.go.dev/golang.org/x/crypto/argon2`
— context7 did not carry this page, so the source is the package doc itself, not a
fabricated context7 citation):

| Option | time | memory | threads |
|--------|------|--------|---------|
| First  | 1    | 2 GiB (`2*1024*1024` KiB) | 4 |
| Second | 3    | 64 MiB (`64*1024` KiB)    | 4 |

We pin the **second option**: `time=3, memory=64 MiB, threads=4`, deriving a
**32-byte** key (AES-256) from a **16-byte** `crypto/rand` salt. The RFC frames the
second option as the answer "if much less memory is available"; here the memory in
question is not the host's total but the tool's own budget while it is streaming a
9 GB+ dump in constant memory — a KDF that transiently grabs 2 GiB would fight that
budget on any host, portable or not.

## Why the weaker-looking option is the right one *here*

The instinct is "pick the strongest — 2 GiB." We reject that, for reasons specific
to this tool rather than a generic default:

- **It contradicts the product's core promise.** This tool exists to restore
  9 GB+ dumps in *constant memory*. A single KDF call that grabs a transient
  **2 GiB** competes with — or evicts — the streaming buffers the restore depends
  on. 64 MiB is negligible beside that streaming budget; 2 GiB is a self-inflicted
  wound, regardless of how much RAM the host happens to have.
- **Our threat model is a stolen local file brute-forced offline**, not a
  breached password database of millions of users. The vault is local, on the
  operator's machine, unlocked by a human-typed passphrase. `memory=64 MiB, time=3`
  is a full RFC-grade cost for that model. Maximal GPU-crack hardness is not what
  this component is defending.
- **Security strictness is deliberately the operator's call.** It is their machine
  and their risk; the tool does not need to be a bank-grade password manager. That
  makes the memory-friendly option the correct default, not a compromise.

## The parameters are hard-coded but *not* binary-only

"Hard-coded" here means **not a user-facing knob** — there is no `--argon2-memory`
flag; the operator never sees or sets these numbers. It does **not** mean the
parameters live only as a binary constant.

Storing them only in the binary is a data-loss trap. When a future version raises
the default (say 64 → 128 MiB as hardware improves), Argon2id with the new memory
cost derives a *different* key, AES-GCM authentication fails, and every vault
written under the old cost becomes **undecryptable** — silently, with no sensible
error for the operator. Five profiles created this year would die on next year's
upgrade.

So the vault records its own cost, as a standard **PHC string** in its single
**settings row** (once — a single vault-level KEK is derived from the passphrase, so
the salt and parameters belong to the vault, not to each profile):

```
$argon2id$v=19$m=65536,t=3,p=4$<salt-b64>$<...>
```

Decryption reads `m,t,p` (and the salt) from the settings row, never from the binary
constant. The hard-coded value governs only what *new* vaults are written with. A
future default bump is therefore seamless: an old vault keeps opening with its own
recorded cost, and re-deriving the KEK re-seals every profile under the new one. The
cost to the user is zero — the PHC string is metadata they never look at.

## Decision

- Argon2id (`IDKey`) parameters default to RFC 9106 §4 option two:
  `time=3, memory=64 MiB (65536 KiB), threads=4`, `keyLen=32`, 16-byte
  `crypto/rand` salt.
- These are **not** a CLI flag or config knob (no user tuning).
- The **settings row records the actual cost + salt** once as a PHC string
  (`$argon2id$v=19$m=…,t=…,p=…$…`) — one vault-level KEK, not a per-profile KDF;
  decryption reads parameters from the settings row, so the hard-coded default may be
  raised later without stranding existing vaults.

## Considered Options

- **RFC 9106 §4 first option (2 GiB, time=1):** rejected — a 2 GiB transient
  allocation competes with the streaming buffers and betrays the constant-memory
  promise the tool is built on. Its extra crack-hardness is not warranted for a
  local, human-unlocked vault.
- **Hard-code the parameters as a binary-only constant, not written to the
  vault:** rejected — the moment a future default differs, every vault written
  under the old cost becomes silently undecryptable. Safety-critical format
  parameters must travel with the ciphertext they governed.
- **Expose the parameters as CLI/config knobs:** rejected — needless cognitive
  load for operators who should not have to reason about Argon2 memory cost, and a
  footgun (a too-low setting silently weakens the vault). The default is sound;
  the recorded PHC string already handles forward migration without user input.

## Consequences

- The vault carries a PHC parameter string in its settings row (one vault-level
  KEK); each profile row stores only its sealed `nonce ‖ ciphertext ‖ tag`. The
  decrypt path parses `m,t,p` from the settings row rather than assuming the current
  default.
- A future memory-cost bump is a one-line default change plus, optionally, a lazy
  re-encrypt on next unlock; no migration is forced and no old vault breaks.
- The KDF adds ~64 MiB transient, comfortably inside the streaming memory envelope;
  the 2 GiB option is never taken.
- The tool remains free of Argon2 tuning surface — one fewer flag to document,
  misuse, or support.
