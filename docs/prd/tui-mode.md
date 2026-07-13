# TUI Mode — Interactive Terminal Interface

## Problem Statement

Operators managing MariaDB restore operations face a steep learning curve with CLI-only tools. They must memorize subcommand syntax (`restore`, `profile save`, `replay`, `verify`), remember flag combinations (`--verify`, `--password-file`, `--data-dir`), and mentally track restore history across sessions. When a restore fails mid-run or completes with deferred objects, operators lack visibility into what happened and what to do next without re-running commands to inspect checkpoint state or profile lists. This creates friction for manual operations, increases cognitive load, and makes the tool less accessible to operators who don't use it daily.

## Solution

Provide a full-screen interactive terminal UI (TUI mode) alongside the existing CLI, modeled after tools like `lazygit` and `k9s`. Launch via `mariadb-restorer tui` for a visual, keyboard-driven interface that displays restore history, manages connection profiles, launches restores through guided flows, and shows live progress — all without memorizing commands. The TUI shares the same domain logic as the CLI (both operate on the same Data Directory, Checkpoint Store, Credential Vault, and restore engine), differing only in presentation. This hybrid model satisfies both automation needs (CLI for scripts/cron) and operator usability (TUI for manual exploration and discovery).

## User Stories

1. As an operator unfamiliar with the CLI syntax, I want a visual interface with keyboard shortcuts, so that I can perform restore operations without memorizing subcommands.
2. As an operator resuming work after days away, I want to see restore history at a glance, so that I can quickly identify which dumps completed, which failed, and which are resumable.
3. As an operator managing multiple environments, I want to browse and edit connection profiles visually, so that I don't have to remember profile names or construct `profile save` commands manually.
4. As an operator setting a vault password, I want a guided flow with visual confirmation of credential source precedence, so that I understand whether my password is being overridden by a file or env var.
5. As an operator launching a restore, I want a wizard that walks me through selecting a dump file, choosing a profile, and confirming settings, so that I don't accidentally run with the wrong credentials or target database.
6. As an operator monitoring a long-running restore, I want live progress updates (statements done, byte offset, time remaining) displayed in the TUI, so that I can gauge completion without polling the CLI.
7. As an operator whose restore was interrupted, I want the TUI to detect the checkpoint and offer a resume action, so that I can continue from where it stopped with one keystroke.
8. As an operator whose restore completed with deferred objects, I want the TUI to display the Exit Code 3 interpretation and offer a replay action, so that I understand what "deferred" means and can drain the queue without constructing the replay command myself.
9. As an operator whose restore completed with verify findings, I want the TUI to display the Exit Code 4 breakdown (FK violations vs Corrupt), so that I know whether I need to inspect data manually or if it's a false positive.
10. As an operator who enabled Fast Mode, I want the TUI to show which session variables are active (autocommit=0, unique_checks=0, foreign_key_checks=0), so that I understand the speed/safety tradeoff in effect.
11. As an operator learning the tool, I want inline help screens that explain domain terms (Statement Boundary, Checkpoint, Batch, Deferred Object), so that I can build my mental model without leaving the TUI or reading external docs.
12. As an operator discovering available actions, I want a keyboard shortcut reference screen, so that I can learn navigation and commands through exploration rather than trial and error.
13. As an operator switching between CLI and TUI, I want both modes to operate on the same Data Directory and state, so that a profile created in the CLI appears in the TUI and vice versa.
14. As an operator running the TUI on a remote server over SSH, I want the interface to render correctly in a standard 80x24 terminal, so that I can use it in constrained environments without requiring a large window.
15. As an operator managing profiles with vaulted passwords, I want the TUI to show a "vaulted" indicator (not the password itself) in the profile list, so that I can distinguish profiles that require the Master Passphrase from those that don't.
16. As an operator unlocking the vault in the TUI, I want a no-echo password prompt (matching CLI behavior), so that my Master Passphrase isn't visible on screen or in terminal history.
17. As an operator renaming a profile with a sealed password, I want the TUI to detect that re-seal is required and prompt for the Master Passphrase, so that the AAD binding stays valid after the rename.
18. As an operator deleting a profile, I want a confirmation prompt (y/N) in the TUI, so that I don't accidentally delete a profile with a vaulted password I can't recreate.
19. As an operator with a stale dump (file size or identity changed), I want the TUI to show the resume guard failure clearly and offer --force-resume as an explicit override, so that I understand why resume was blocked and can make an informed decision.
20. As an operator whose restore hit a timeout, I want the TUI to display Exit Code 1 (Fatal Error) with the timeout context, so that I know whether to adjust --write-timeout or diagnose server load.
21. As an operator whose restore was interrupted via Ctrl-C, I want the TUI to display Exit Code 130 (SIGINT) and offer resume, so that I understand it was a deliberate stop, not a failure.
22. As an operator running verify, I want the TUI to stream CHECK TABLE results as they arrive, so that I can see progress on long-running integrity scans rather than waiting in silence.
23. As an operator with multiple concurrent restores (different dumps), I want the TUI to list all active/resumable restores from the Checkpoint Store, so that I can track parallel operations without manually inspecting SQLite.
24. As an operator on a headless server without a TTY, I want the TUI to detect the non-interactive environment and exit with a clear error, so that I don't hang a cron job by launching `tui` in an automation script.
25. As an operator exploring TUI features, I want a demo mode or sample data option, so that I can practice navigation and workflows without needing a real dump file or database connection.
26. As an operator who prefers dark mode terminals, I want the TUI color scheme to be readable on dark backgrounds, so that text doesn't blend into my terminal theme.
27. As an operator who prefers light mode terminals, I want the TUI color scheme to be readable on light backgrounds, so that highlights and status indicators are visible.
28. As an operator using the TUI for the first time, I want a brief onboarding overlay (dismissable) explaining the main screens and how to navigate, so that I can orient myself without reading external documentation.
29. As an operator managing profiles, I want the TUI to validate connection settings before saving (ping the host:port), so that I catch typos in host/port immediately rather than discovering them at restore time.
30. As an operator setting a password via TUI, I want the interface to ask twice (entry + confirmation) matching CLI `set-password` behavior, so that I don't seal a mistyped password into the vault.
31. As an operator with a long profile list, I want the TUI to support search/filter by name or host, so that I can quickly find the profile I need without scrolling through dozens of entries.
32. As an operator launching a restore, I want the TUI to show the resolved credential source (e.g., "using vaulted password" or "using --password-file override"), so that I have transparency about which secret is active.
33. As an operator whose restore completed cleanly, I want the TUI to display Exit Code 0 with a summary (statements executed, time elapsed), so that I can verify success at a glance.
34. As an operator viewing restore history, I want the TUI to show dump identity (hash prefix) alongside path, so that I can distinguish between different versions of the same filename.
35. As an operator with portable Data Directory, I want the TUI to display the active Data Directory path on startup, so that I know which state the tool is operating on when I override with --data-dir.

