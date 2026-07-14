// Command mariadb-restorer restores MariaDB/MySQL SQL dump files with
// crash-resume, constant memory, and speed optimizations.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "restore":
		cmdRestore(args)
	case "profile":
		cmdProfile(args)
	case "replay":
		cmdReplay(args)
	case "verify":
		cmdVerify(args)
	case "tui":
		cmdTUI(args)
	case "help", "--help", "-h":
		printUsage()
	case "version", "--version", "-v":
		fmt.Fprintf(os.Stdout, "mariadb-restorer %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}
