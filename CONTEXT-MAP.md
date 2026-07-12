# Context Map

Multi-context domain documentation structure. Each context has its own CONTEXT.md and ADRs.

## Contexts

### restore-engine
**Path:** `src/restore-engine/CONTEXT.md`

Core restore execution logic — parsing SQL dumps, managing checkpoints, handling batches and transactions, and coordinating the restore lifecycle.

**Key domain terms:** Dump, Statement Boundary, Byte Offset, Checkpoint Store, Dump Identity, Resume Batch, Deferred Object/Store/Replay, Fatal vs Tolerable Error, Progress, Implicit-Commit Statement, TTY-adaptive behavior.

**ADRs:** `src/restore-engine/docs/adr/`

### credential-vault
**Path:** `src/credential-vault/CONTEXT.md`

Credential storage and retrieval — encrypting/decrypting MariaDB connection passwords, managing master passphrases, and referencing credentials by vault-id.

**Key domain terms:** Connection Profile, Credential Vault, Master Passphrase, sealed_password, vault-id/AAD, Credential Source.

**ADRs:** `src/credential-vault/docs/adr/`

## System-wide Terms

Terms that span multiple contexts or represent top-level operational concerns remain in the root **CONTEXT.md** — Data Directory, Exit Code, Timeout, Fast Mode, SQL_LOG_BIN, Verify.