## Implementation Decisions

### TUI Framework Selection

Use **Bubble Tea** (`github.com/charmbracelet/bubbletea`) as the TUI framework. Bubble Tea is the Go standard for full-screen terminal UIs (used by `gh`, `glow`, `soft-serve`), provides an Elm-architecture model (predictable state updates), and integrates with Lip Gloss for declarative styling and Bubbles for pre-built components (tables, text inputs, spinners). The framework is actively maintained, well-documented, and has strong community adoption. Alternative `tview` is more batteries-included but less composable and harder to test in isolation.

### Screen Architecture

Implement a **screen-based navigation model** with a central router that manages the active screen stack. Each screen is a discrete Bubble Tea component implementing the `tea.Model` interface (Init, Update, View methods). The router handles screen transitions, back/forward navigation, and keyboard shortcut dispatch. Screens include: Home (restore history list), Profile List, Profile Editor (CRUD), Restore Launcher (wizard), Live Progress Monitor, Post-Restore Report, Help/Shortcuts Reference, Glossary. The router maintains a navigation stack so "back" (Esc) returns to the previous screen, and "quit" (q) exits the application gracefully.

### Shared Domain Logic

TUI and CLI share the same domain packages: checkpoint store access, profile CRUD, credential resolution, restore engine orchestration. The TUI does not duplicate business logic; it delegates to shared packages and renders their results. This ensures parity between modes: a profile created in TUI appears in CLI `profile list`, a checkpoint written by CLI is visible in TUI restore history, and both modes respect the same Credential Source precedence and AAD binding constraints. The TUI is a presentation layer over existing domain modules.

