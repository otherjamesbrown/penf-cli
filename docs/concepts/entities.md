# Entities

Entities are the core objects that Penfold tracks and resolves.

## Entity Types

### People

Individuals mentioned in or authoring content.

**Fields:**
- `canonical_name` - Primary display name
- `email_addresses` - Known email addresses (array)
- `aliases` - Alternative names, nicknames
- `company` - Organization affiliation
- `job_title` - Role/title
- `is_internal` - Whether they're internal to your org

**Creation modes:**
- Auto-created from email headers (From/To/CC)
- Auto-created from mention resolution
- Manually added via CLI

**CLI:** `penf relationship entity list --type person`

### Products

Business products, services, or features your organization works on.

**Fields:**
- `name` - Product name
- `description` - What the product does
- `product_type` - `product`, `sub_product`, or `feature`
- `parent_id` - Hierarchy parent
- `status` - `active`, `beta`, `sunset`, `deprecated`

Products support hierarchy: Product → Sub-Product → Feature

**CLI:** `penf product list`, `penf product add`

### Projects

Initiatives, programs, or time-bounded efforts.

**Fields:**
- `name` - Project name
- `description` - What the project is about
- `status` - `planning`, `active`, `on_hold`, `completed`, `cancelled`
- `keywords` - Related terms for matching
- `start_date`, `end_date` - Timeline

**CLI:** Coming soon

### Teams

Groups of people working together.

**Fields:**
- `name` - Team name
- `description` - Team purpose
- `team_type` - `department`, `project_team`, `working_group`, `committee`
- `lead_person_id` - Team lead
- `member_person_ids` - Team members

**CLI:** Coming soon

### Glossary Terms

Acronyms, abbreviations, and domain terminology.

**Fields:**
- `term` - The acronym/abbreviation
- `expansion` - Full form
- `definition` - Longer explanation (optional)
- `context` - Tags for categorization (array)
- `aliases` - Alternative spellings

See [glossary.md](glossary.md) for detailed documentation.

**CLI:** `penf glossary list`, `penf glossary add`

## Entity Relationships

Entities connect to each other:

```
┌────────┐     member_of     ┌───────┐
│ Person │ ───────────────▶  │ Team  │
└────────┘                   └───────┘
    │                            │
    │ works_on                   │ owns
    ▼                            ▼
┌─────────┐                ┌─────────┐
│ Project │ ◀───────────── │ Product │
└─────────┘   related_to   └─────────┘
```

## Auto-Created vs Confirmed

### Auto-Created Entities

When Penfold processes content, it creates entities automatically:
- Email addresses → Person entities
- Unknown acronyms → Review queue (not yet in glossary)
- Name mentions → Resolved or queued for review

Auto-created entities have:
- `auto_created: true`
- `needs_review: true`
- Lower `confidence_score`

### Confirmed Entities

Entities that have been:
- Manually added via CLI
- Reviewed and confirmed via onboarding
- Seeded via `penf init entities`

Confirmed entities have:
- `needs_review: false`
- Higher trust in resolution

## Resolution Priority

When multiple entities could match, Penfold prefers:

1. **Exact email match** - Definitive
2. **Confirmed entities** - Over auto-created
3. **Higher confidence score** - Based on evidence
4. **More recent activity** - Recently seen entities

## Related Documentation

- [Glossary concepts](glossary.md) - Multi-context terms
- [People resolution](people.md) - How people are matched
- [Mention resolution](mentions.md) - How mentions become entities
- [Onboarding workflow](../workflows/onboarding.md) - Review auto-created entities
