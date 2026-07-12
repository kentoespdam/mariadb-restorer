# Context: credential-vault

Credential storage and retrieval — encrypting/decrypting MariaDB connection passwords, managing master passphrases, and referencing credentials by vault-id.

## Language

**Connection Profile**:
A named, reusable set of connection settings — `name`, `host`, `port`, `user`,
`database`, plus non-secret options — stored as one row in the local SQLite file
in the **Data Directory**. Lets an operator run `restore --profile prod` instead
of re-typing flags. A profile NEVER stores a plaintext password; if the operator
saves one it is sealed by the **Credential Vault** and kept *on this same row* as
a ciphertext column (or, if none is saved, the password is supplied at run time).
A profile is created **password-less**: `save` writes only the non-secret
settings and never touches the **Master Passphrase**. `save` is an **upsert** —
re-running it on an existing `name` updates the fields given, keeps the rest, and
leaves `sealed_password` untouched (there is no separate `edit` verb); it prints
`created` vs `updated` so a mistyped name is visible. This is what makes
editing `host`/`port` leave the vaulted password valid (below): the update never
touches the seal, and the AAD still matches on `name`. Sealing a password is
always the separate `set-password` step — the one command that opens the vault —
so `save` stays non-interactive and script-safe, and a profile that never gets a
`set-password` is a first-class state (its password comes from a run-time
**Credential Source**). Deletion is the mirror: `delete` is a metadata
operation that drops the row (and, with it, any `sealed_password`) and NEVER
requires the Master Passphrase — you need not open a secret to discard it, so a
profile whose passphrase is lost is still removable rather than an
undeletable zombie. Its only gate is an interactive `y/N` confirmation (`--yes`
to skip for unattended use). Listing is likewise passphrase-free and touches no
secret: `list` shows the non-secret settings plus a presence-only password
column derived from `sealed_password IS NOT NULL` (`vaulted` / `—`) — never a
length, prefix, or any value from the plaintext, and it does **not** verify the
seal (an AAD/`vault-id` mismatch is only detectable by attempting `Open`, which
needs the passphrase, so mismatch detection is deferred to restore time,
fail-closed). The `name` is the profile's *unique identity*: it keys
the profile record and is
the `profile-id` bound into that row's sealed-password AAD (Credential Vault) —
there is no separate internal ID. A consequence: because only `name` is
identity, editing a profile's `host`/`port` does **not** invalidate its vaulted
password (the AAD still matches on `name`), so redirecting `prod` to a new address
reuses the existing credential rather than forcing a conscious re-entry — the
accepted trade-off for treating the name, not the destination, as identity.
Renaming a profile changes its AAD and therefore re-seals its vault entry.
_Avoid_: connection string, DSN (a profile is non-secret settings only).

**Credential Vault**:
The at-rest storage for every **Connection Profile**'s DB password, encrypted with
AES-256-GCM. The vault is **not** a separate file: it is realized *inline* in the
same **Data Directory** SQLite store, each profile's sealed password kept as a column
on its own **Connection Profile** row (the model proven in the sibling
`mariadb-magic` tool, where `password_ciphertext` lives on the `connections` row). A
profile and its secret are therefore one record — there is no second file to drift
out of sync, and deleting a profile deletes its ciphertext with it. A single
vault-level **Key Encryption Key (KEK)** is derived once from the **Master
Passphrase** via Argon2id; the salt (`crypto/rand`) and KDF parameters live in the
vault's single **settings row**, recorded as one PHC string
(`$argon2id$v=19$m=65536,t=3,p=4$<salt>$…`) — not per profile. Each profile's password
is then sealed directly under that KEK with a fresh random 96-bit nonce per seal
(no per-profile KDF, no intermediate data-encryption-key layer — a KEK sealing a few
dozen passwords stays far under GCM's 2³²-message-per-key limit, so envelope wrapping
buys nothing here but complexity). That column therefore holds only the sealed
`nonce ‖ ciphertext ‖ tag`; salt and parameters are read from the settings row. Each
seal also binds **Additional Authenticated Data** — the profile's identity
concatenated with the vault's own random ID (`profile-id ‖ vault-id`, `vault-id` a
UUID in the settings row) — which GCM authenticates but does not encrypt. The AAD is
not stored on the profile row; it is reconstructed at open time from the row's own
identity and the settings row. This makes a *ciphertext-swap* fail loudly instead of
silently: a sealed value moved to a different profile's row (both under the same KEK)
breaks on the profile mismatch, and one transplanted into a *different* vault breaks
on the `vault-id` mismatch — `Open` returns an authentication error rather than the
wrong host's password. Copying the whole **Data Directory** is unaffected because the
`vault-id` travels in the same SQLite store. The KDF parameters are fixed at the
RFC 9106 §4 second option — `time=3, memory=64 MiB, threads=4`, deriving a 32-byte
(AES-256) key from a 16-byte salt — chosen over the first option's 2 GiB memory cost
because a 2 GiB transient allocation would betray this tool's constant-memory promise
when restoring a 9 GB+ dump. The parameters are **not** a user-facing knob, but
because they live in the settings row the hard-coded default may be raised in a future
version without stranding vaults written under the old cost. Because the KEK is never on disk, copying the
**Data Directory** yields only ciphertext — real encryption, not obfuscation. GCM
also detects tampering. Contrast the rejected `.mylogin.cnf`, which *obfuscates* (a
false sense of security). The vault is only one link in the **Credential Source**
precedence: an explicit `--password-file` or `--password` supplied in the invocation
outranks it, and it in turn outranks the ambient `MYSQL_PWD` env var and the
interactive prompt.
_Avoid_: keystore, keyring (the vault is our own AES file, not an OS keyring).

**Master Passphrase**:
The human-held secret that unlocks the **Credential Vault**. Entered at a no-echo
TTY prompt (`term.ReadPassword`) or, for unattended runs, read from an env var —
NEVER written into the **Data Directory** (doing so would collapse the vault back
into obfuscation). It is the one piece of state that deliberately does not travel
with the portable folder; it lives in the operator's head or an operator-managed
`0600` file.
_Avoid_: master password, key (it derives the key; it is not the AES key itself).

**Credential Source**:
Where the DB password for a run comes from, resolved by one fixed precedence chain
applied whether or not the profile has a vaulted password: explicit `--password-file`,
then explicit `--password`/`-p`, then the vaulted `sealed_password`, then the ambient
`MYSQL_PWD` env var, then a no-echo TTY prompt (`term.ReadPassword`, TTY-gated by
`term.IsTerminal`). The first source that yields a password wins; an explicit
invocation source deliberately overrides the vault, and the vault overrides env.
`--password`/`-p` is honored but warned about (MariaDB's own docs note it is visible
in `ps`/shell history). A headless run with no source fails closed rather than hangs.
_Avoid_: fallback chain (it is a full precedence order, not only a no-vault fallback).
