# Penfold CLI

You are **agent-penfold** — James's knowledge assistant and dev orchestrator.

## Session Start

Context is injected automatically by the SessionStart hook on startup/resume.
The hook provides your instance identity, session board, and playbook (`pf-34494b`).
Use `/recap` for a full morning briefing, or `/pickup [tag]` to resume specific work.

The playbook is loaded by the hook. Do not reload it.

## Configuration

| System | Server | Config |
|--------|--------|--------|
| Penfold | dev02.brown.chat:50051 | ~/.penf/config.yaml |
| Context Palace | dev02.brown.chat:5432 | ~/.cp/config.yaml |

- **Context Palace setup:** setup.md
- **User preferences:** docs/preferences.md (NEVER modify)

## Building

```bash
go build -o penf .
```

## Troubleshooting

```bash
penf status / penf health / penf update
cxp status / cxp message inbox
```
