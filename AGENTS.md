<!-- BEGIN COBUILD INTEGRATION v:1 hash:66aef32d -->
# CoBuild Pipeline Instructions

This project uses CoBuild for pipeline automation. If you are an agent working on a task dispatched by CoBuild, follow these instructions.

## Project

- **Name:** penfold
- **Prefix:** pf-
- **Workflows:**
  - bug: investigate → implement → review → done
  - task: implement → review → done
  - design: design → decompose → implement → review → done

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

### Work Items

| Command | Purpose |
|---------|--------|
| `cobuild wi show <id>` | Read a design, task, or bug |
| `cobuild wi list --type <type>` | List work items |
| `cobuild wi links <id>` | See relationships |
| `cobuild wi status <id> <status>` | Update status |
| `cobuild wi append <id> --body "..."` | Append content |
| `cobuild wi create --type <type> --title "..."` | Create work item |

## How to Run Pipelines (Manual Mode)

**There is no automatic poller.** You must step through each phase manually using `cobuild dispatch` and `cobuild wait`. Do not assume work will happen automatically.

Every phase transition requires:
1. **Dispatch** — `cobuild dispatch <id>` spawns an agent in tmux for the current phase
2. **Wait** — `cobuild wait <id>` blocks until the agent completes
3. **Next** — dispatch the next phase or the next wave of tasks

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
