<!-- BEGIN COBUILD INTEGRATION v:1 hash:df639b38 -->
# CoBuild Pipeline Instructions

This project uses CoBuild for pipeline automation. If you are a dispatched CoBuild agent working on a task, follow these instructions.

## For orchestrators — the ONLY thing you need to know

Every CoBuild command prints a `Next step:` line telling you exactly what to run next. **Follow it mechanically.** Do not reason about which phase needs which command — that's what the CLI is for.

The loop:

1. `cobuild init <id>` (if the pipeline doesn't exist yet)
2. `cobuild dispatch <id>` — spawns a dispatched CoBuild agent for the current phase
3. Read the `Next step:` line printed by the command — run that
4. Repeat step 3 until phase = done

If you are ever unsure what to run next, run `cobuild next <id>` — it prints the single concrete command for the current state.

Do NOT execute phase work yourself (decompose, review, investigate, etc.) just because you could. **Every phase has a skill and a dispatched CoBuild agent runs it.** Your only job as orchestrator is to type the commands and follow the output.

## Terminology

Two roles show up throughout CoBuild's docs, skills, and commit messages. Use these terms consistently:

- **orchestrator agent** — whoever invokes `cobuild dispatch`, `cobuild run`, or any other pipeline CLI. Stays lightweight and delegates work. Can be an interactive Claude/Codex session, the `cobuild poller` daemon, a cron job, or a human at a shell prompt.
- **dispatched CoBuild agent** — the fresh Claude Code or Codex process CoBuild spawns in a tmux window inside a git worktree to execute a phase's skill. Does all the real reading, editing, and committing. Exits when the skill is done.

If you see "M", "parent session", "calling agent", "fresh session", or "implementing agent" in older docs, they all map onto one of these two terms — prefer the canonical terms above.

## Project

- **Name:** penfold
- **Prefix:** pf-
- **Workflows:**
  - bug: fix → review → done
  - bug-complex: investigate → implement → review → done
  - design: design → decompose → implement → review → done
  - task: implement → review → done

## Commands

### Pipeline

| Command | When to use |
|---------|------------|
| `cobuild init <id>` | Submit a design/bug/task to the pipeline |
| `cobuild dispatch <task-id>` | Dispatch to a dispatched CoBuild agent (works for every phase) |
| **`cobuild next <id>`** | **Print the single next command to run for a pipeline — use when confused** |
| `cobuild dispatch-wave <design-id>` | Dispatch all ready tasks in a wave |
| `cobuild process-review <task-id>` | Process Gemini review → merge or re-dispatch |
| `cobuild merge <task-id>` | Merge an approved PR manually |
| `cobuild merge-design <design-id>` | Smart merge all PRs (conflict detection) |
| `cobuild deploy <design-id>` | Deploy affected services |
| `cobuild retro <design-id>` | Run retrospective |
| `cobuild status` | Show all active pipelines |
| `cobuild audit <id>` | View gate history and timeline |
| `cobuild show <id>` | Compact current state for one pipeline |
| `cobuild scan` | Refresh project anatomy (file index for agents) |
| `cobuild wait <id> [id...]` | Block until tasks reach target status (2h max) |
| `cobuild complete <task-id>` | **Run as your LAST action** if you ARE the dispatched agent |

**Manual gate recording (Advanced — see below):** `cobuild review / decompose / investigate / gate` — use only when the gate work happened outside a dispatched agent session.

### Work Items

| Command | Purpose |
|---------|--------|
| `cobuild wi show <id>` | Read a design, task, or bug |
| `cobuild wi list --type <type>` | List work items |
| `cobuild wi links <id>` | See relationships |
| `cobuild wi status <id> <status>` | Update status |
| `cobuild wi append <id> --body "..."` | Append content |
| `cobuild wi create --type <type> --title "..."` | Create work item |

## How to Run a Pipeline

**Default and only flow: dispatch.** Every phase has a skill, and `cobuild dispatch` spawns a dispatched CoBuild agent that reads the skill and executes it. You never do the work yourself.

```bash
cobuild init <id>          # if the pipeline doesn't exist yet
cobuild dispatch <id>      # start — spawns a dispatched CoBuild agent for the current phase
# → follow the `Next step:` line it prints
# → repeat until phase = done
```

**`cobuild dispatch` is phase-aware.** It reads the current pipeline phase and generates the right prompt automatically — readiness review for design, decomposition for decompose, investigation for investigate, implementation for tasks, and so on. One command advances the entire pipeline.

**If you are ever confused:** run `cobuild next <id>`. It prints the single concrete command for the current state. Do not try to infer it from the workflow table or your memory of which phase comes next — let the CLI tell you.

**Do not own any phase yourself.** Even if you (the orchestrator agent) *could* do the work inline — read the design, break it into tasks, record the gate — **don't**. That pattern exists to be used in genuinely exceptional cases (see Advanced below), not as a default shortcut. The dispatched agent model keeps your context lean and produces a clean audit trail.

### Bug workflow note

**Most bugs** use the `fix` workflow — a single dispatched CoBuild agent investigates and fixes together in one session. **Complex bugs** (root cause unknown, multi-repo, data/security implications, non-obvious fix shape) should be labeled `needs-investigation` before `cobuild init` — this routes them to the `bug-complex` workflow with a read-only investigation phase first.

```bash
cobuild wi label add <bug-id> needs-investigation   # only if complex
cobuild init <bug-id>
cobuild dispatch <bug-id>    # follow the Next step: output from here
```

## Advanced: recording a gate without dispatching

**This is an exceptional path, not the default.** Use it only when the gate work genuinely happened outside a dispatched CoBuild agent session — for example, a design that was reviewed live with the developer in a meeting, or an investigation that was done by a human. For anything the pipeline can do, prefer `cobuild dispatch`.

```bash
cobuild review <id> --verdict pass --readiness 5 --body "<findings>"   # record design review
cobuild decompose <id> --verdict pass --body "<task summary>"          # record decomposition
cobuild investigate <id> --verdict pass --body "<root cause>"           # record investigation
```

These commands record the gate and advance the phase without dispatching. **If you find yourself reaching for them because you weren't sure whether decompose had a skill, stop and run `cobuild dispatch <id>` instead — every phase has one.**

## Task Completion Protocol

When you have completed your implementation:

1. Run tests: `go test ./... && go vet ./...`
2. Build: `go build -o penf .`
3. **Run `cobuild complete <task-id>`**

The Stop hook will run `cobuild complete` automatically when you finish.
If it fails, run it manually as your last action.

## Orchestrator Protocol

If you are the orchestrator agent (dispatching tasks, not executing them yourself),
**follow through the full lifecycle. Do not stop after dispatch.**

After dispatching tasks:

1. **Monitor** — use `cobuild audit <id>` or `cobuild status` for instant checks (do NOT use `cobuild wait` as a background task — it's a 2-hour blocking command)
2. **Process reviews** — run `cobuild process-review <task-id>` for each needs-review task. This automatically: waits for Gemini review, classifies findings, merges clean PRs, or re-dispatches agents for fixes. If it says "Waiting" — Gemini hasn't reviewed yet, retry after a few minutes.
3. **Report** — tell the user what shipped, not "want me to review?"
4. **Deploy** — do NOT deploy automatically. Run `cobuild deploy <id> --dry-run` to show which services would be affected, then **ask the user** for approval. On approval, run `cobuild deploy <id>` (triggers deploy commands from pipeline config with smoke tests and auto-rollback). Deploy touches production and is always a human decision.

Only pause for user input if there is an actual blocker: merge conflict, critical Gemini finding you can't resolve, a design decision, or deploy approval.

## What CoBuild manages vs what you do directly

Be explicit when reporting status. State clearly whether an action is:
- **A CoBuild pipeline action** — "CoBuild will handle this: `cobuild merge-design <id>`"
- **A direct action you'll take** — "I'll run the deploy command now"
- **A human action needed** — "You need to approve this PR"

## Skills

| Directory | Skills | Purpose |
|-----------|--------|---------|
| `design/` | gate-readiness-review, implementability | Design evaluation |
| `decompose/` | decompose-design | Break designs into tasks |
| `investigate/` | bug-investigation | Root cause analysis for needs-investigation bugs |
| `implement/` | dispatch-task, stall-check | Task dispatch and monitoring |
| `review/` | gate-process-review, gate-review-pr, merge-and-verify | Code review |
| `done/` | gate-retrospective | Post-delivery retrospective |
| `shared/` | create-design, playbook | Cross-phase reference |

<!-- END COBUILD INTEGRATION -->
