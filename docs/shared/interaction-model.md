# Penfold Interaction Model

> **Last updated:** 2026-01-26

## Overview

Penfold is designed for **AI-assisted** interaction, not direct human CLI use.

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           INTERACTION FLOW                                    │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│    ┌─────────┐      Natural       ┌─────────────┐      CLI        ┌───────┐ │
│    │  User   │ ◄──────────────── │ Claude Code │ ◄────────────── │  penf │ │
│    └─────────┘      Language      └─────────────┘     Commands    └───────┘ │
│         │                               │                            │       │
│         │                               │                            │       │
│         │  "Find discussions           │  penf search "API"         │       │
│         │   about the API"             │  --format json             │       │
│         │                               │                            │       │
│         │                               ▼                            ▼       │
│         │                         ┌─────────────┐              ┌─────────┐  │
│         │                         │   Process   │              │ Gateway │  │
│         │                         │   Results   │◄─────────────│  gRPC   │  │
│         │                         └─────────────┘              └─────────┘  │
│         │                               │                            │       │
│         │  "Found 5 discussions,       │                            │       │
│         │   most recent was..."        │                            ▼       │
│         │◄──────────────────────────────                       ┌─────────┐  │
│         │                                                      │   DB    │  │
│                                                                └─────────┘  │
└──────────────────────────────────────────────────────────────────────────────┘
```

## Key Insight

**The user never runs CLI commands directly.** Claude Code:
1. Interprets user intent
2. Chooses appropriate commands
3. Processes results intelligently
4. Presents findings in natural language

This is why:
- `--help` text is designed for AI comprehension
- Commands support `--format json` for structured output
- Batch processing commands exist for intelligent bulk operations
- Documentation explains "when to use" not just "how to use"

## Three Audiences

### 1. End Users (via Claude Code)

Users interact in natural language:
- "What did we discuss about the database migration?"
- "Who should I talk to about Kubernetes?"
- "Review the new acronyms from this week's meetings"

They never see CLI commands or raw output.

### 2. Claude Code (AI Assistant)

Claude Code uses the `penf` CLI to:
- Search content (`penf search "query" --format json`)
- Process review queues (`penf process acronyms context`)
- Manage entities (`penf product add`, `penf glossary add`)
- Check system health (`penf health`, `penf status`)

Claude reads documentation in `~/.penf/docs/` to understand:
- When to use each command
- How to interpret results
- What workflows exist

### 3. Developers (Building Penfold)

Developers work on:
- CLI commands (`cmd/penf/`)
- Gateway services (`services/gateway/`)
- Worker processes (`services/worker/`)
- Documentation (`context/`)

## Client Documentation Flow

When a user runs `penf init` or `penf update`:

```
┌─────────────────────────────────────────────────────────────────┐
│                    DOCUMENTATION DEPLOYMENT                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Repository                          User's Machine             │
│  ───────────                         ──────────────             │
│                                                                 │
│  context/shared/        ─────────►   ~/.penf/docs/shared/       │
│    vision.md                           vision.md                │
│    entities.md                         entities.md              │
│    use-cases.md                        use-cases.md             │
│    interaction-model.md                interaction-model.md     │
│                                                                 │
│  context/client/        ─────────►   ~/.penf/docs/              │
│    index.md                            index.md                 │
│    concepts/                           concepts/                │
│    workflows/                          workflows/               │
│    processes.md                        processes.md             │
│                                                                 │
│  context/client/        ─────────►   ~/.penf/                   │
│    preferences.md        (once)        preferences.md           │
│                                       (never overwritten)       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## What Claude Code Reads

On startup, Claude Code should read:
1. `~/.penf/docs/index.md` - System overview and navigation
2. `~/.penf/docs/shared/` - Core concepts (vision, entities, use-cases)
3. `~/.penf/preferences.md` - User's personal preferences

When performing specific tasks:
- Review acronyms → `workflows/acronym-review.md`
- Onboarding → `workflows/onboarding.md`
- Search → `concepts/glossary.md` (for query expansion)

## Batch Processing Pattern

Instead of one-at-a-time operations, Claude Code uses batch processing:

```
1. GET CONTEXT
   penf process acronyms context --format json
   → Returns all pending items + existing glossary + guidance

2. ANALYZE INTELLIGENTLY
   Claude categorizes:
   - Standard tech terms → auto-resolve
   - Already in glossary → dismiss
   - Ambiguous → ask user

3. PRESENT SUMMARY
   "Found 15 acronym questions:
    - 8 standard tech terms (auto-resolving)
    - 3 already in glossary (dismissing)
    - 4 need your input: [list]"

4. EXECUTE BATCH
   penf process acronyms batch-resolve '{"resolutions":[...],"dismissals":[...]}'
```

This is more efficient and provides better UX than processing items one at a time.

## Preferences System

Users can customize Claude's behavior via `~/.penf/preferences.md`:

```yaml
# Example preferences
auto_resolve:
  auto_resolve_standard_tech: true  # Auto-resolve MVP, API, etc.

domain:
  company: "Akamai"
  industry: "Cloud Infrastructure"
  common_acronyms:
    LKE: "Linode Kubernetes Engine"

review_style:
  mode: batch        # vs interactive
  ask_threshold: 0.7 # Ask user below this confidence
```

Claude Code can **update these preferences** based on user feedback, personalizing the experience over time.

## See Also

- [vision.md](vision.md) - What Penfold is and why
- [../client/index.md](../client/index.md) - Full documentation index
- [../client/workflows/](../client/workflows/) - Task-specific workflows
