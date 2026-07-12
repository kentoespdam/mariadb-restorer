# Vault AES-GCM seals bind `profile-id ‖ vault-id` as Additional Authenticated Data

The **Credential Vault** (CONTEXT.md) seals every **Connection Profile**'s DB
password under one vault-level KEK (ADR-0014) with AES-256-GCM. GCM already gives
confidentiality plus integrity of the ciphertext itself. The open question was
whether the seal should *also* be bound to the identity of the thing it belongs to.
It should. This is an at-rest format decision — the AAD is part of what every
`Open` must reconstruct — so it is recorded here.

## The attack a bare seal does not stop

All entries in a vault are sealed under the *same* KEK. That is deliberate
(ADR-0014: one KEK, direct-seal, no envelope). But it means a ciphertext lifted from
one entry decrypts perfectly under any other entry, because the key is identical.
So:

- **Intra-vault swap.** An attacker (or a corrupted write, or a careless hand-edit
  of the store) copies the `staging` profile's sealed blob into the `prod`
  profile's row. The KEK is the same, GCM authentication passes, and the tool
  connects to the `prod` host using the *staging* credentials — or, worse, uses
  `prod` credentials the operator believes are scoped to `staging`. No error. A
  restore runs against the wrong database with the wrong secret.
- **Cross-vault transplant.** A blob is moved from vault A into vault B. If the two
  vaults were unlocked by the same passphrase — a real case, one operator, same
  memorized secret across machines — the KEKs collide only when the salts collide,
  but nothing structurally *stops* the attempt from being meaningful; the format
  should reject it outright rather than depend on salt luck.

GCM's ciphertext integrity does not catch either: nothing in the sealed bytes says
*which profile* or *which vault* they were sealed for.

## The fix: authenticate identity without encrypting it

AES-GCM's AEAD interface authenticates a fourth argument — Additional Authenticated
Data — that it does **not** encrypt (confirmed from `pkg.go.dev/crypto/cipher`:
`Seal(dst, nonce, plaintext, additionalData)` / `Open(dst, nonce, ciphertext,
additionalData)`; "the additional data ... must match the value passed to Seal", and
`Open` returns an error otherwise — context7 was unreachable at decision time, so the
source is the package doc, not a fabricated context7 citation). AAD is exactly right
for identity: the profile name and vault ID are not secrets, they must not be
encrypted (we need them in the clear to *find* the row), but they must be
tamper-evident and bound to the ciphertext.

We bind **`profile-id ‖ vault-id`**:

- `profile-id` — the profile's stable identity. Defeats the intra-vault swap: the
  `prod` row reconstructs AAD with `profile-id=prod`, but the transplanted blob was
  sealed under `profile-id=staging`, so `Open` fails authentication.
- `vault-id` — a random UUID minted once and stored in the **settings row**.
  Defeats the cross-vault transplant: a blob sealed under vault A's `vault-id` fails
  to open in vault B, whose settings row carries a different `vault-id`.

The AAD is **not** stored on the profile row. It is *reconstructed* at open time from
the row's own profile identity and the settings row's `vault-id`. Storing it would be
redundant and would invite an attacker to rewrite the stored AAD to match a
transplanted blob — reconstruction from independent sources is what makes the bind
meaningful.

## Why this does not break legitimate portability

Copying the **whole Data Directory** (the portability story) is unaffected: the
`vault-id` lives in the settings row and travels with the store, so every profile
still reconstructs the AAD it was sealed with. The bind rejects moving *one blob*
between contexts, not moving *the store* between machines.

## Decision

- Every vault seal passes `additionalData = profile-id ‖ vault-id` to GCM `Seal`;
  every open reconstructs the same AAD from the row's profile and the settings row's
  `vault-id` and passes it to `Open`.
- `vault-id` is a `crypto/rand` UUID written once into the settings row.
- The AAD is reconstructed, never stored in the entry.
- A profile-identity or vault-identity mismatch surfaces as an `Open` authentication
  error (fail-closed), not a silent wrong-credential connection.

## Considered Options

- **No AAD (bare GCM):** rejected — leaves the intra-vault swap and cross-vault
  transplant silently exploitable, precisely because ADR-0014's single-KEK design
  makes all ciphertexts interchangeable under the key.
- **AAD = profile-id only:** rejected as insufficient — stops the intra-vault swap
  but not the cross-vault transplant between two same-passphrase vaults. Adding
  `vault-id` costs nothing and closes it.
- **Store the AAD alongside the entry:** rejected — redundant, and an attacker who
  can rewrite the blob can rewrite a stored AAD to match. Reconstruction from the
  row + header is what gives the bind its teeth.
- **Per-key separation instead of AAD (a distinct subkey per profile):** rejected —
  that is the envelope/DEK layer ADR-0014 already declined; a few dozen passwords
  stay far under GCM limits, and AAD achieves the identity bind without reintroducing
  per-profile key management.

## Consequences

- The settings row gains a `vault-id` (UUID) field; the decrypt path must read it
  and the row's profile identity *before* calling `Open`.
- Renaming a profile changes its `profile-id` contribution to the AAD; a rename must
  therefore re-seal that profile's entry under the new identity (cheap — one GCM
  seal, no KDF). This is documented as a rename-time step, not a migration.
- Tampering, swaps, and transplants all converge on the same failure mode — a GCM
  authentication error on open — which the operator surface reports as a corrupt or
  mismatched vault entry rather than proceeding with wrong credentials.
