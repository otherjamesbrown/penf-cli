# People

People entities represent individuals mentioned in or authoring content.

## How People Are Created

### From Email Headers

When emails are ingested, people are auto-created from:
- `From:` - Email sender
- `To:` - Direct recipients
- `CC:` - Carbon copy recipients

These have `auto_created: true` and `needs_review: true`.

### From Mention Resolution

When content mentions someone by name ("John said..."), and the mention
resolves to a new person, an entity is created.

### Manual Creation

Via CLI or seeding:
```bash
penf init entities  # Interactive seeding
```

## Person Fields

| Field | Description |
|-------|-------------|
| `canonical_name` | Primary display name |
| `email_addresses` | Array of known emails |
| `aliases` | Alternative names, nicknames |
| `company` | Organization |
| `job_title` | Role/position |
| `department` | Department within company |
| `is_internal` | True if internal to your org |
| `account_type` | `person`, `service_account`, `distribution_list` |

## Resolution: Matching Names to People

When Penfold sees "JB mentioned..." in content, it attempts to resolve
"JB" to a known person.

### Resolution Stages

1. **Exact email match**
   - If mention includes email, match directly
   - Highest confidence

2. **Alias lookup**
   - Check if "JB" is a known alias for someone
   - Configured aliases have high confidence

3. **Name similarity**
   - Compare against `canonical_name` and `aliases`
   - Uses fuzzy matching for typos

4. **LLM disambiguation**
   - If multiple candidates, use LLM with context
   - Considers surrounding text, project, history

### Confidence Scores

Resolution produces a confidence score (0.0 - 1.0):

| Score | Meaning | Action |
|-------|---------|--------|
| 0.9+ | High confidence | Auto-resolve |
| 0.7 - 0.9 | Medium confidence | Auto-resolve, flag for review |
| 0.5 - 0.7 | Low confidence | Queue for human review |
| < 0.5 | Very low | Don't resolve, queue for review |

## Aliases

Aliases help resolution work with variations:

```
canonical_name: "James Brown"
aliases: ["JB", "J. Brown", "Jimmy"]
```

All of these will resolve to the same person.

### Common Alias Patterns

- First name + last initial: "John S."
- Initials: "JS", "J.S."
- Nicknames: "Jim" for "James"
- Formal/informal: "Robert" / "Bob"
- Typos: "Jmaes" for "James"

## Duplicate Detection

Penfold detects potential duplicate people:

```
Person A: "Sarah Johnson" <sarah.j@company.com>
Person B: "Sarah J." <sarah.johnson@company.com>

→ Potential duplicate detected (0.91 similarity)
```

### Merging Duplicates

```bash
# View potential duplicates
penf relationship entity list --duplicates

# Merge (keeps person A, merges B into A)
penf relationship entity merge <id-A> <id-B>
```

Merging:
- Combines email addresses
- Combines aliases
- Updates all references to point to merged entity

## Internal vs External

`is_internal` distinguishes:
- **Internal**: Colleagues in your organization
- **External**: Customers, partners, vendors

This affects:
- Search filtering ("show only internal discussions")
- Privacy considerations
- Resolution priority

## Review Status

Auto-created people have `needs_review: true`.

To review them, use the onboarding workflow:

```bash
# Get all entities needing review
penf process onboarding context --output json

# Mark as reviewed via batch command
penf process onboarding batch '{"confirm_people": [12, 14]}'
```

## CLI Commands

```bash
# List people
penf relationship entity list --type person

# Show person details
penf relationship entity show <id>

# Filter by confidence score
penf relationship entity list --type person --confidence-min 0.8

# Merge duplicate people (first ID is kept, second is merged into it)
penf relationship entity merge <keep-id> <merge-id>
```

Note: To find potential duplicates, use `penf process onboarding context` which
includes duplicate detection in its analysis.

## Related Documentation

- [Entities overview](entities.md) - All entity types
- [Mention resolution](mentions.md) - How mentions work
- [Onboarding workflow](../workflows/onboarding.md) - Reviewing new people
