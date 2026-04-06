# Penfold CLI

## CoBuild

This project uses [CoBuild](https://github.com/otherjamesbrown/cobuild) for pipeline automation — designs flow through structured phases (design → decompose → implement → review → done) with quality gates.

**Read `.cobuild/AGENTS.md` for full pipeline instructions, commands, and task completion protocol.**

## Building

```bash
go build -o penf .
go test ./...
go vet ./...
```

## Configuration

| System | Server | Config |
|--------|--------|--------|
| Penfold | dev02.brown.chat:50051 | ~/.penf/config.yaml |
| Context Palace | dev02.brown.chat:5432 | ~/.cobuild/config.yaml |

## Deploying Backend Services

Services are in the penfold repo. `penf deploy` delegates to `penfold/scripts/deploy.sh`:

```bash
penf deploy gateway     # Build + deploy gateway to dev02 (systemd)
penf deploy worker      # Build + deploy worker to dev01 (launchd)
penf deploy ai          # Build + deploy AI coordinator to dev02 (systemd)
penf deploy mcp         # Build + deploy MCP server to dev02 (systemd)
penf deploy all         # Deploy all in dependency order
penf deploy --status    # Check all services
```

The canonical deploy logic lives in `penfold/scripts/` — never duplicate it in Go code.

## Troubleshooting

```bash
penf status / penf health / penf update
cobuild wi list --type task
```
