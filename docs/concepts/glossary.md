# Glossary Terms

The glossary stores acronyms, abbreviations, and domain-specific terminology
for search query expansion and content understanding.

## Purpose

When you search for "TER meeting notes", Penfold expands this to also search
for "Technical Execution Review meeting notes" - finding content even when
the full form was used instead of the acronym.

## Term Structure

Each glossary term has:

| Field | Required | Description |
|-------|----------|-------------|
| `term` | Yes | The acronym/abbreviation (e.g., "TER") |
| `expansion` | Yes | Full form (e.g., "Technical Execution Review") |
| `definition` | No | Longer explanation |
| `context` | No | Array of tags for categorization |
| `aliases` | No | Alternative spellings, typos |
| `expand_in_search` | No | Whether to use for query expansion (default: true) |

## Multi-Context Terms

**The same acronym can mean different things in different contexts.**

### Example: "VIP"

```bash
# In sales/customer context
penf glossary add VIP "Very Important Person" --context sales,customers

# In networking/infrastructure context
penf glossary add VIP "Virtual IP Address" --context networking,MTC,infrastructure
```

Both entries can coexist. Penfold uses context to pick the right one.

### How Context Resolution Works

When Penfold encounters "VIP" in content:

1. **Extract context signals** from surrounding content:
   - Project references (e.g., "MTC project")
   - Topic keywords (e.g., "load balancer", "customer tier")
   - Source metadata (meeting type, email subject)

2. **Score glossary matches:**
   - Each entry's `context` tags are compared to signals
   - Higher overlap = better match

3. **Select best match:**
   - Use highest-scoring context match
   - If no context match, use the most common meaning
   - If truly ambiguous, may queue for human review

### When Context Can't Resolve

If context is insufficient:
- In search: Expands to ALL meanings (OR query)
- In entity linking: May queue for human review
- In display: Shows "[ambiguous: VIP]" with options

## Aliases for Typos & Variations

Link common misspellings or variations to the canonical term:

```bash
# Create the canonical term
penf glossary add TER "Technical Execution Review"

# Link common variations
penf glossary alias TER "T.E.R."
penf glossary alias TER "ter"
penf glossary alias TER "TIR"  # Common typo
```

All aliases resolve to the same expansion.

## Source Tracking

Terms track their origin:
- `manual` - Added via CLI by user
- `discovered` - Found in content, confirmed by user
- `imported` - Bulk imported from file

## CLI Commands

### List Terms

```bash
# All terms
penf glossary list

# Filter by context
penf glossary list --context MTC

# JSON output for Claude
penf glossary list -o json
```

### Add Terms

```bash
# Simple
penf glossary add MVP "Minimum Viable Product"

# With context and definition
penf glossary add DBaaS "Database as a Service" \
  --context MTC,Oracle,products \
  --definition "Managed database platform for enterprise customers"

# With aliases
penf glossary add MTC "Major TikTok Contract" \
  --aliases "TikTok Project","TT Contract"
```

### Test Query Expansion

```bash
penf glossary expand "TER meeting notes"
# Output:
#   Original: TER meeting notes
#   Expanded: (TER OR "Technical Execution Review") meeting notes
```

### Show Term Details

```bash
penf glossary show TER
# Output:
#   Term: TER
#   Expansion: Technical Execution Review
#   Context: MTC, meetings
#   Aliases: T.E.R., ter
#   Source: manual
```

## Processing Unknown Acronyms

When content processing finds an unknown acronym:

1. Creates a review question: "What does 'XYZ' mean?"
2. Queues for human review
3. Claude processes via `penf process acronyms context`

See [Acronym Review Workflow](../workflows/acronym-review.md) for details.

## Best Practices

### Do Add

- Domain-specific acronyms unknown to general audience
- Internal project codenames
- Company-specific terminology
- Common variations and typos as aliases

### Don't Add

- Universal acronyms (API, HTTP, JSON) - Unless your org uses them differently
- Terms that would cause false positives in search
- Temporary/joke terms

### Context Guidelines

Use context tags for:
- Project names (MTC, Phoenix)
- Domains (sales, engineering, legal)
- Product areas (DBaaS, LKE)

Keep context tags:
- Lowercase
- Consistent across terms
- Meaningful for disambiguation

## Related Documentation

- [Entity types](entities.md) - Glossary as an entity type
- [Acronym review workflow](../workflows/acronym-review.md) - Processing unknowns
- [Init entities workflow](../workflows/init-entities.md) - Bulk seeding glossary