### State Management

TUI state is ephemeral (in-memory Bubble Tea model); all durable state lives in the Data Directory (Checkpoint Store, Connection Profiles, Vault settings row). On startup, the TUI reads from Data Directory to populate initial screen state. User actions trigger writes to Data Directory (save profile → INSERT/UPDATE profile row, start restore → begin checkpoint tracking). The TUI does not maintain a separate state file; it is a stateless view over the persistent Data Directory, ensuring consistency with CLI operations.

### Credential Handling in TUI

When the TUI needs to unlock the vault (e.g., `set-password`, `rename` with sealed password), it uses `term.ReadPassword` for no-echo input, matching CLI behavior. The Master Passphrase is never stored in the TUI model; it is held in memory only for the duration of the seal/unseal operation and discarded immediately. Profile List displays a "vaulted" indicator (derived from `sealed_password IS NOT NULL`) but never shows password length, prefix, or plaintext. The TUI respects Credential Source precedence (ADR-0017) when launching restores: if user provides `--password-file` override in launcher wizard, it outranks the vaulted password.

### Live Progress Rendering

During an active restore, the TUI spawns the restore operation in a goroutine and subscribes to progress updates via a channel. The restore engine emits progress events (statements done, byte offset, batch commits, deferred object count) to the channel; the TUI Update method consumes these events and re-renders the progress screen. This ensures the TUI remains responsive to keyboard input (pause, cancel) while streaming updates. On restore completion, the goroutine signals completion with final Exit Code, and the TUI transitions to the Post-Restore Report screen.

### Resume Detection and Guards

On the Home screen, the TUI queries the Checkpoint Store for resumable restores (rows with byte_offset > 0). For each, it computes current dump_identity (size + head hash) and compares to stored identity. If mismatch detected, the TUI displays a warning indicator and disables one-click resume; selecting the entry shows the mismatch details and offers `--force-resume` as an explicit action. This mirrors ADR-0005 behavior: refuse-by-default on identity change, manual override available.

### Exit Code Interpretation

Post-Restore Report screen decodes Exit Code into human-readable explanations. Exit Code 0 → "Restore completed successfully" with summary. Exit Code 1 → "Restore failed (Fatal Error)" with error message, resume available. Exit Code 3 → "Restore completed with deferred objects" with count, offers "Replay Now" action that invokes the replay command. Exit Code 4 → "Restore completed with verify findings" with breakdown (FK violations vs Corrupt), flags Corrupt as potential false positive per ADR-0020. Exit Code 130/143 → "Restore interrupted (user stop)" with resume available. The TUI teaches operators what each code means without requiring them to memorize the exit-code contract.

### Keyboard Shortcuts

Global shortcuts: `q` quit, `Esc` back, `?` help, `g` glossary, `h` home, `p` profiles, `r` new restore. Context shortcuts vary by screen: on Home, `Enter` resume selected restore, `d` delete checkpoint; on Profile List, `Enter` edit, `n` new, `Del` delete; on Live Progress, `Ctrl-C` interrupt (triggers Exit Code 130). All shortcuts are listed on the Help screen and displayed as footer hints on each screen (e.g., "q: quit | ?: help | Esc: back"). Vim-style navigation (`j`/`k` for list movement) is supported alongside arrow keys.

### Accessibility and Terminal Compatibility

The TUI is designed for standard VT100-compatible terminals with minimum 80x24 size. It uses ANSI escape codes for colors/formatting (via Lip Gloss) and degrades gracefully on terminals without color support (plain text fallback). The TUI detects if stdin is not a TTY (`term.IsTerminal` check on launch) and exits with a clear error, preventing hangs in headless/cron environments. This matches ADR-0017 interactive prompt behavior: TUI is interactive-only, CLI handles unattended use.

