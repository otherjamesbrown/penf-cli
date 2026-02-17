# Workflow: Post-Import Onboarding

Guided review of entities discovered after importing content.

## Purpose

After importing emails or documents, Penfold discovers new entities:
- People from email headers
- Unknown acronyms
- Unresolved person mentions
- Potential duplicate people

This workflow helps Claude guide you through reviewing them efficiently.

## When to Use

- After `penf ingest email` imports new content
- After `penf ingest meeting` processes transcripts
- Periodically to review accumulated discoveries
- User asks to "review new entities" or "what did we find?"

## Batch Data Command

```bash
penf process onboarding context --output json
```

Returns everything Claude needs for guided review:

```json
{
  "summary": {
    "new_people": 12,
    "new_acronyms": 15,
    "unresolved_mentions": 8,
    "potential_duplicates": 3,
    "last_import": "2024-01-22T10:30:00Z"
  },
  "new_people": [
    {
      "id": 45,
      "canonical_name": "Sarah Johnson",
      "email_addresses": ["sarah.j@external.com"],
      "company": null,
      "auto_created": true,
      "needs_review": true,
      "source_count": 5,
      "first_seen": "2024-01-15T10:30:00Z"
    }
  ],
  "new_acronyms": [
    {
      "id": 123,
      "term": "PLD",
      "question": "What does 'PLD' mean?",
      "context": "...discussed in the PLD review yesterday...",
      "source_reference": "email-2024-01-15-001",
      "priority": "medium"
    }
  ],
  "unresolved_mentions": [
    {
      "id": 201,
      "mentioned_text": "JB",
      "context_snippet": "JB mentioned that the timeline...",
      "candidates": [
        {"person_id": 5, "name": "James Brown", "score": 0.85},
        {"person_id": 12, "name": "John Baker", "score": 0.72}
      ]
    }
  ],
  "potential_duplicates": [
    {
      "person_id": 45,
      "canonical_name": "Sarah Johnson",
      "email_addresses": ["sarah.j@external.com"],
      "potential_match": {
        "person_id": 3,
        "canonical_name": "Sarah J.",
        "email_addresses": ["sarah.johnson@external.com"],
        "similarity": 0.91
      }
    }
  ],
  "workflow": {
    "recommended_order": ["duplicates", "people", "acronyms", "mentions"],
    "batch_command": "penf process onboarding batch '<json>'"
  }
}
```

## Claude's Guided Flow

Claude processes the context and guides the user:

```
After importing 15 emails, here's what Penfold discovered:

Summary:
  - 12 new people (from email headers)
  - 15 unknown acronyms
  - 8 unresolved person mentions
  - 3 potential duplicate people

I'll guide you through each category. Starting with duplicates
since fixing those first prevents redundant work.

──────────────────────────────────────────────────────────────
POTENTIAL DUPLICATES (3)
──────────────────────────────────────────────────────────────

1. "Sarah Johnson" <sarah.j@external.com> (new, seen 5 times)
   Looks like: "Sarah J." <sarah.johnson@external.com> (existing)
   Similarity: 91%

   These appear to be the same person. Merge them?
   [y] Yes, merge  [n] No, keep separate  [s] Skip for now

> y

✓ Merged Sarah Johnson into Sarah J.

[... continues through each duplicate ...]

──────────────────────────────────────────────────────────────
NEW PEOPLE (12)
──────────────────────────────────────────────────────────────

These people were auto-created from email headers.
I can confirm them all, or we can review individually.

Auto-created people:
  1. Mike Chen <mike.chen@partner.com> (seen 8 times)
  2. Lisa Park <lisa.p@vendor.com> (seen 3 times)
  ...

[a] Confirm all  [r] Review individually  [s] Skip for now

> a

✓ Confirmed 12 people

[... continues through acronyms and mentions ...]
```

## Recommended Review Order

1. **Duplicates first** - Merging prevents duplicate effort
2. **People** - Quick confirmation, builds entity base
3. **Acronyms** - Enables search expansion
4. **Mentions** - Depends on people being resolved

## Batch Processing

After Claude determines actions, execute in batch:

```bash
penf process onboarding batch '{
  "merge_people": [
    {"keep_id": 3, "merge_id": 45}
  ],
  "confirm_people": [12, 14, 16, 18, 20, 22, 24, 26, 28, 30],
  "acronym_resolutions": [
    {"id": 123, "expansion": "Product Launch Date"},
    {"id": 124, "expansion": "Technical Execution Review"}
  ],
  "acronym_dismissals": [
    {"id": 125, "reason": "Speaker initials, not acronym"}
  ],
  "mention_resolutions": [
    {"mention_id": 201, "person_id": 5, "create_pattern": true}
  ],
  "mention_dismissals": [
    {"mention_id": 202, "reason": "Not a person reference"}
  ]
}'
```

## Available Actions

| Category | Action | Effect |
|----------|--------|--------|
| Duplicates | `merge_people` | Combines two people into one |
| People | `confirm_people` | Marks as reviewed, no longer auto-created |
| Acronyms | `acronym_resolutions` | Adds to glossary |
| Acronyms | `acronym_dismissals` | Marks as not needing glossary entry |
| Mentions | `mention_resolutions` | Links mention to person |
| Mentions | `mention_dismissals` | Marks as not a real mention |

## Partial Review

You don't have to review everything at once:

```bash
# Just review duplicates
penf process onboarding context --category duplicates

# Just review acronyms
penf process onboarding context --category acronyms
```

Or skip categories in Claude's guided flow.

## Dry Run

Preview changes without executing:

```bash
penf process onboarding batch --dry-run '{...}'
```

## Progress Tracking

The system tracks onboarding progress:

```bash
# View onboarding status
penf process onboarding status

# Output:
# Last import: 2024-01-22 10:30
# Pending review:
#   People: 4 remaining
#   Acronyms: 8 remaining
#   Mentions: 2 remaining
#   Duplicates: 0 remaining
```

## Related Documentation

- [Init entities workflow](init-entities.md) - Seed before import
- [Acronym review workflow](acronym-review.md) - Detailed acronym processing
- [Mention review workflow](mention-review.md) - Detailed mention processing
- [People concepts](../concepts/people.md) - Understanding people entities
