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

## Deploying Backend Services

Services are managed by native process managers — launchd on dev01 (macOS), systemd on dev02 (Linux).

```bash
penf deploy gateway     # Build + deploy gateway to dev02 (systemd)
penf deploy worker      # Build + deploy worker to dev01 (launchd)
penf deploy ai          # Build + deploy AI coordinator to dev02 (systemd)
penf deploy all         # Deploy all in dependency order
penf deploy --status    # Check all services
```

Deploy scripts in the penfold repo also work:
```bash
./scripts/deploy.sh worker|gateway|ai|all|status
```

Failed health checks trigger automatic rollback to the previous binary.

## Troubleshooting

```bash
penf status / penf health / penf update
cxp status / cxp message inbox
```
