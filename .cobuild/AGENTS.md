# CoBuild Pipeline Instructions

This project uses CoBuild for pipeline automation. If you are an agent working on a task dispatched by CoBuild, follow these instructions.

## Project

- **Name:** penfold
- **Prefix:** pf-
- **Connector:** context-palace
- **Workflows:**
  - design: design → decompose → implement → review → done
  - bug: investigate → implement → review → done
  - task: implement → review → done

## Multi-Repo

Designs may span two repos: **penf-cli** (CLI binary) and **penfold** (backend server).

Tasks are tagged with their target repo during decomposition. Your worktree is already set to the correct repo for your task.

## Commands

### Pipeline

| Command | When to use |
|---------|------------|
| `cobuild init <id>` | Submit a design/bug/task to the pipeline (auto-detects type and start phase) |
| `cobuild show <id>` | See pipeline state |
| `cobuild gate <id> <gate> --verdict pass\|fail --body "..."` | Record a gate verdict |
| `cobuild investigate <id> --verdict pass --body "..."` | Record bug investigation verdict |
| `cobuild dispatch <task-id>` | Dispatch a task to an implementing agent |
| `cobuild dispatch-wave <design-id>` | Dispatch all ready tasks for a design |
| `cobuild wait <id> [id...]` | Wait for tasks to reach target status |
| `cobuild complete <task-id>` | **Run as your LAST action** after implementing a task |
| `cobuild merge <task-id>` | Merge an approved PR and close the task |
| `cobuild merge-design <design-id>` | Smart merge all PRs for a design (conflict detection) |
| `cobuild deploy <design-id>` | Deploy affected services after merge |
| `cobuild retro <design-id>` | Run pipeline retrospective |
| `cobuild status` | Show all active pipelines |
| `cobuild audit <id>` | View gate history |

### Work Items

| Command | Purpose |
|---------|---------|
| `cobuild wi show <id>` | Read a design, task, or bug |
| `cobuild wi list --type <type>` | List work items |
| `cobuild wi links <id>` | See relationships (child-of, blocked-by) |
| `cobuild wi status <id> <status>` | Update work item status |
| `cobuild wi append <id> --body "..."` | Append content to a work item |
| `cobuild wi create --type <type> --title "..."` | Create a new work item |

## Bug Workflow

Bugs go through investigation before implementation:

1. `cobuild init <bug-id>` — enters investigate phase
2. Investigation agent analyses root cause (read-only, does NOT fix)
3. Investigation produces: root cause, affected files, fragility assessment, fix spec
4. `cobuild investigate <bug-id> --verdict pass` — advances to implement
5. Fix task created as child of bug with implementation spec
6. `cobuild dispatch <fix-task-id>` — agent implements the fix
7. Review → merge → done

## Task Completion Protocol

When you have completed your implementation:

1. Run tests: `go test ./...`
2. Run vet: `go vet ./...`
3. Build: `go build -o penf .`
4. **Run `cobuild complete <task-id>`**

This commits remaining changes, pushes your branch, creates a PR, appends evidence to the work item, and marks the task as needs-review.

**Do this as your LAST action. Do not skip it.**

## What CoBuild manages vs what you do directly

Be explicit when reporting status to the developer. State clearly whether an action is:
- **A CoBuild pipeline action** — "CoBuild will handle this: `cobuild merge-design <id>`"
- **A direct action you'll take** — "I'll run `penf deploy gateway` now"
- **A human action needed** — "You need to approve this PR before CoBuild can merge"

## Skills

Pipeline skills are in `skills/` organized by phase:

| Directory | Skills | Purpose |
|-----------|--------|---------|
| `design/` | gate-readiness-review, implementability | Design evaluation |
| `decompose/` | decompose-design | Break designs into tasks |
| `investigate/` | bug-investigation | Root cause analysis for bugs |
| `implement/` | dispatch-task, stall-check | Task dispatch and monitoring |
| `review/` | gate-review-pr, gate-process-review, merge-and-verify | Code review |
| `done/` | gate-retrospective | Post-delivery retrospective |
| `shared/` | playbook, create-design, design-review | Cross-phase reference |

## Context

Architecture and project context is in `.cobuild/context/`:
- `architecture.md` — codebase structure, build/test, key patterns, dependencies

These are assembled into your CLAUDE.md at dispatch time via context layers.
