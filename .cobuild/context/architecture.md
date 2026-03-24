# Architecture — penf-cli

## Overview

`penf-cli` is the CLI client for the Penfold system — James's institutional memory platform. It communicates with the Penfold backend (gateway + worker + AI coordinator) over gRPC with mTLS.

## Structure

```
penf-cli/
    main.go                 # Entry point
    cmd/                    # Cobra command files (one per command group)
        deploy.go           # Service deployment commands
        health_preflight.go # Health and preflight checks
        ingest.go           # Content ingestion (email, meeting)
        pipeline*.go        # Pipeline inspection, config, routing
        search.go           # Content search
        entity.go           # Entity management (people, products, projects)
        ...                 # ~100+ command files
        templates/          # Prompt/output templates
    client/                 # gRPC client wrappers
        client.go           # Base client (connection, TLS)
        *_client.go         # Per-domain clients (pipeline, content, search, etc.)
    api/proto/              # Protobuf definitions (shared with penfold backend)
    pkg/                    # Shared libraries
        buildinfo/          # Version/build metadata
        contentid/          # Content ID generation
        db/                 # Direct DB access (read-only queries)
        enrichment/         # Content enrichment logic
        errors/             # Error types
        ingest/             # Ingestion logic (EML parsing, batch processing)
        logging/            # Structured logging (zerolog)
        mentions/           # Mention resolution
        products/           # Product matching
    services/search/        # Local search service
    contextpalace/          # Context Palace integration
    config/                 # Configuration loading
    credentials/            # Credential management
```

## Build & Test

```bash
go build -o penf .          # Build CLI binary
go test ./...               # Run all tests
go vet ./...                # Static analysis
```

The built binary is typically installed to `~/bin/penf`.

## Key Patterns

- **Cobra commands** — each command group is a file in `cmd/` with a root command and subcommands
- **gRPC client** — all backend communication goes through `client/` wrappers over mTLS to dev02.brown.chat:50051
- **Proto definitions** — shared between penf-cli and penfold repos, in `api/proto/`
- **Direct DB reads** — some commands read directly from Postgres (via `pkg/db/`) for performance, but all writes go through gRPC
- **Zerolog** — structured JSON logging throughout

## External Dependencies

- **Penfold Gateway** — dev02.brown.chat:50051 (gRPC, mTLS)
- **PostgreSQL** — dev02.brown.chat:5432 (read-only direct access for some queries)
- **Context Palace** — dev02.brown.chat:5432 (same DB server, different schema)
- **Redis** — used for caching
- **Ollama** — dev01:11434 (local LLM inference)

## Deployment

penf-cli is a local binary — `go build -o penf . && cp penf ~/bin/penf`.

The penfold backend services are deployed via `penf deploy <service>`:
- gateway → dev02 (systemd)
- worker → dev01 (launchd)
- ai coordinator → dev02 (systemd)

Failed health checks trigger automatic rollback.

## Multi-Repo Relationship

| Repo | Contains |
|------|----------|
| penf-cli | CLI binary, proto definitions, cobra commands |
| penfold | Backend server, gateway, worker, Temporal, DB migrations |
