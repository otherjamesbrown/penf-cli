# CoBuild Pipeline Instructions

This project uses CoBuild for pipeline automation. If you are an agent working on a task dispatched by CoBuild, follow these instructions.

## Project

- **Name:** penfold
- **Prefix:** pf-
- **Connector:** context-palace
- **Workflows:**
  - design: design → decompose → implement → review → done
  - bug/task: implement → review → done

## Multi-Repo

Designs may span two repos: **penf-cli** (CLI binary) and **penfold** (backend server).

Tasks are tagged with their target repo during decomposition. Your worktree is already set to the correct repo for your task.

## Commands

### Pipeline

| Command | When to use |
|---------|------------|
| `cobuild show <id>` | See pipeline state for a design |
| `cobuild complete <task-id>` | **Run as your LAST action** after implementing a task |
| `cobuild gate <id> <gate> --verdict pass\|fail --body "..."` | Record a gate verdict |
| `cobuild audit <id>` | View gate history for a design |

### Work Items

| Command | Purpose |
|---------|---------|
| `cobuild wi show <id>` | Read a design, task, or bug |
| `cobuild wi list --type <type>` | List work items |
| `cobuild wi links <id>` | See relationships (child-of, blocked-by) |
| `cobuild wi status <id> <status>` | Update work item status |
| `cobuild wi append <id> --body "..."` | Append content to a work item |
| `cobuild wi create --type <type> --title "..."` | Create a new work item |

## Task Completion Protocol

When you have completed your implementation:

1. Run tests: `go test ./...`
2. Run vet: `go vet ./...`
3. Build: `go build -o penf .`
4. **Run `cobuild complete <task-id>`**

This commits remaining changes, pushes your branch, creates a PR, appends evidence to the work item, and marks the task as needs-review.

**Do this as your LAST action. Do not skip it.**

## Skills

Pipeline skills are in `skills/` organized by phase:

| Directory | Skills | Purpose |
|-----------|--------|---------|
| `design/` | gate-readiness-review, implementability | Design evaluation |
| `implement/` | dispatch-task, stall-check | Task dispatch and monitoring |
| `review/` | gate-review-pr, gate-process-review, merge-and-verify | Code review |
| `done/` | gate-retrospective | Post-delivery retrospective |
| `shared/` | playbook, create-design | Cross-phase reference |

## Context

Architecture and project context is in `.cobuild/context/`:
- `architecture.md` — codebase structure, build/test, key patterns, dependencies

These are assembled into your CLAUDE.md at dispatch time via context layers.
