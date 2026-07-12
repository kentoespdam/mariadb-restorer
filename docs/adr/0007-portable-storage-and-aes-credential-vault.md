# Storage is executable-adjacent and portable; saved passwords use an AES-GCM vault, not the OS keyring

The tool must be **portable**: copy the folder (or the binary on a USB stick) to
another host and its saved state — resume checkpoints and saved connections —
should travel with it. This rules out an OS keyring (Keychain / Secret Service /
Windows Credential Manager) for secrets: a keyring is bound to a *host + OS user*,
not to the executable's folder, so profiles would move but passwords would not.
The keyring is exactly the anchor that would make the tool non-portable.

## Storage location — the Data Directory

State lives in a single SQLite file in the **Data Directory**, which defaults to
the **directory of the running executable**:

```
os.Executable()  →  filepath.EvalSymlinks()  →  filepath.Dir()
```

The `EvalSymlinks` step is required per the Go docs (context7): *"If a symlink was
used to start the process, the result might be the symlink or the path it pointed
to. For a stable result, path/filepath.EvalSymlinks might help."*

Because a system install directory (`/usr/local/bin`, `C:\Program Files`) is often
read-only, if the executable's directory is not writable the tool falls back to
`os.UserConfigDir()` and prints a warning. `--data-dir` overrides both. Portable
mode (binary in a user-owned folder / removable media) gets the executable-adjacent
default; system installs degrade gracefully instead of failing to start.

## Saved credentials — the Credential Vault (real encryption, not obfuscation)

A **Connection Profile** stores only non-secret settings (host, port, user,
database). If the operator chooses to save the DB password, it goes into the
**Credential Vault**, encrypted at rest:

- **Cipher:** AES-256-GCM via `crypto/aes` + `cipher.NewGCM` — authenticated
  encryption, so tampering with the file is detected on open, not silently
  decrypted to garbage.
- **Key derivation:** `argon2.IDKey` (Argon2id) from a human **Master
  Passphrase** and a single vault-level salt from `crypto/rand` — one KEK for the
  whole store, not one per profile (see ADR-0014).
- **On disk:** the salt + KDF parameters live once in a settings row; each profile
  row carries only its own `{nonce, ciphertext, tag}` sealed password column (see
  ADR-0016). Salt and nonce are not secret; only the passphrase is.
- **Unlock:** the Master Passphrase is entered at a no-echo TTY prompt
  (`term.ReadPassword`) or, for unattended runs, read from an env var.

The decisive property: **the key is never on disk.** Copying the Data Directory
yields only ciphertext — so the folder stays portable *and* the secret stays safe.
The Master Passphrase is the one piece of state that deliberately does not travel
with the folder; it lives in the operator's head (or an operator-managed `0600`
file for cron).

This is the direct application of the lesson already locked from MariaDB's own
`.mylogin.cnf`: the MariaDB docs (context7) call `mysql_config_editor` storage
*"only obfuscates passwords, not encrypts them, offering a false sense of
security."* Obfuscation stores a key the tool itself can read; encryption puts the
key behind a human secret. The vault is the latter.

## Considered Options

- **OS keyring via `zalando/go-keyring`:** rejected as the primary mechanism —
  correct place for a secret on a *fixed* host, but host-bound, so it breaks the
  portability requirement (profiles travel, passwords don't). May return later as
  an opt-in backend for fixed installs; not in the MVP.
- **Store the password in plaintext (or base64/XOR obfuscated) in the SQLite
  file:** rejected — this is exactly the `.mylogin.cnf` "false sense of security"
  the project already decided against. A copied folder would hand over the secret.
- **AES with a key hardcoded in the binary or a keyfile beside the executable:**
  rejected — looks like encryption but the key ships next to the ciphertext, so a
  copied folder yields both. This is obfuscation with extra steps; explicitly
  forbidden.
- **Never store the password at all (always prompt / env / `--password-file`):**
  viable and kept as the *fallback* when no password is vaulted, but rejected as
  the only option because the user explicitly asked to save and reuse connections.
- **scrypt instead of Argon2id for the KDF:** rejected — scrypt is sound, but
  Argon2id is the current recommendation (Password Hashing Competition winner,
  stronger GPU/ASIC resistance) and is available in `golang.org/x/crypto/argon2`.

## Consequences

- The tool gains a small credential-management surface: `profile save` /
  `profile set-password` / `restore --profile <name>`, plus the run-time fallback
  chain (env → `--password-file` → no-echo prompt) when a profile has no vaulted
  password.
- An operator who forgets the Master Passphrase cannot recover a vaulted password;
  it must be re-entered. This is inherent to real encryption and is the correct
  trade-off.
- Argon2id parameters (memory, iterations, parallelism) are tunable constants to
  pin at implementation time; they must be strong enough to be slow for a human
  and stored alongside the salt so future runs can reproduce the derivation.
- Unattended mode (Master Passphrase via env var) is supported but only safe if
  the env source is NOT written into the Data Directory; the tool must warn if it
  detects the passphrase being persisted there.
- The vault is decrypted only in memory, only for the duration of a restore; the
  plaintext password is never written back to disk.
