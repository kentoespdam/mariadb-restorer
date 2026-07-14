package main

import (
	"fmt"
	"os"
)

func printUsage() {
	usage := `MariaDB Restorer — a resilient, streaming SQL dump restore tool.

Usage:
  mariadb-restorer restore [flags] <file>    Restore a dump file
  mariadb-restorer profile <command>         Manage connection profiles
  mariadb-restorer replay                    Re-execute deferred objects
  mariadb-restorer verify [flags] <file>     Post-restore integrity check
  mariadb-restorer tui [flags]               Launch interactive TUI mode
  mariadb-restorer help                      Show this help
  mariadb-restorer version                   Print version

Flags:
  --data-dir <path>         Data directory (default: executable directory)
  --profile <name>          Connection profile name
  --verify                  Enable post-restore integrity check
  --restart                 Restore from byte 0, ignore checkpoint
  --force-resume            Force resume despite dump identity mismatch
  --timeout <dur>           Connection timeout (default: 30s)
  --read-timeout <dur>      Read timeout (default: 5m)
  --write-timeout <dur>     Write timeout (default: 10m)
  --progress <mode>         Progress mode: auto, plain, none (default: auto)
  --password-file <path>    Read password from file
  --password <string>       Password (warning: visible in process list)
  --yes                     Non-interactive mode
  --no-color                Disable ANSI colors
  --demo                    Launch TUI in demo mode

Exit Codes:
  0     Clean restore, no deferred objects, no verify findings
  3     Restore completed with deferred objects (run 'replay')
  4     Restore completed with verify findings (inspect data)
  1     Fatal error (resumable — re-run 'restore <file>')
  130   Interrupted (SIGINT) — resumable
  143   Interrupted (SIGTERM) — resumable

More information: https://github.com/kentoespdam/mariadb-restorer
`
	fmt.Fprint(os.Stdout, usage)
}

func printProfileUsage() {
	usage := `Manage connection profiles.

Usage:
  mariadb-restorer profile save [--profile <name>] [--host <h>] [--port <p>] [--user <u>] [--database <db>]
  mariadb-restorer profile list
  mariadb-restorer profile delete --profile <name>
  mariadb-restorer profile rename <old> <new>
  mariadb-restorer profile set-password --profile <name>

Use 'profile save' without --password to create a password-less profile.
Use 'profile set-password' to seal a password into the Credential Vault.
`
	fmt.Fprint(os.Stderr, usage)
}