### Onboarding and Discoverability

On first launch (detected by absence of a `~/.mariadb-restorer-tui-welcomed` marker file), display a brief overlay explaining the screen layout, navigation model, and how to access help. The overlay is dismissable (`Enter` or `Esc`) and never shown again once dismissed. The Help screen includes a domain glossary (Statement Boundary, Checkpoint, Batch, Resume Batch, Deferred Object, Credential Vault, Master Passphrase, Fast Mode, Verify) with definitions sourced from CONTEXT.md, so operators can learn the tool's vocabulary without leaving the interface.

### Profile Validation

When saving a profile in the TUI Editor, offer optional connection validation: attempt TCP dial to `host:port` with 5-second timeout. If successful, show green checkmark; if failed, show red X with error (e.g., "connection refused", "no route to host"). Validation is opt-in (button press, not automatic on field change) to avoid network noise on every keystroke. Validation does NOT test credentials (that would require the password, which may not be set yet or may come from runtime sources); it only checks reachability. This catches common typos (wrong port, wrong host) before the operator commits the profile.

### Color Scheme

Use a **universal color palette** that works on both dark and light terminal backgrounds: green for success, red for errors, yellow for warnings, blue for info, white/default for primary text, dim/gray for secondary text. Avoid bright-on-bright or dark-on-dark combinations. Test on common terminal themes (Solarized Dark, Solarized Light, Monokai, default xterm) during development. Provide a `--no-color` flag (or respect `NO_COLOR` env var) to disable all ANSI styling for accessibility or terminals with poor color support.

### Restore Launcher Wizard

The launcher presents a multi-step flow: (1) Select dump file (file picker or text input with tab completion), (2) Select connection profile (list of saved profiles + "Create New" option), (3) Confirm settings (shows resolved credential source, Fast Mode status, timeout values), (4) Optional --verify toggle, (5) Launch. Each step is a separate sub-screen; "Next" advances, "Back" returns, "Cancel" exits to Home. On launch, the TUI transitions to Live Progress Monitor and begins streaming updates. This guided flow eliminates the need to remember flag order or profile syntax.

### Concurrent Restore Tracking

The Checkpoint Store can hold multiple rows (one per dump_identity). The Home screen lists all rows with status indicators: "In Progress" (byte_offset < dump_size_bytes, updated_at recent), "Resumable" (byte_offset < dump_size_bytes, updated_at stale), "Completed" (row deleted on success, so never appears). The TUI does not enforce single-restore-at-a-time; operators can launch multiple restores from separate TUI instances or CLI invocations. Each restore's progress is tracked independently by dump_identity. This matches ADR-0005 design: Checkpoint Store is multi-tenancy safe.

### Demo Mode

Provide a `--demo` flag that launches the TUI with synthetic data: pre-populated profiles (staging, prod, dev), fake restore history (one completed, one resumable, one failed), simulated live restore (progress bar that ticks up without actual database connection). Demo mode writes to an in-memory state (does not touch Data Directory) and exits cleanly on quit. This allows operators to explore the interface, practice navigation, and understand workflows without needing a real dump file, database, or vault passphrase. Demo mode displays a banner at the top: "DEMO MODE — no actual operations".

### Error Handling and Feedback

All TUI operations that can fail (save profile, start restore, delete checkpoint) display inline error messages on failure: red text below the action button or in a status bar at the bottom of the screen. Errors persist until the user takes another action or dismisses explicitly. For fatal errors that prevent TUI operation (Data Directory not writable, Checkpoint Store corrupted), display a full-screen error message with instructions (e.g., "Run with --data-dir to specify a writable location") and exit on keypress. Non-fatal warnings (e.g., resume guard mismatch) are displayed as yellow indicators that don't block interaction.

### Testing Strategy

Each TUI screen is a discrete Bubble Tea model with Init/Update/View methods, making it testable in isolation. Unit tests instantiate a screen model, send synthetic `tea.Msg` events (keypresses, progress updates), and assert on resulting model state and View output. Integration tests launch the full TUI router with a test Data Directory, simulate user input sequences (navigate to Profile List, create profile, return to Home), and verify state changes in the Checkpoint Store and profile tables. Use `tea.WithInput` / `tea.WithOutput` to inject test I/O streams. No UI automation framework needed; Bubble Tea's architecture is test-friendly by design.

