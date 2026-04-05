<!-- BEGIN COBUILD INTEGRATION v:1 hash:69527112 -->
# CoBuild Pipeline Instructions

This project uses CoBuild for pipeline automation. If you are an agent working on a task dispatched by CoBuild, follow these instructions.

## Project

- **Name:** penfold
- **Prefix:** pf-
- **Workflows:**
  - design: design → decompose → implement → review → done
  - bug: investigate → implement → review → done
  - task: implement → review → done

## Commands

### Pipeline

| Command | When to use |
|---------|------------|
| `cobuild init <id>` | Submit a design/bug/task to the pipeline |
| `cobuild gate <id> <gate> --verdict pass\|fail` | Record a gate verdict |
| `cobuild investigate <id> --verdict pass` | Record bug investigation verdict |
| `cobuild dispatch <task-id>` | Dispatch a task to an implementing agent |
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

### Option B: Delegate to a separate agent (dispatch)

If you want a fresh agent to handle a phase in its own context:

```bash
cobuild dispatch <id>   # spawns agent in tmux for the current phase
cobuild wait <id>       # blocks until the agent completes
```

`cobuild dispatch` is phase-aware — it generates the right prompt automatically.
Use this for implementation (agents write code) and when you want a clean context.

### Which to use?

| Situation | Use |
|-----------|-----|
| You just reviewed the design with the developer | Option A — record the gate |
| You need an agent to write code | Option B — dispatch |
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

```bash
cobuild init <bug-id>                        # enters investigate phase
cobuild dispatch <bug-id>                    # spawns investigation agent (READ-ONLY)
cobuild wait <bug-id>                        # wait for investigation
# Agent records investigation report + gate → creates fix task → advances to implement
cobuild dispatch <fix-task-id>               # spawns implementing agent
cobuild wait <fix-task-id>                   # wait for fix
cobuild merge <fix-task-id>                  # merge the fix PR
cobuild deploy <bug-id>                      # deploy if needed
```

### Task Workflow

```bash
cobuild init <task-id>                       # enters implement phase
cobuild dispatch <task-id>                   # spawns implementing agent
cobuild wait <task-id>                       # wait for completion
cobuild merge <task-id>                      # merge PR
```

**Key:** `cobuild dispatch` is phase-aware. It reads the current pipeline phase and generates the right prompt automatically — investigation prompt for bugs, readiness review for designs, implementation for tasks. You don't need different commands for different phases.

## Task Completion Protocol

When you have completed your implementation:

1. Run tests: `go test ./... && go vet ./...`
2. Build: `go build -o penf .`
3. **Run `cobuild complete <task-id>`**

**Do this as your LAST action. Do not skip it.**

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
| `investigate/` | bug-investigation | Root cause analysis for bugs |
| `implement/` | dispatch-task, stall-check | Task dispatch and monitoring |
| `review/` | gate-process-review, gate-review-pr, merge-and-verify | Code review |
| `done/` | gate-retrospective | Post-delivery retrospective |
| `shared/` | create-design, playbook | Cross-phase reference |

<!-- END COBUILD INTEGRATION -->
