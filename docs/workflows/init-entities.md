# Workflow: Init Entities

Seed known entities before importing content.

## Purpose

Before importing emails or documents, seed the system with entities you
already know about. This helps Penfold:

- Match mentions to the correct people
- Resolve acronyms without asking
- Link content to the right products/projects
- Reduce review queue after import

## When to Use

- First-time setup before any imports
- Before importing content from a new domain/project
- When you know many entities that will appear in content

## Command

```bash
penf init entities [--from-json <file>]
```

## Interactive Mode

Without `--from-json`, runs an interactive wizard:

```
$ penf init entities

Entity Seeding Wizard
=====================

1. PEOPLE
   Add key people who will appear in your content.

   Add person (or 'done' to continue):
   > Name: John Smith
   > Email: john.smith@company.com
   > Company: Acme Corp
   > Title (optional): VP Engineering

   Added: John Smith <john.smith@company.com>
   Add another? (y/n): n

2. PRODUCTS
   What products/services does your organization work on?

   Add product (or 'done' to continue):
   > Name: DBaaS
   > Description: Database as a Service platform
   > Aliases (comma-separated): Database Service, Managed DB

   Added: DBaaS
   Add another? (y/n): n

3. PROJECTS
   What active projects should Penfold know about?

   Add project (or 'done' to continue):
   > Name: MTC
   > Description: Major TikTok Contract migration
   > Keywords (comma-separated): TikTok, migration, Oracle

   Added: MTC
   Add another? (y/n): n

4. GLOSSARY
   What domain-specific acronyms does your organization use?

   Add term (or 'done' to continue):
   > Term: TER
   > Expansion: Technical Execution Review
   > Context (comma-separated, optional): MTC, meetings

   Added: TER = Technical Execution Review
   Add another? (y/n): n

Summary:
  People:   3 added
  Products: 2 added
  Projects: 1 added
  Glossary: 5 added

Ready to import content!
```

## JSON Import Mode

For bulk seeding, prepare a JSON file:

```json
{
  "people": [
    {
      "name": "John Smith",
      "email": "john.smith@company.com",
      "company": "Acme Corp",
      "title": "VP Engineering",
      "aliases": ["JS", "J. Smith"]
    },
    {
      "name": "Sarah Johnson",
      "email": "sarah.j@company.com",
      "is_internal": true
    }
  ],
  "products": [
    {
      "name": "DBaaS",
      "description": "Database as a Service platform",
      "type": "product",
      "aliases": ["Database Service", "Managed DB"],
      "status": "active"
    },
    {
      "name": "PostgreSQL Support",
      "description": "PostgreSQL engine support",
      "type": "feature",
      "parent": "DBaaS"
    }
  ],
  "projects": [
    {
      "name": "MTC",
      "description": "Major TikTok Contract migration",
      "status": "active",
      "keywords": ["TikTok", "migration", "Oracle"]
    }
  ],
  "glossary": [
    {
      "term": "TER",
      "expansion": "Technical Execution Review",
      "context": ["MTC", "meetings"]
    },
    {
      "term": "VIP",
      "expansion": "Very Important Person",
      "context": ["sales", "customers"]
    },
    {
      "term": "VIP",
      "expansion": "Virtual IP Address",
      "context": ["networking", "MTC"]
    }
  ]
}
```

Then import:

```bash
penf init entities --from-json entities.json
```

## Multi-Context Glossary Terms

Notice the JSON example has "VIP" twice with different contexts. This is
intentional - the same acronym can mean different things in different contexts.

See [Glossary Concepts](../concepts/glossary.md) for details on multi-context terms.

## What Gets Created

| Entity Type | Table | Flags Set |
|-------------|-------|-----------|
| People | `people` | `auto_created: false`, `needs_review: false` |
| Products | `products` | Standard product record |
| Projects | `projects` | Standard project record |
| Glossary | `glossary` | `source: 'imported'` |

Seeded entities are marked as confirmed (not auto-created), giving them
higher priority in resolution.

## Batch Size Limits

Each entity type (people, products, projects) has a maximum batch size of 500
items per request. If your JSON file contains more than 500 of any entity type,
split them into multiple files or import in stages:

```bash
# For large imports, split your JSON files
penf init entities --from-json entities-part1.json
penf init entities --from-json entities-part2.json
```

Glossary terms are processed individually, so there's no batch limit for them.

## After Seeding

1. **Import content:**
   ```bash
   penf ingest email *.eml
   ```

2. **Run onboarding review:**
   ```bash
   penf process onboarding context
   ```
   This shows what NEW entities were discovered during import.

## Tips

### Start Small

You don't need to seed everything. Start with:
- Key people who appear frequently
- Domain-specific acronyms
- Active projects

Penfold will discover the rest and queue for review.

### Export → Edit → Import

If you have entities in another system:
1. Export to CSV/JSON
2. Transform to the import schema
3. Import with `--from-json`

### Iterate

You can run `penf init entities` multiple times:
- Existing entities are updated (matched by email/name)
- New entities are added
- Nothing is deleted

## Related Documentation

- [Onboarding workflow](onboarding.md) - Review after import
- [Glossary concepts](../concepts/glossary.md) - Multi-context terms
- [Entities overview](../concepts/entities.md) - All entity types
