# Project Instructions for AI Agents

Instructions and context for AI coding agents on **mariadb-restorer** (Go).

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Issue Tracking — bd (beads)

Use `bd` for ALL task tracking. Run `bd prime` for full workflow. Create the issue BEFORE writing code.

```bash
bd ready                # available work    bd update <id> --claim   # claim
bd show <id>            # details            bd close <id>            # complete
```

- Do NOT use TodoWrite, TaskCreate, or markdown TODO lists.
- Use `bd remember` for persistent knowledge — NOT MEMORY.md files.

## Session Completion

Work is NOT complete until `git push` succeeds. Before saying "done":

1. File issues for remaining work.
2. Run quality gates (tests, linters, build) if code changed.
3. Close finished issues, update in-progress ones.
4. **Push (mandatory):** `git pull --rebase && bd dolt push && git push` → confirm `git status` shows up to date.
5. Clear stashes, prune remote branches.

NEVER stop before pushing or say "ready to push when you are" — YOU push. If push fails, retry until it succeeds.
<!-- END BEADS INTEGRATION -->

## Shell Conventions

Always use non-interactive flags — aliases like `-i` hang the agent on y/n prompts.

- Files: `cp -f`, `mv -f`, `rm -f`, `rm -rf`, `cp -rf` (never the bare form).
- `scp`/`ssh`: `-o BatchMode=yes` · `apt-get`: `-y` · `brew`: `HOMEBREW_NO_AUTO_UPDATE=1`.

## Agent Skills

| Concern | Rule | Reference |
|---------|------|-----------|
| Issue tracker | Issues & PRDs tracked via `bd` (beads) | `docs/agents/issue-tracker.md` |
| Triage labels | Default roles: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix` | `docs/agents/triage-labels.md` |
| Domain docs | Multi-context: `CONTEXT-MAP.md` → per-context `CONTEXT.md` | `docs/agents/domain.md` |

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **mariadb-restorer** (25 symbols, 23 relationships, 0 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/mariadb-restorer/context` | Codebase overview, check index freshness |
| `gitnexus://repo/mariadb-restorer/clusters` | All functional areas |
| `gitnexus://repo/mariadb-restorer/processes` | All execution flows |
| `gitnexus://repo/mariadb-restorer/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
