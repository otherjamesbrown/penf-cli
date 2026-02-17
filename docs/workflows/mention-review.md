# Workflow: Mention Review

Resolve person mentions that couldn't be automatically matched.

## Purpose

When Penfold processes content, it extracts person mentions ("JB said...",
"Per Sarah's email...") and tries to resolve them to known people. Mentions
that can't be auto-resolved are queued for human review.

## When to Use

- After content import discovers unresolved mentions
- `penf review questions stats` shows pending mention questions
- User asks to "resolve mentions" or "who is JB?"
- Part of the [onboarding workflow](onboarding.md)

## Batch Data Command

```bash
penf process mentions context --output json
```

Returns:

```json
{
  "mentions": [
    {
      "id": 201,
      "mentioned_text": "JB",
      "context_snippet": "JB mentioned that the timeline needs adjustment",
      "source_reference": "email-2024-01-15-001",
      "status": "pending",
      "candidates": [
        {
          "person_id": 5,
          "name": "James Brown",
          "email": "james.brown@company.com",
          "score": 0.85,
          "reason": "Initials match, same project context"
        },
        {
          "person_id": 12,
          "name": "John Baker",
          "email": "john.baker@company.com",
          "score": 0.72,
          "reason": "Initials match"
        }
      ]
    }
  ],
  "patterns": [
    {
      "pattern_text": "SJ",
      "entity_id": 3,
      "entity_name": "Sarah Johnson",
      "times_used": 15
    }
  ],
  "stats": {
    "total_pending": 8,
    "resolved_today": 5,
    "patterns_count": 23
  },
  "workflow": {
    "batch_command": "penf process mentions batch-resolve '<json>'"
  }
}
```

## Decision Guidelines

### Auto-Resolve (High Confidence)

When a single candidate has score > 0.9:
- Strong alias match
- Email prefix match
- Consistent with existing patterns

### Needs Human Input

- **Multiple candidates**: Similar scores, can't auto-pick
- **No candidates**: Unknown person, may need to create
- **Low confidence**: Best candidate scores below threshold
- **Ambiguous context**: Same initials, different contexts

### Common Patterns

| Mention Type | Resolution Strategy |
|--------------|---------------------|
| Full name | Usually auto-resolves |
| First name only | Check context for disambiguation |
| Initials (JB) | Often needs human input |
| Nickname | May need alias creation |
| Typo | May need alias for the typo |

### Not a Person

Some "mentions" aren't actually people:
- Product names that look like names
- Acronyms miscategorized as names
- Transcription errors

Dismiss these with appropriate reason.

## Intelligent Processing Strategy

When Claude receives the context:

1. **Review candidates for each mention:**
   - Single high-confidence → resolve
   - Clear winner with context → resolve
   - Uncertain → ask user

2. **Look for patterns:**
   - If "JB" resolved to James Brown in same project before → use pattern
   - If new pattern would help → offer to create

3. **Present summary to user:**
   ```
   Found 8 unresolved mentions:
   - 3 have clear matches (auto-resolving)
   - 2 have multiple candidates (need your pick)
   - 2 might be new people (need confirmation)
   - 1 doesn't look like a person (dismissing)
   ```

4. **Create patterns for recurring resolutions**

## Available Actions

| Action | Effect |
|--------|--------|
| Resolve to person | Links mention to existing person |
| Resolve + create pattern | Links and creates pattern for future |
| Create new person | Creates person entity, then links |
| Dismiss | Marks as not a person mention |

## Batch Resolve Format

```bash
penf process mentions batch-resolve '{
  "resolutions": [
    {
      "mention_id": 201,
      "entity_id": 5,
      "entity_type": "ENTITY_TYPE_PERSON",
      "create_pattern": true
    },
    {
      "mention_id": 202,
      "entity_id": 12,
      "entity_type": "ENTITY_TYPE_PERSON",
      "create_pattern": false
    }
  ],
  "new_patterns": [
    {
      "mention_text": "Jimmy B",
      "entity_id": 5,
      "entity_type": "ENTITY_TYPE_PERSON"
    }
  ],
  "dismissals": [
    {
      "mention_id": 203,
      "reason": "Product name, not a person"
    }
  ]
}'
```

## Creating Patterns

Patterns enable auto-resolution of recurring mentions:

```
Pattern: "JB" → James Brown (person_id: 5)
```

When to create patterns:
- Initials that consistently refer to same person
- Nicknames
- Common typos of names

When NOT to create patterns:
- Generic names that could be anyone ("John")
- Context-dependent references
- One-time mentions

## Example Session

```
$ penf process mentions context --output json

# Claude analyzes:
Found 8 unresolved mentions:

1. "JB" - "JB mentioned that the timeline..."
   Candidates:
   - James Brown (85%) - same MTC project context
   - John Baker (72%) - different department

   → Resolve to James Brown, create pattern? [y/n] y

2. "Lisa" - "Lisa will send the report..."
   Candidates:
   - Lisa Park (78%) - vendor contact
   - Lisa Chen (71%) - internal engineer

   Context suggests vendor discussion.
   → Resolve to Lisa Park? [y/n] y

3. "TechBot" - "TechBot posted the update..."
   No candidates. Looks like a service account, not person.
   → Dismiss? [y/n] y

# Execute batch:
$ penf process mentions batch-resolve '{
  "resolutions": [
    {"mention_id": 201, "entity_id": 5, "entity_type": "ENTITY_TYPE_PERSON", "create_pattern": true},
    {"mention_id": 202, "entity_id": 8, "entity_type": "ENTITY_TYPE_PERSON", "create_pattern": false}
  ],
  "dismissals": [
    {"mention_id": 203, "reason": "Service account, not person"}
  ]
}'

Batch complete: 2 resolved, 1 pattern created, 1 dismissed
```

## Handling New People

If a mention refers to someone not in the system:

1. **Check if they should be added:**
   - Will they appear in future content?
   - Is this a one-time mention?

2. **If adding:**
   ```bash
   # First, add the person
   penf relationship entity add --name "New Person" --email "new@company.com"

   # Then resolve the mention to them
   penf process mentions batch-resolve '{"resolutions":[...]}'
   ```

3. **If not adding:**
   - Dismiss with reason "One-time external reference"

## Related Documentation

- [People concepts](../concepts/people.md) - How people work
- [Mentions concepts](../concepts/mentions.md) - How mentions work
- [Onboarding workflow](onboarding.md) - Full post-import review
- [Acronym review workflow](acronym-review.md) - Similar process for acronyms
