# Issue tracker: bd (beads)

Issues and PRDs for this repo are tracked with **bd (beads)** — a local-first
issue tracker driven by the `bd` CLI. There is no GitHub/GitLab issue tracker
for this project.

Run `bd prime` for full workflow context before working with issues.

## Core commands

```bash
bd ready                 # Find available work (no blockers)
bd list --status=open    # All open issues
bd show <id>             # View issue details + dependencies
bd search <query>        # Find issues by keyword
bd create --title="..." --description="..." --type=task|bug|feature --priority=2
bd update <id> --claim   # Claim work
bd update <id> --status=in_progress
bd close <id>            # Complete work
bd dep add <issue> <depends-on>   # Add a dependency (issue depends on depends-on)
```

## When a skill says "publish to the issue tracker" / "create an issue"

Create a beads issue with `bd create`. Put the summary in `--title` and the body
(why it exists + what needs to be done) in `--description`. Set `--type`
(`task`/`bug`/`feature`) and `--priority` (0–4; 0=critical, 2=medium, 4=backlog).
Use `--design`, `--notes`, and `--acceptance` for the corresponding sections.
Wire up ordering between issues with `bd dep add`.

When a skill (e.g. `to-issues`) produces several issues at once, create each with
its own `bd create`, then link dependencies so `bd ready` surfaces them in order.

## When a skill says "fetch the relevant ticket"

Run `bd show <id>` (use `bd search <query>` first if you need to locate it). The
user will normally pass the issue id directly.

## Triage state

Triage roles map to beads labels — see `triage-labels.md` for the strings. Apply
them when triaging with `bd update <id> --labels=<role>` (or the label mechanism
your beads config uses).

## Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown
  TODO lists. `bd` is the single source of truth for work in this repo.
- Do NOT use `bd edit` — it opens `$EDITOR` (vim/nano) and blocks agents. Use
  `bd update` with inline flags instead.
- Use `bd remember "insight"` for persistent knowledge across sessions; search
  with `bd memories <keyword>`.
