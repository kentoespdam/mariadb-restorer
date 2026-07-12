# Credential resolution is a single precedence chain: explicit flag > vault > env > prompt

A **Connection Profile** may carry a vaulted `sealed_password` (ADR-0016), but the
same `restore` invocation can *also* offer a password through a CLI flag, a file,
or the environment. When more than one source is present, exactly one must win, and
the rule must be the **same whether or not the profile has a vaulted password** —
two different resolution orders (one for "vault present", one for "vault absent")
is the surprise that gets an operator connecting with the wrong secret. This is a
security contract operators will script against, so it is pinned here.

## The precedence

One chain, evaluated top to bottom; the first source that yields a password wins:

```
1. --password-file <path>   explicit, recommended  (0600 file, not in ps/history)
2. --password / -p           explicit, discouraged  (visible in ps + shell history)
3. sealed_password (vault)   the profile's own saved secret (Argon2id → AES-GCM)
4. MYSQL_PWD (env)           ambient, unattended-only
5. no-echo TTY prompt        term.ReadPassword(fd) — interactive last resort
```

Nothing below the winning line is consulted. `restore --profile prod` with no
override uses the vault (line 3). `restore --profile prod --password-file x`
deliberately overrides the vault with file `x` (line 1) — the operator typed a
newer intent *now*, and it beats stored state.

## Why explicit beats the vault

Least-surprise: a value the operator supplies *in this invocation* is a conscious,
present-tense instruction; the vault is stored state from some earlier session.
Silently ignoring a flag the operator just typed (the "vault always wins" option)
is the more dangerous failure — the operator believes they overrode the credential
and did not. Precedence is **not** endorsement: `--password/-p` outranks the vault
even though MariaDB's own docs call command-line passwords insecure ("visible in
command history or process lists", confirmed via context7 `/mariadb-corporation/
mariadb-docs`). The tool honors the explicit override *and* is entitled to warn
about the `ps` exposure. `--password-file` sits above `--password` precisely because
it is the explicit path MariaDB's docs recommend over a bare `-p`.

## Why env sits *below* the vault, not above it

`MYSQL_PWD` is ambient, not explicit: it is inherited by child processes and
readable at `/proc/<pid>/environ`, and MariaDB's docs flag it directly — "using a
more secure method for sending the password to the server is strongly recommended"
(context7 `/mariadb-corporation/mariadb-docs`). The vault, by contrast, is gated by
a human-typed Master Passphrase (ADR-0014). So a passphrase-unlocked vault is a
*stronger* statement of intent than an ambient env var, and outranks it. Env keeps
a real role — the unattended fallback for a profile with **no** vaulted password —
but it never overrides one that has been deliberately sealed.

## The interactive prompt is last and TTY-gated

The no-echo prompt (`term.ReadPassword`, cross-platform: Unix termios `ECHO`-off,
Windows `SetConsoleMode` clearing `ENABLE_ECHO_INPUT` — confirmed via context7
`/golang/term`) only fires when every non-interactive source is empty **and** stdin
is an interactive terminal. The tool guards the prompt with `term.IsTerminal(fd)`
(same package): when stdin is piped or the run is headless, there is no TTY to read,
so instead of blocking forever the tool fails closed with a "no credential source
available" error. Unattended runs are expected to arrive via lines 1–4.

## Decision

- Credential resolution is the single chain above, applied identically regardless of
  whether the profile has a `sealed_password`.
- Explicit invocation sources (`--password-file`, then `--password/-p`) outrank the
  vault; the vault outranks `MYSQL_PWD`; the interactive prompt is the final resort.
- Using `--password/-p` emits a warning about `ps`/history exposure but is honored.
- The TTY prompt is reached only when lines 1–4 are empty and `term.IsTerminal`
  reports an interactive stdin; otherwise the run fails closed with a clear
  "no credential source" error rather than hanging.

## Considered Options

- **Vault always wins when present (explicit sources ignored):** rejected — a flag
  the operator just typed is silently discarded, so they connect with a credential
  they believed they had overridden. Deterministic but exactly the wrong surprise.
- **Error out when both a vault password and an explicit source are present:**
  rejected as the default — safe but noisy, and it breaks the legitimate, common
  case of "I have a saved profile but tonight I need a different password." A
  conscious explicit override should just work, not force a second `--no-vault`
  flag. (An explicit `--no-vault` may still exist as sugar for "ignore the stored
  one," but ambiguity alone is not an error.)
- **Env (`MYSQL_PWD`) above the vault:** rejected — promotes an ambient,
  process-inheritable value over a passphrase-gated secret, inverting the trust
  order MariaDB's own guidance implies.
- **Prompt first / prompt regardless of TTY:** rejected — prompting when a valid
  non-interactive source exists defeats unattended use, and prompting without a TTY
  hangs a headless run forever. Prompt is last and TTY-gated.

## The write path (`profile set-password`) mirrors the same sources

Resolution above is the *read* path — where a restore reads its password. Sealing a
new password into the vault (`profile set-password <name>`) is the *write* path, and
it accepts the same source shapes so operators learn one model: an interactive
no-echo prompt is the default (entered twice, the second time to confirm — a typo in
a sealed password is unrecoverable), with `--password-file` and `--stdin`
(`cat p | profile set-password prod --stdin`) for unattended provisioning, and
`--password/-p` honored but warned about exactly as on the read path. The command
that exists to *protect* a secret does not silently accept a leaky input without
saying so — but it does not refuse the explicit override either, keeping parity with
the read-path stance that explicit intent is honored, never overridden by the tool.

## Consequences

- Operators get one rule to remember for every profile, vaulted or not.
- `profile set-password` reads the plaintext-to-seal from the same source shapes
  (prompt / `--password-file` / `--stdin` / warned `--password`), so read and write
  paths share one mental model; the interactive prompt asks twice to confirm.
- Scripts can rely on `--password-file` deterministically overriding a saved
  password without deleting or editing the profile first.
- The tool must detect an interactive stdin (`term.IsTerminal`) before prompting and
  fail closed otherwise; a headless run with no credential source errors rather than
  hangs.
- A `--password/-p` invocation is honored but warned about; the recommended explicit
  path is `--password-file` with a `0600` file.