## Testing Decisions

### What Makes a Good Test

Tests should verify **external behavior** (state transitions, rendered output, side effects on Data Directory), not implementation details (internal model fields, helper function signatures). For example, test that pressing `Enter` on a profile list entry transitions to the Profile Editor screen and that the editor displays the profile's current host/port, but don't test the internal `selectedIndex` field directly. Tests should be **resilient to refactoring**: if we swap Bubble Tea for `tview`, the tests should still pass (or require only I/O adapter changes, not full rewrites).

### Modules to Test

All eight TUI modules require tests per user request:

1. **TUI Framework Layer**: Test that event loop starts, handles keypresses, renders without panic. Mock terminal I/O.
2. **Screen Manager/Router**: Test screen transitions (Home → Profile List → Editor → back to List → Home), navigation stack integrity, global shortcut dispatch.
3. **Restore History Viewer**: Test that Checkpoint Store rows appear in list, status indicators are correct (resumable vs failed), selecting an entry shows details, delete action removes row.
4. **Connection Profile Manager UI**: Test CRUD flows (create profile, edit host/port, rename triggers re-seal prompt, delete shows confirmation). Verify profile table changes persist to Data Directory.
5. **Interactive Restore Launcher**: Test wizard flow (select dump, select profile, confirm, launch), credential source display, --verify toggle. Mock restore engine to avoid actual DB connection.
6. **Live Progress Monitor**: Test that progress events update display (statements_done increments, byte_offset advances), completion transitions to Report screen, Ctrl-C triggers interrupt.
7. **Post-Restore Report Viewer**: Test Exit Code decoding (0/1/3/4/130/143 each render correct message), Deferred Object count display, replay action invocation.
8. **Help/Discovery System**: Test that Help screen lists all shortcuts, Glossary screen displays domain terms, navigation from any screen to Help and back works.

### Prior Art for Tests

The existing CLI (not yet implemented) will have tests for shared domain packages (checkpoint store, profile CRUD, credential resolution). TUI tests should **reuse these domain test utilities** (e.g., `NewTestDataDir()` that creates a temporary SQLite store) to ensure consistency. For Bubble Tea-specific testing patterns, refer to upstream examples in `github.com/charmbracelet/bubbletea/examples` and community projects like `glow` (Markdown renderer) or `soft-serve` (Git server TUI). These demonstrate how to test models in isolation, mock tea.Msg events, and assert on View output using string matching or snapshot testing.

### Test Coverage Goals

