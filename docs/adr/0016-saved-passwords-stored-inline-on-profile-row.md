# Saved passwords are stored inline on the profile row, not in a separate vault file

The **Credential Vault** (CONTEXT.md) needs an at-rest home. The question is *where*
the sealed password physically lives relative to the **Connection Profile** it
belongs to. This is an at-rest format decision — costly to change once operators hold
real data — so it is recorded here. It sits on top of ADR-0014 (one vault-level KEK,
PHC recorded once) and ADR-0015 (each seal binds `profile-id ‖ vault-id` as AAD).

## Decision: inline column on the profile row

There is **no separate vault file**. The tool already keeps its state in a single
SQLite store in the **Data Directory** (ADR-0007). The sealed password is a
**ciphertext column on the profile's own row** in that same store; the KDF material
(salt, PHC parameters, `vault-id`) lives in one **settings row** in the same store.

```
profiles(name PK, host, port, user, database, …, sealed_password BLOB NULL)
settings(id, kdf_phc TEXT, vault_id TEXT, …)         -- single row
```

`sealed_password` holds exactly `nonce ‖ ciphertext ‖ tag`; it is `NULL` when the
operator has not saved a password (the run-time credential fallback then applies).

This is the model already proven in the sibling **mariadb-magic** tool, whose
`connections` row carries a `password_ciphertext` column and whose `app_settings`
row carries the single salt + KDF parameters (one KEK for the store). Adopting it
here keeps the two tools' credential handling aligned and lets us reuse a design that
has already survived contact with real use.

## Why inline, not a second file

- **One record, one source of truth — no drift.** A profile and its secret are the
  same row. There is no way for a vault file to list an entry whose profile was
  deleted, or a profile whose vault entry went missing. Deleting the profile deletes
  the ciphertext in the same statement; a `DELETE`/`UPDATE` is atomic under SQLite's
  transaction. A separate file would need reconciliation logic to detect and repair
  orphaned entries and dangling references — pure accidental complexity.
- **Portability is already solved by ADR-0007.** The Data Directory is copied whole;
  an inline column travels with it exactly as a separate file would, and the
  `vault-id` in the settings row travels too (so ADR-0015's cross-store bind still
  holds). A second file adds nothing to portability and one more thing to forget to
  copy.
- **Consistency with ADR-0007's own wording.** ADR-0007 already said the sealed
  bytes sit *"beside the profile row."* Inline storage is the literal realization of
  that; a separate JSON file was a later over-elaboration that drifted from it.
- **No format-versioning surface to maintain.** SQLite's schema *is* the format;
  migrations are schema migrations, which the store needs regardless. A bespoke
  `vault_version` integer and unknown-key handling in a hand-rolled JSON document
  would be a second, parallel versioning scheme for no gain.

## Considered Options

- **Separate versioned-JSON vault file (the earlier draft of this ADR):** rejected.
  It introduced profile↔vault drift (orphan entries, dangling references) that has to
  be detected and repaired, plus its own `vault_version` scheme — all to store a few
  dozen sealed blobs that already have a natural home on the profile row. It was
  over-engineering relative to the proven mariadb-magic inline design.
- **A separate SQLite table (not a column) for sealed passwords:** rejected as
  needless indirection — a 1:1 relationship to the profile row is a column, not a
  side table; a join buys nothing and reintroduces the same delete-consistency care a
  column gets for free.
- **Custom binary vault format:** rejected — length-prefix/magic/endianness
  complexity for data that SQLite already stores and versions.

## Consequences

- The schema carries `sealed_password` on the profile row and a single settings row
  for the KDF PHC string + `vault-id`. There is no vault file to open, version, or
  keep in sync.
- Deleting or renaming a profile is a single-row operation that keeps the secret and
  its identity consistent by construction; rename still re-seals under the new AAD
  (ADR-0015), now an `UPDATE` of the same row.
- ADR-0014's forward-migration story is unchanged: the PHC string in the settings row
  still lets the hard-coded default cost rise without stranding existing seals.
- The header-authentication question that a standalone file raised (whether to MAC a
  plaintext header) disappears: there is no separate header. The `kdf`/`vault-id`
  fail-closed properties from ADR-0014/0015 still hold on the settings row, and a
  tamper-and-return attack remains out of the offline-theft threat model.
