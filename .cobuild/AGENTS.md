<!-- BEGIN COBUILD INTEGRATION v:1 hash:de2662a7 -->
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

### Work Items

| Command | Purpose |
|---------|--------|
| `cobuild wi show <id>` | Read a design, task, or bug |
| `cobuild wi list --type <type>` | List work items |
| `cobuild wi links <id>` | See relationships |
| `cobuild wi status <id> <status>` | Update status |
| `cobuild wi append <id> --body "..."` | Append content |
| `cobuild wi create --type <type> --title "..."` | Create work item |

## Bug Workflow

Bugs go through investigation before implementation:

1. `cobuild init <bug-id>` — enters investigate phase
2. Investigation agent analyses root cause (read-only, does NOT fix)
3. `cobuild investigate <bug-id> --verdict pass` — advances to implement
4. Fix task created as child with implementation spec
5. `cobuild dispatch <fix-task-id>` → review → merge → done

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
