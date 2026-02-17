# Penfold System Documentation

> **You should have already read [assistant-rules.md](assistant-rules.md)** - that's your entry point and identity.

Use this documentation to understand Penfold's concepts and workflows.

## Quick Navigation

**Your Identity:**
- How you operate → [assistant-rules.md](assistant-rules.md)
- Your preferences for James → [preferences.md](preferences.md)

**Session Memory:**
- Daily logs → `memory/YYYY-MM-DD.md` (create as needed)
- **At session start:** Read recent memory files to restore context

**Dev Communication (Agent Mail):**
- Your agent: **RedWolf** | Dev agent: **RusticDesert**
- **Project key:** `/Users/james/github/otherjamesbrown/penfold` (always use this, regardless of machine)
- Check inbox at session start: `fetch_inbox(project_key="...", agent_name="RedWolf")`
- Send bugs/feedback: `send_message(...)` via MCP
- See [assistant-rules.md](assistant-rules.md#agent-mail---dev-communication) for details

**North Star (shared knowledge):**
- Vision and principles → [shared/vision.md](shared/vision.md)
- Core entities → [shared/entities.md](shared/entities.md)
- Use cases → [shared/use-cases.md](shared/use-cases.md)
- Interaction model → [shared/interaction-model.md](shared/interaction-model.md)

**Understand the system:**
- Entity types and resolution → [concepts/entities.md](concepts/entities.md)
- Glossary and acronyms → [concepts/glossary.md](concepts/glossary.md)
- People resolution → [concepts/people.md](concepts/people.md)
- Mention resolution → [concepts/mentions.md](concepts/mentions.md)
- Products and hierarchy → [concepts/products.md](concepts/products.md)

**Workflows (how to do things):**
- Seed entities before import → [workflows/init-entities.md](workflows/init-entities.md)
- Review after import → [workflows/onboarding.md](workflows/onboarding.md)
- Process unknown acronyms → [workflows/acronym-review.md](workflows/acronym-review.md)
- Resolve person mentions → [workflows/mention-review.md](workflows/mention-review.md)

## System Overview

Penfold aggregates content from email, meetings, and documents into a
searchable knowledge base with automatic entity resolution.

### Entity Types

| Type | Description | CLI Commands |
|------|-------------|--------------|
| People | Individuals with emails, aliases | `penf relationship entity` |
| Products | Business products, hierarchy | `penf product` |
| Projects | Initiatives, timelines | (coming soon) |
| Teams | Groups of people | (coming soon) |
| Glossary | Acronyms, terminology | `penf glossary` |

### Processing Flow

```
┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐
│ INGEST  │ ──▶ │ EXTRACT │ ──▶ │ RESOLVE │ ──▶ │ REVIEW  │ ──▶ │ SEARCH  │
└─────────┘     └─────────┘     └─────────┘     └─────────┘     └─────────┘
Content enters   Entities &      Match to        Queue unknowns  Content is
the system       mentions ID'd   known entities  for human       queryable
```

1. **Ingest** - Content (email, meetings) enters via `penf ingest`
2. **Extract** - System identifies people, acronyms, mentions
3. **Resolve** - Mentions are matched to known entities in the database
4. **Review** - Unknown entities are queued for human review via Claude
5. **Search** - Content becomes searchable with entity context

### First-Time Setup Flow

```
penf init              # Configure CLI connection
penf init entities     # Seed known people, products, projects, glossary
penf ingest email ...  # Import content
penf process onboarding context  # Claude guides you through new entities
```

## Key Concepts

### Multi-Context Terms

The same acronym can mean different things in different contexts.
See [concepts/glossary.md](concepts/glossary.md) for details.

**Example:** "VIP" might mean:
- "Very Important Person" in sales context
- "Virtual IP Address" in networking context

### Entity Resolution

When content mentions "JB said...", Penfold tries to resolve "JB" to a
known person. See [concepts/mentions.md](concepts/mentions.md) for details.

Resolution uses:
- Email addresses (exact match)
- Name aliases (configured matches)
- LLM disambiguation (when multiple candidates exist)

### Auto-Created vs Confirmed Entities

Entities have two creation modes:
- **Auto-created**: Discovered from content, needs review
- **Confirmed**: Explicitly added or reviewed by user

Auto-created entities have `needs_review: true` and should be reviewed
via the onboarding workflow.

## CLI Quick Reference

```bash
# Status & Health
penf status                    # Check gateway connection
penf health                    # System health overview

# Search
penf search "query"            # Search content
penf search "query" -o json    # JSON output for Claude

# Glossary
penf glossary list             # List all terms
penf glossary add TERM "Expansion"  # Add term
penf glossary show TERM        # Show term details

# Entity Management
penf relationship entity list  # List entities
penf product list              # List products
penf product add "Name"        # Add product

# AI Workflows
penf process acronyms context  # Get acronym review context
penf process mentions context  # Get mention resolution context
penf process onboarding context  # Get post-import review context

# Review Queue
penf review questions list     # List pending questions
penf review questions resolve ID "answer"  # Answer question
```

## Quick Reference for AI Agents

This section provides patterns optimized for Claude Code working with the penf CLI.

### Batch Processing Pattern

Use `penf process <workflow> context` to get full context for intelligent batch processing:

```bash
# Get all pending items with context for decision-making
penf process acronyms context --output json
penf process mentions context --output json
penf process onboarding context --output json
```

Then submit batch resolutions:

```bash
# Resolve multiple items at once
penf process acronyms batch-resolve '{"resolutions":[...],"dismissals":[...]}'
```

### JSON Output for Processing

Always use `--output json` or `-o json` when you need to process results:

```bash
# Search with JSON output
penf search "API gateway" -o json

# List with JSON output
penf review questions list -o json
penf glossary list -o json
penf product list -o json
```

### Common JSON Response Patterns

**Search results:**
```json
{
  "results": [
    {"id": 123, "content": "...", "score": 0.95, "source": "email", "date": "..."}
  ],
  "total": 42
}
```

**Review questions:**
```json
{
  "questions": [
    {"id": 1, "type": "acronym", "term": "LKE", "context": "...", "candidates": [...]}
  ]
}
```

**Process context (acronyms):**
```json
{
  "pending": [...],
  "glossary": [...],
  "stats": {"total": 15, "auto_resolvable": 8},
  "guidance": "..."
}
```

### Decision Guide: Which Command to Use

| Task | Command | When to Use |
|------|---------|-------------|
| Find information | `penf search "query" -o json` | User asks about a topic |
| Review new acronyms | `penf process acronyms context` | After content import |
| Resolve person mentions | `penf process mentions context` | Ambiguous name references |
| Post-import review | `penf process onboarding context` | After `penf ingest` |
| Answer pending questions | `penf review questions list -o json` | Check review queue |
| Add known term | `penf glossary add TERM "Expansion"` | User provides definition |
| Find product info | `penf product query "who owns X"` | Natural language product queries |

### Workflow: Processing a Review Queue

```bash
# 1. Get context for intelligent processing
penf process acronyms context --output json > context.json

# 2. Analyze and categorize (Claude does this)
#    - Standard tech terms → auto-resolve
#    - Already in glossary → dismiss
#    - Ambiguous → ask user

# 3. Submit batch resolution
penf process acronyms batch-resolve '{
  "resolutions": [
    {"question_id": 1, "expansion": "Kubernetes", "context": "container orchestration"}
  ],
  "dismissals": [
    {"question_id": 2, "reason": "duplicate of existing term"}
  ]
}'
```

### Error Handling

When commands fail, check:

```bash
# Gateway connectivity
penf status

# Detailed health
penf health

# With verbose output
penf <command> --verbose
```

Common errors:
- "connection refused" → Gateway not running, check `penf status`
- "not found" → Entity doesn't exist, verify with `list` command first
- "unauthorized" → Auth expired, run `penf auth login`

## For More Information

Each concept and workflow document provides detailed explanations,
examples, and CLI command references. Follow the links in Quick Navigation above.
