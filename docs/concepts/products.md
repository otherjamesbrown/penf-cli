# Products

Products represent business products, services, or features that your
organization works on. They support hierarchy and team associations.

## Product Hierarchy

Products can be organized hierarchically:

```
Product (top-level)
├── Sub-Product
│   ├── Feature
│   └── Feature
└── Sub-Product
    └── Feature
```

**Example:**
```
Managed Databases (product)
├── DBaaS (sub_product)
│   ├── PostgreSQL Support (feature)
│   └── MySQL Support (feature)
└── Managed Redis (sub_product)
    └── Cluster Mode (feature)
```

## Product Types

| Type | Description |
|------|-------------|
| `product` | Top-level business product |
| `sub_product` | Component or sub-product |
| `feature` | Specific feature or capability |

## Product Fields

| Field | Description |
|-------|-------------|
| `name` | Product name |
| `description` | What the product does |
| `product_type` | `product`, `sub_product`, `feature` |
| `parent_id` | Parent product (for hierarchy) |
| `status` | Lifecycle status |

## Product Status

| Status | Description |
|--------|-------------|
| `active` | Currently active and supported |
| `beta` | In beta testing |
| `sunset` | Being phased out |
| `deprecated` | No longer supported |

## Product Aliases

Products can have aliases for matching variations:

```bash
penf product add "DBaaS" --aliases "Database as a Service,Managed DB"
```

This helps when content uses different names for the same product.

## Team Associations

Products can be linked to teams:

```bash
penf product team add "DBaaS" --team-id 5 --role owner
penf product team add "DBaaS" --team-id 8 --role contributor
```

### Team Roles

| Role | Description |
|------|-------------|
| `owner` | Primary team responsible |
| `contributor` | Contributing team |
| `stakeholder` | Interested parties |

## Timeline Events

Track product milestones and decisions:

```bash
penf product event add "DBaaS" \
  --type milestone \
  --title "GA Release" \
  --date 2024-03-15 \
  --description "General availability launch"
```

### Event Types

- `milestone` - Key dates/achievements
- `decision` - Architecture/business decisions
- `change` - Significant changes
- `incident` - Issues or outages

## CLI Commands

### List Products

```bash
# All products
penf product list

# Filter by type
penf product list --type product

# Filter by status
penf product list --status active

# JSON output
penf product list -o json
```

### Add Products

```bash
# Simple
penf product add "DBaaS" --type product

# With details
penf product add "DBaaS" \
  --type product \
  --description "Database as a Service platform" \
  --status active

# With parent (hierarchy)
penf product add "PostgreSQL Support" \
  --type feature \
  --parent "DBaaS"
```

### Show Product Details

```bash
penf product show "DBaaS"
```

### View Hierarchy

```bash
penf product hierarchy "Managed Databases"
# Output:
# Managed Databases
# ├── DBaaS
# │   ├── PostgreSQL Support
# │   └── MySQL Support
# └── Managed Redis
#     └── Cluster Mode
```

### Team Management

```bash
# List teams for product
penf product team list "DBaaS"

# Add team
penf product team add "DBaaS" --team-id 5 --role owner

# Remove team
penf product team remove "DBaaS" --team-id 5
```

### Timeline

```bash
# View timeline
penf product timeline "DBaaS"

# Add event
penf product event add "DBaaS" \
  --type decision \
  --title "Migrate to K8s" \
  --date 2024-01-10
```

## Linking to Glossary

Glossary terms can be linked to products:

```bash
penf glossary link DBaaS 123 --type product
```

This enables:
- "DBaaS" in content links to the product entity
- Search for "DBaaS" shows product context
- Mention resolution knows "DBaaS" is a product

## Natural Language Queries

Ask questions about products:

```bash
penf product query "Who owns DBaaS?"
penf product query "What products is Team X working on?"
penf product query "What decisions were made about LKE this quarter?"
```

## Related Documentation

- [Entities overview](entities.md) - All entity types
- [Glossary linking](glossary.md) - Linking terms to products
- [Init entities workflow](../workflows/init-entities.md) - Seeding products
