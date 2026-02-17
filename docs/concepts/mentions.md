# Mentions

Mentions are references to entities found in content text. Penfold extracts
and resolves mentions to connect content to known entities.

## What Gets Extracted

### Person Mentions

Names or references to people:
- "John said we should..."
- "Per Sarah's email..."
- "JB and team reviewed..."

### Product/Project Mentions

References to products or projects:
- "The DBaaS migration is..."
- "MTC timeline update..."
- "LKE cluster issues..."

### Acronym Mentions

Unknown acronyms that need definition:
- "The TER meeting..." (if TER not in glossary)
- "PLD review scheduled..." (unknown acronym)

## Mention Resolution Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                     Content Ingested                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  EXTRACTION: Find potential mentions in text                    │
│  - NER (Named Entity Recognition)                               │
│  - Pattern matching (email, @mentions)                          │
│  - Acronym detection (uppercase sequences)                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  CANDIDATE GENERATION: Find possible matches                    │
│  - Exact email match                                            │
│  - Alias lookup                                                 │
│  - Fuzzy name matching                                          │
│  - Pattern database lookup                                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  DISAMBIGUATION: Pick the right match                           │
│  - Single high-confidence candidate → auto-resolve              │
│  - Multiple candidates → LLM disambiguation                     │
│  - No candidates → queue for human review                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│  OUTCOME:                                                       │
│  - RESOLVED: Linked to entity                                   │
│  - PENDING: Queued for human review                             │
│  - DISMISSED: Not an entity mention                             │
└─────────────────────────────────────────────────────────────────┘
```

## Resolution Status

| Status | Meaning |
|--------|---------|
| `pending` | Needs resolution |
| `auto_resolved` | System resolved with high confidence |
| `llm_resolved` | LLM disambiguated between candidates |
| `user_resolved` | Human resolved via review |
| `dismissed` | Not an entity mention |

## Patterns: Learning From Resolutions

When a mention is resolved, Penfold can create a **pattern** for future use:

```
Mention: "JB" → Resolved to: James Brown (person_id: 5)
Pattern created: "JB" → James Brown
```

Next time "JB" appears, it auto-resolves without LLM call.

### Pattern Types

- **Exact**: "JB" matches only "JB"
- **Case-insensitive**: "jb", "Jb", "JB" all match
- **Contextual**: "JB" in MTC context → James Brown; "JB" in Sales → John Baker

## Mention Context

Each mention captures context for resolution:

```json
{
  "mentioned_text": "JB",
  "context_snippet": "JB mentioned that the timeline needs adjustment",
  "source_id": 123,
  "position": {"start": 45, "end": 47},
  "surrounding_entities": ["MTC", "timeline"]
}
```

Context helps disambiguate when multiple candidates exist.

## Candidate Scoring

Candidates are scored based on:

| Factor | Weight | Description |
|--------|--------|-------------|
| Alias match | High | Direct alias match |
| Name similarity | Medium | Fuzzy match to canonical name |
| Recency | Medium | Recently mentioned people score higher |
| Context match | Medium | Related to current project/topic |
| Historical pattern | High | Previously resolved same way |

## CLI Commands

### View Pending Mentions

```bash
# Get context for Claude processing (recommended)
penf process mentions context --output json

# List pending person-type questions
penf review questions list --type person
```

### Resolve Mentions

```bash
# Single resolution
penf review questions resolve <id> --person-id 5

# Batch resolution
penf process mentions batch-resolve '{
  "resolutions": [
    {"mention_id": 123, "entity_id": 5, "entity_type": "ENTITY_TYPE_PERSON", "create_pattern": true}
  ],
  "dismissals": [
    {"mention_id": 456, "reason": "Not a person reference"}
  ]
}'
```

### Create Patterns

```bash
# Pattern is created automatically when create_pattern: true in resolution
# Or create manually:
penf process mentions batch-resolve '{
  "new_patterns": [
    {"mention_text": "JB", "entity_id": 5, "entity_type": "ENTITY_TYPE_PERSON"}
  ]
}'
```

## When Mentions Queue for Review

A mention goes to the review queue when:

1. **No candidates found** - The name/reference matches nothing
2. **Multiple low-confidence candidates** - Can't auto-pick
3. **LLM uncertain** - LLM confidence below threshold
4. **New pattern needed** - First time seeing this reference

## Claude's Role

Claude processes mentions via `penf process mentions context`:

1. Gets all pending mentions with candidates
2. Analyzes context and candidate scores
3. Resolves high-confidence ones automatically
4. Asks user about ambiguous ones
5. Creates patterns for recurring resolutions

See [Mention Review Workflow](../workflows/mention-review.md) for details.

## Related Documentation

- [People](people.md) - Person entity details
- [Entities](entities.md) - All entity types
- [Mention review workflow](../workflows/mention-review.md) - Processing pending mentions
