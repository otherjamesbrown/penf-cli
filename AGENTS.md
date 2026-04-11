<!-- BEGIN COBUILD INTEGRATION v:1 hash:69878b25 -->
# CoBuild Pipeline Instructions

This project uses CoBuild for pipeline automation. If you are a dispatched CoBuild agent working on a task, follow these instructions.

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
| `cobuild gate <id> <gate> --verdict pass\|fail` | Record a gate verdict |
| `cobuild investigate <id> --verdict pass` | Record bug investigation verdict |
| `cobuild dispatch <task-id>` | Dispatch a task to a dispatched CoBuild agent |
| `cobuild dispatch-wave <design-id>` | Dispatch all ready tasks |
| `cobuild wait <id> [id...]` | Wait for tasks to complete |
| `cobuild complete <task-id>` | **Run as your LAST action** after implementing |
| `cobuild merge <task-id>` | Merge an approved PR |
| `cobuild merge-design <design-id>` | Smart merge all PRs (conflict detection) |
| `cobuild deploy <design-id>` | Deploy affected services |
| `cobuild retro <design-id>` | Run retrospective |
| `cobuild status` | Show all active pipelines |
| `cobuild audit <id>` | View gate history |
| `cobuild scan` | Refresh project anatomy (file index for agents) |
| `cobuild explain` | Show pipeline in human-readable form |

### Work Items

| Command | Purpose |
|---------|--------|
| `cobuild wi show <id>` | Read a design, task, or bug |
| `cobuild wi list --type <type>` | List work items |
| `cobuild wi links <id>` | See relationships |
| `cobuild wi status <id> <status>` | Update status |
| `cobuild wi append <id> --body "..."` | Append content |
| `cobuild wi create --type <type> --title "..."` | Create work item |

## How to Run Pipelines

There are two ways to advance each phase:

### Option A: You already did the work (interactive session)

If you've already reviewed the design, decomposed into tasks, or done investigation
in the current session with the developer, just record the gate verdict:

```bash
cobuild review <id> --verdict pass --readiness 5 --body "<findings>"   # record design review
cobuild decompose <id> --verdict pass --body "<task summary>"          # record decomposition
cobuild investigate <id> --verdict pass --body "<root cause>"           # record investigation
```

The gate command records the verdict and advances the phase. No dispatch needed.

### Option B: Delegate to a dispatched CoBuild agent

If you want a dispatched CoBuild agent to handle a phase in its own context:

```bash
cobuild dispatch <id>   # spawns dispatched CoBuild agent in tmux for the current phase
cobuild wait <id>       # blocks until the dispatched CoBuild agent completes
```

`cobuild dispatch` is phase-aware — it generates the right prompt automatically.
Use this for implementation (dispatched CoBuild agents write code) and when you want a clean context.

### Which to use?

| Situation | Use |
|-----------|-----|
| You just reviewed the design with the developer | Option A — record the gate |
| You need a dispatched CoBuild agent to write code | Option B — dispatch |
| You decomposed tasks in conversation | Option A — record the gate |
| You want investigation in a clean context | Option B — dispatch |
| Phase needs multiple file reads/edits | Option B — saves your context |

### Design Workflow

```bash
cobuild init <design-id>                     # enters design phase
cobuild dispatch <design-id>                 # spawns readiness review agent
cobuild wait <design-id>                     # wait for review to complete
# Agent records gate → advances to decompose
cobuild dispatch <design-id>                 # spawns decomposition agent
cobuild wait <design-id>                     # wait for decomposition
# Agent creates tasks, records gate → advances to implement
cobuild dispatch-wave <design-id>            # dispatch ready tasks
cobuild wait <task-1> <task-2> ...           # wait for implementation
# Repeat dispatch-wave/wait for each wave
cobuild merge-design <design-id> --dry-run   # preview merge plan
cobuild merge-design <design-id>             # merge all PRs
cobuild deploy <design-id>                   # deploy affected services
cobuild retro <design-id>                    # run retrospective
```

### Bug Workflow

**Default (most bugs):** single `fix` session — agent investigates and fixes together.

**Escalation path:** if the bug is complex, label it `needs-investigation` first — it routes to a read-only investigation phase that produces a fix spec before any code is changed.

#### When to add `needs-investigation`

Apply the label if **any** of these are true:

1. Root cause unknown (symptom visible, mechanism unclear)
2. Bug spans multiple services, modules, or repos
3. Data or security implications — need blast radius assessment before fixing
4. This area has broken before, or the fix might have unintended side effects
5. Reproduces inconsistently — needs investigation to find the trigger
6. Fix shape is non-obvious (can't describe it in 1-2 sentences)
7. Investigation produces options that require a stakeholder decision

If none apply → omit the label. The fix agent will investigate as it fixes.

#### Default bug flow

```bash
cobuild init <bug-id>                        # enters fix phase
cobuild dispatch <bug-id>                    # spawns fix agent (investigate + implement)
cobuild wait <bug-id>                        # wait for fix
cobuild merge <bug-id>                       # merge the fix PR
cobuild deploy <bug-id>                      # deploy if needed
```

#### Complex bug flow (needs-investigation label)

```bash
cobuild wi label add <bug-id> needs-investigation
cobuild init <bug-id>                        # enters investigate phase
cobuild dispatch <bug-id>                    # spawns investigation agent (READ-ONLY)
cobuild wait <bug-id>                        # wait for investigation
# Agent records investigation report + gate → creates fix task → advances to implement
cobuild dispatch <fix-task-id>               # spawns dispatched CoBuild agent (implement skill)
cobuild wait <fix-task-id>                   # wait for fix
cobuild merge <fix-task-id>                  # merge the fix PR
cobuild deploy <bug-id>                      # deploy if needed
```

### Task Workflow

```bash
cobuild init <task-id>                       # enters implement phase
cobuild dispatch <task-id>                   # spawns dispatched CoBuild agent (implement skill)
cobuild wait <task-id>                       # wait for completion
cobuild merge <task-id>                      # merge PR
```

**Key:** `cobuild dispatch` is phase-aware. It reads the current pipeline phase and generates the right prompt automatically — investigation prompt for bugs, readiness review for designs, implementation for tasks. You don't need different commands for different phases.

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
