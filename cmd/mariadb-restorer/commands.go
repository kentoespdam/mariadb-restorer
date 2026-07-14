package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kentoespdam/mariadb-restorer/internal/tui"
)

// resolveDataDir returns the effective data directory.
func resolveDataDir(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	// Default to current directory (portable mode).
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// cmdRestore handles the restore subcommand.
func cmdRestore(args []string) {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	dataDir := fs.String("data-dir", "", "Data directory path")
	profile := fs.String("profile", "", "Connection profile name")
	verify := fs.Bool("verify", false, "Enable post-restore integrity check")
	restart := fs.Bool("restart", false, "Restore from byte 0")
	forceResume := fs.Bool("force-resume", false, "Force resume despite identity mismatch")
	timeout := fs.String("timeout", "30s", "Connection timeout")
	readTimeout := fs.String("read-timeout", "5m", "Read timeout")
	writeTimeout := fs.String("write-timeout", "10m", "Write timeout")
	progress := fs.String("progress", "auto", "Progress mode: auto, plain, none")
	passwordFile := fs.String("password-file", "", "Read password from file")
	pwd := fs.String("password", "", "Password (warning: visible)")
	yes := fs.Bool("yes", false, "Non-interactive mode")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Set data directory for cross-command use.
	_ = resolveDataDir(*dataDir)

	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: mariadb-restorer restore [flags] <dump-file>")
		fs.PrintDefaults()
		os.Exit(1)
	}
	_ = profile
	_ = verify
	_ = restart
	_ = forceResume
	_ = timeout
	_ = readTimeout
	_ = writeTimeout
	_ = progress
	_ = passwordFile
	_ = pwd
	_ = yes

	dumpFile := remaining[0]
	fmt.Fprintf(os.Stderr, "restore: not yet implemented — file=%s\n", dumpFile)
	os.Exit(1)
}

// cmdProfile handles the profile subcommand.
func cmdProfile(args []string) {
	if len(args) < 1 {
		printProfileUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "save":
		fmt.Fprintln(os.Stderr, "profile save: not yet implemented")
	case "list":
		fmt.Fprintln(os.Stderr, "profile list: not yet implemented")
	case "delete":
		fmt.Fprintln(os.Stderr, "profile delete: not yet implemented")
	case "rename":
		fmt.Fprintln(os.Stderr, "profile rename: not yet implemented")
	case "set-password":
		fmt.Fprintln(os.Stderr, "profile set-password: not yet implemented")
	default:
		fmt.Fprintf(os.Stderr, "Unknown profile command: %s\n\n", args[0])
		printProfileUsage()
		os.Exit(1)
	}
}

// cmdReplay handles the replay subcommand.
func cmdReplay(_ []string) {
	fmt.Fprintln(os.Stderr, "replay: not yet implemented")
	os.Exit(1)
}

// cmdVerify handles the verify subcommand.
func cmdVerify(_ []string) {
	fmt.Fprintln(os.Stderr, "verify: not yet implemented")
	os.Exit(1)
}

// cmdTUI handles the tui subcommand.
func cmdTUI(args []string) {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	dataDir := fs.String("data-dir", "", "Data directory path")
	demo := fs.Bool("demo", false, "Launch in demo mode with synthetic data")
	noColor := fs.Bool("no-color", false, "Disable ANSI colors")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, "Usage: mariadb-restorer tui [flags]")
		fs.PrintDefaults()
		os.Exit(1)
	}

	// Check NO_COLOR env var (cross-platform standard).
	if os.Getenv("NO_COLOR") != "" {
		*noColor = true
	}
	_ = noColor // color mode will be implemented later

	dir := resolveDataDir(*dataDir)

	if err := tui.Run(dir, *demo); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}