Aim for **high coverage on state transitions and user-facing logic** (router, screen workflows, credential resolution display, Exit Code decoding), **moderate coverage on rendering** (verify key UI elements appear, but don't assert exact layout or whitespace), and **integration tests for critical paths** (end-to-end: launch TUI, create profile, start restore, see progress, quit). Rendering tests should be **snapshot-light**: use them to catch regressions (e.g., profile list suddenly empty when it should have entries), not to enforce pixel-perfect layout.

## Out of Scope

- **Real-time collaboration**: Multiple operators editing profiles or launching restores simultaneously from different TUI instances is supported (Checkpoint Store is multi-tenancy safe), but the TUI does not auto-refresh to show changes made by other instances. Operator must navigate away and back to refresh.
- **Remote TUI over HTTP/WebSockets**: TUI is terminal-only (SSH-friendly). No web-based UI or browser frontend.
- **Mouse support**: Navigation is keyboard-only. No mouse clicks, scrolling, or drag-and-drop.
- **Custom themes or color schemes**: One universal palette that works on dark and light backgrounds. No user-configurable themes.
- **Undo/redo for profile edits**: Changes are committed immediately on save. No undo stack.
- **In-TUI log viewer**: TUI shows progress and final reports, but detailed logs (if any) must be viewed outside the TUI (e.g., `tail -f` in another terminal).
- **Plugin system or extensibility**: TUI screens and workflows are hard-coded. No user-defined screens or actions.
- **AI/ML-based anomaly detection**: Verify phase detects FK violations via CHECK TABLE EXTENDED (ADR-0020), but the TUI does not analyze patterns or suggest optimizations based on restore history.

## Further Notes

### Alignment with ADRs

This PRD respects all 30 ADRs across restore-engine and credential-vault contexts. Key alignments:

- **ADR-0005** (Checkpoint Store): TUI reads from modernc SQLite, displays dump_identity and byte_offset, enforces resume guards on identity mismatch.
- **ADR-0007** (Portable storage): TUI operates on Data Directory (defaults to executable's own dir), displays active Data Directory path on startup.
- **ADR-0011/0013** (Deferred Object): TUI decodes Exit Code 3, displays deferred count, offers replay action.
- **ADR-0014** (Argon2id KDF): TUI uses term.ReadPassword for Master Passphrase, never stores it, respects 64 MiB memory cost (no additional memory pressure from TUI itself).
- **ADR-0015** (AEAD AAD): TUI detects profile rename, prompts for passphrase to re-seal under new AAD.
- **ADR-0016** (Inline sealed_password): TUI shows "vaulted" indicator from `sealed_password IS NOT NULL`, never displays ciphertext or plaintext.
- **ADR-0017** (Credential Source precedence): TUI displays resolved source (explicit > vault > env > prompt) in launcher confirmation step.
- **ADR-0020** (Verify): TUI streams CHECK TABLE results, decodes FK violations vs Corrupt, flags latter as potential false positive.
- **ADR-0028** (Timeout): TUI displays timeout values in launcher confirmation, decodes timeout Exit Code 1 with context.
- **ADR-0029** (sql_log_bin): TUI does not query or display sql_log_bin state (dump-owned, passed through verbatim).
- **ADR-0030** (Error classification): TUI decodes all Exit Codes per classification (0/1/3/4/130/143), teaches operator what each means.

### Relationship to Existing PRD

The existing `PRD_Enterprise_MariaDB_Restorer.md` (July 11, 2026) covers core restore functionality (streaming, checkpointing, view stubbing, Fast Mode, crash resilience). This PRD is **additive**: it defines the TUI mode that wraps and presents that core functionality. Both PRDs target the same audience (system integrators, DBAs, backend developers) and share the same technical stack (Go, SQLite, MariaDB driver). The TUI PRD assumes the core restore engine will be implemented per the original PRD; the TUI is the second interface (alongside CLI) over that engine.

### Implementation Sequencing

TUI mode depends on CLI mode being functional (shared domain packages must exist). Suggested implementation order: (1) Core restore engine + CLI (per original PRD), (2) TUI Framework Layer + Screen Router (infrastructure), (3) Home screen (restore history), (4) Profile Manager, (5) Restore Launcher, (6) Live Progress, (7) Post-Restore Report, (8) Help/Glossary, (9) Demo mode. This allows incremental delivery: early TUI builds can coexist with CLI, and operators can start using Home + Profile Manager even before Live Progress is complete.

### Performance Considerations

TUI rendering (via Lip Gloss) is lightweight; the bottleneck is Data Directory I/O (SQLite reads for profile lists, checkpoint queries). Expect sub-100ms response for screen transitions on modern hardware. Live Progress updates stream via channel at ~1 event per batch commit (ADR-0001: batch size is tunable, default TBD); even at 1000 commits/sec the TUI can render at 60 FPS without stutter. The TUI does not hold the full dump file in memory (streaming is engine's responsibility); TUI memory footprint is <5 MB beyond Go runtime overhead.

### Deployment and Distribution

TUI and CLI compile into the same binary. Launch via `mariadb-restorer tui` (TUI mode) or `mariadb-restorer restore <file>` (CLI mode). No separate `mariadb-restorer-tui` binary; one tool, two interfaces. The binary remains a single static artifact (pure Go, no cgo via modernc SQLite) suitable for copy-to-any-host deployment. TUI mode requires a TTY; CLI mode works headless. Operators can use both modes interchangeably on the same Data Directory without conflicts.
