# Penfold CLI

Command-line interface for [Penfold](https://github.com/otherjamesbrown/penfold), a personal information system that aggregates and correlates communication channels into queryable institutional memory.

## Context

`penf-cli` is the command-line client for **Penfold**, one of four operator instruments James is building to support a senior engineering role at a PE-backed company. See `~/SYSTEMS.md` for cross-project data flows and ownership boundaries.

**Operator instruments:**
- **Penfold** -- email, Teams, meeting transcripts → unified institutional memory (this repo's CLI talks to it)
- **[Mycroft](https://github.com/otherjamesbrown/mycroft)** -- GitLab/GitHub metrics + code scanning for velocity, quality, secure coding
- **Moneypenny** -- consolidates disparate facts into a single queryable source
- **M-Intel** -- topic research and hypothesis generation with evidence grading

**Infrastructure:**
- **[Context Palace](https://github.com/otherjamesbrown/context-palace)** -- work tracking + KB for AI agents (`cxp` CLI)
- **[CoBuild](https://github.com/otherjamesbrown/cobuild)** -- design → decompose → implement → review → deploy automation

`penf-cli` shares the `pf-` Context Palace project namespace with the [Penfold backend](https://github.com/otherjamesbrown/penfold) — they're the same product, split into client and server repos.

## What is Penfold?

Penfold ingests content from email, meeting transcripts, documents, and URLs, then runs it through an AI pipeline that extracts entities, assertions, and relationships. The result is a searchable knowledge base that can answer questions like "what's happening with Project X?" or "what did Rob commit to last week?"

### Key capabilities

- **Content ingestion** -- Gmail sync, .eml import, meeting transcripts, URLs, documents
- **AI pipeline** -- Triage, entity extraction, semantic analysis, embedding generation
- **Entity resolution** -- Automatically links mentions of the same person/project across sources
- **Knowledge queries** -- Full-text and semantic search, project briefings, AI-powered Q&A
- **Conversation threading** -- Groups related emails into conversations with rolling summaries
- **Watch instructions & alerts** -- Natural-language rules evaluated against incoming content
- **Digests** -- Automated daily/weekly project summaries
- **Quality evaluation** -- LLM-as-judge scoring via Langfuse for extraction, triage, and summary quality
- **Multi-tenancy** -- Isolated content and configuration per tenant

## Installation

```bash
go build -o penf .
```

## Quick start

```bash
# Initialize configuration
penf init

# Check system health
penf health -e

# Ingest email files
penf ingest email ./emails/ --source "my-archive"

# Search the knowledge base
penf search "quarterly planning"

# AI-powered query
penf ai query "What are the open risks for Project Alpha?"

# Project briefing
penf briefing "Project Alpha"
```

## Architecture

```
penf (CLI)  ──gRPC──>  Gateway  ──>  Worker (Temporal)
                          │              │
                          ├── AI Coordinator (LLM inference)
                          ├── Gmail Connector (email sync)
                          └── PostgreSQL + pgvector
```

- **Gateway** -- API server, routing, auth (dev02, systemd)
- **Worker** -- Temporal workflow executor, pipeline processing (dev01, launchd)
- **AI Coordinator** -- LLM inference for triage, extraction, summarization (dev02, systemd)
- **Gmail Connector** -- OAuth2 Gmail sync with incremental updates (dev02, systemd)
- **MCP Server** -- Model Context Protocol interface for AI assistants (dev02, systemd)

## CLI overview

| Category | Commands | Purpose |
|----------|----------|---------|
| **Querying** | `search`, `ai`, `briefing`, `assertions`, `thread`, `conversation`, `digest`, `alert`, `instruction` | Find and explore knowledge |
| **Ingestion** | `ingest`, `meeting` | Bring content into the system |
| **Pipeline** | `pipeline`, `workflow`, `reprocess`, `content`, `classify` | Monitor and manage processing |
| **Entities** | `entity`, `relationship`, `glossary`, `mention`, `context`, `trust`, `seniority` | Manage people, orgs, projects, vocabulary |
| **Review & Triage** | `review`, `process`, `audit`, `escalations`, `watch` | Validate and curate AI output |
| **Operations** | `health`, `status`, `deploy`, `logs`, `trace`, `debug`, `db`, `model`, `quality`, `schedule`, `ledger` | System administration |
| **Setup** | `init`, `auth`, `cert`, `tenant`, `config`, `update` | Configuration and auth |

## AI assistant integration

The CLI is designed for use by AI assistants. All commands support `--output json` for structured data, and command help includes examples and flag descriptions for discovery.

```bash
penf <command> --help       # Discover subcommands and flags
penf health -e              # System health with pipeline statistics
penf pipeline status        # Processing pipeline overview
penf debug info             # Full diagnostic information
```

## Development

```bash
go build -o penf .
go test ./...
go vet ./...
```

## Deployment

Backend services live in the [penfold](https://github.com/otherjamesbrown/penfold) repo. The CLI delegates deployment:

```bash
penf deploy gateway     # Build + deploy gateway
penf deploy worker      # Build + deploy worker
penf deploy ai          # Build + deploy AI coordinator
penf deploy mcp         # Build + deploy MCP server
penf deploy all         # Deploy all in dependency order
penf deploy --status    # Check all services
```

## License

Private.
