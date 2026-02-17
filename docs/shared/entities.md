# Penfold Entity Model

This document describes the key entities in the Penfold system and their relationships.

> **Last updated:** 2026-01-26

---

## Overview

Penfold is a personal information system that aggregates content from communication channels (email, meetings, documents) and builds a queryable knowledge base with entity resolution, relationship discovery, and AI-powered enrichment.

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              ENTITY DOMAINS                                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│   CONTENT                    ORGANIZATIONAL              KNOWLEDGE              │
│   ┌─────────────────┐       ┌─────────────────┐        ┌─────────────────┐     │
│   │ Sources         │       │ People          │        │ Glossary        │     │
│   │ Meetings        │       │ Teams           │        │ Extracted Links │     │
│   │ Attachments     │       │ Projects        │        │ Content Mentions│     │
│   └─────────────────┘       │ Products        │        └─────────────────┘     │
│                             └─────────────────┘                                 │
│                                                                                 │
│   PROCESSING                 CONFIGURATION              AI INFRASTRUCTURE       │
│   ┌─────────────────┐       ┌─────────────────┐        ┌─────────────────┐     │
│   │ Ingest Jobs     │       │ Tenants         │        │ AI Models       │     │
│   │ Enrichment      │       │ Domains         │        │ Routing Rules   │     │
│   │ Review Queue    │       │ Integrations    │        │ Model Health    │     │
│   └─────────────────┘       └─────────────────┘        └─────────────────┘     │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Content Entities

### Sources

The primary content container. Stores ingested content from emails, documents, and other sources.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `tenant_id` | Multi-tenant isolation |
| `content_id` | Unique content identifier (format: `<type>-<timestamp><random>`) |
| `raw_content` | Original content text |
| `source_tag` | User-defined label (e.g., "outlook-backup-2024") |
| `meeting_id` | Link to meeting record (if transcript/chat) |
| `participant_emails` | Extracted participant email addresses |

**Content ID Format:** `<type>-<id>` where type is:
- `em` = email
- `mt` = meeting
- `dc` = document
- `tr` = transcript
- `at` = attachment

**CLI:** `penf ingest email`, `penf ingest file`, `penf search`

---

### Meetings

Meeting records with metadata, linked to transcript and chat sources.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `title` | Meeting title |
| `meeting_date` | When the meeting occurred |
| `platform` | `webex`, `teams`, `zoom`, `google_meet` |
| `duration_seconds` | Meeting length |
| `participants` | JSON array of participant names |
| `participant_count` | Number of participants |
| `has_transcript` | Whether transcript is available |
| `has_chat` | Whether chat log is available |
| `processing_status` | `pending`, `processing`, `completed`, `failed` |

**Relationships:**
- Sources → Meetings (via `meeting_id`)
- Meeting Participants → People (via `person_id`)

**CLI:** `penf ingest meeting`

---

### Source Attachments

Files attached to source content (emails, documents).

| Field | Description |
|-------|-------------|
| `source_id` | Parent source |
| `filename` | Original filename |
| `content_type` | MIME type |
| `size_bytes` | File size |
| `storage_path` | Where file is stored |

---

## Organizational Entities

### People

Canonical person records resolved from email addresses, names, and other identifiers.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `canonical_name` | Normalized display name |
| `primary_email` | Primary email address |
| `title` | Job title |
| `department` | Department |
| `country` | Geographic location |
| `is_internal` | True if internal based on domain matching |
| `account_type` | `person`, `role`, `distribution`, `bot`, `external_service` |
| `confidence` | Resolution confidence (0.0-1.0) |
| `needs_review` | Flag for human review |
| `potential_duplicates` | Array of potentially duplicate person IDs |

**Account Types:**
- `person` - Individual human
- `role` - Role-based account (e.g., support@)
- `distribution` - Distribution list
- `bot` - Automated account (e.g., jira@)
- `external_service` - External service account

**CLI:** `penf init entities` (imports from CSV)

---

### Person Aliases

Alternative identifiers that resolve to a person.

| Field | Description |
|-------|-------------|
| `person_id` | Parent person |
| `alias_type` | `email`, `slack_id`, `name`, `display_name` |
| `alias_value` | The alias value |
| `confidence` | How confident we are in this mapping |
| `source` | `auto_created`, `gmail_api`, `manual` |

**Example:** Person "John Smith" might have aliases:
- `email`: john.smith@company.com
- `email`: jsmith@company.com
- `name`: "John Smith"
- `display_name`: "J. Smith"
- `slack_id`: U12345678

---

### Teams

Groups of people for organizational structure.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `name` | Team name (unique per tenant) |
| `description` | Team description |

**Relationships:**
- Team Members → People
- Product Teams → Products

---

### Projects

Work projects with keyword matching and Jira integration.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `name` | Project name |
| `description` | Project description |
| `keywords` | Array of keywords for content matching |
| `jira_projects` | Array of linked Jira project keys |

**Relationships:**
- Project Members → People or Teams
- Content Enrichment → Projects (via `project_id`)

---

## Products

### Products

Business products organized in a 3-level hierarchy.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `name` | Product name |
| `description` | Product description |
| `parent_id` | Parent product (for hierarchy) |
| `product_type` | `product`, `sub_product`, `feature` |
| `status` | `active`, `beta`, `sunset`, `deprecated` |
| `keywords` | Search keywords |

**Hierarchy:**
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
LKE (product)
├── LKE Standard (sub_product)
│   ├── Node Pools (feature)
│   └── Auto-scaling (feature)
└── LKE Enterprise (sub_product)
    └── ACL Clusters (feature)
```

**CLI:** `penf product list`, `penf product add`, `penf product hierarchy`

---

### Product Aliases

Alternative names for products.

| Field | Description |
|-------|-------------|
| `product_id` | Parent product |
| `alias` | Alternative name (globally unique) |

**Example:** "LK Enterprise" → resolves to "LKE Enterprise"

---

### Product Teams

Associates teams with products, with optional context labels.

| Field | Description |
|-------|-------------|
| `product_id` | Product |
| `team_id` | Team |
| `context` | Team context (e.g., "Core Team", "DRI Team", "Engineering") |

A team can have multiple contexts on the same product.

**CLI:** `penf product team <product>`

---

### Product Team Roles

Scoped role assignments within product teams.

| Field | Description |
|-------|-------------|
| `product_team_id` | Product-team association |
| `person_id` | Person |
| `role` | Role name (e.g., "DRI", "Manager", "Lead") |
| `scope` | Role scope (e.g., "Networking", "Database", "Security") |
| `is_active` | Whether role is currently active |
| `started_at` | When role started |
| `ended_at` | When role ended (if inactive) |

**Example Query:** "Who is the DRI for LKE Networking?"

---

### Product Events (Timeline)

Timeline events for tracking product history.

| Field | Description |
|-------|-------------|
| `product_id` | Product |
| `event_type` | `decision`, `milestone`, `risk`, `release`, `competitor`, `org_change`, `market`, `note` |
| `visibility` | `internal` (our events) or `external` (competitor/market) |
| `source_type` | `manual` or `derived` (extracted from content) |
| `title` | Event title |
| `description` | Event description |
| `occurred_at` | When the event happened |
| `recorded_by` | Who recorded it |
| `metadata` | Additional structured data (JSON) |

**CLI:** `penf product event add`, `penf product timeline`

---

## Knowledge Entities

### Glossary

Domain terminology and acronym definitions for search query expansion.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `term` | Acronym or term (e.g., "TER", "DBaaS") |
| `expansion` | Full expansion (e.g., "Technical Execution Review") |
| `definition` | Longer description |
| `context` | JSON array of context tags (e.g., ["MTC", "meetings"]) |
| `aliases` | Alternative forms (e.g., ["T.E.R.", "ter"]) |
| `expand_in_search` | Whether to use for query expansion |
| `source` | `manual`, `extracted`, `suggested` |

**Entity Linking:** Terms can be linked to products, projects, or companies via `glossary_linked_entities`.

**CLI:** `penf glossary list`, `penf glossary add`, `penf glossary expand "<query>"`

---

### Extracted Links

URLs extracted from content with categorization.

| Field | Description |
|-------|-------------|
| `url` | The URL |
| `link_category` | Category (Jira, Confluence, Google Docs, etc.) |
| `normalized_url` | Canonical form |
| `first_seen_at` | When first encountered |
| `occurrence_count` | How many times seen |

**Link Categories:** `jira_ticket`, `confluence_page`, `google_doc`, `google_sheet`, `github_pr`, `github_issue`, `slack_message`, `external`, `internal`, `unknown`

---

### Content Mentions

References to entities found in content (unified mention resolution).

| Field | Description |
|-------|-------------|
| `content_id` | Source content |
| `entity_type` | `person`, `term`, `product`, `company`, `project` |
| `mentioned_text` | The text that was mentioned |
| `position` | Character offset in content |
| `context_snippet` | Surrounding text |
| `resolved_entity_id` | Resolved entity (FK to appropriate table) |
| `resolution_confidence` | Confidence score |
| `resolution_source` | `exact_match`, `alias`, `fuzzy`, `project_context`, `user_confirmed` |
| `status` | `pending`, `auto_resolved`, `user_resolved`, `dismissed` |
| `candidates` | JSON array of candidate matches |

**CLI:** `penf audit mentions`, `penf process mentions`

---

## Review & Processing

### Review Queue

Questions for human review when AI needs clarification.

| Field | Description |
|-------|-------------|
| `question_type` | `acronym`, `person`, `entity`, `duplicate`, `other` |
| `priority` | `high`, `medium`, `low` |
| `question` | The question text |
| `context` | Additional context |
| `source_type` | Where question originated (meeting, email, etc.) |
| `source_id` | Source record ID |
| `suggested_term` | For acronym questions |
| `suggested_expansion` | AI's best guess |
| `candidate_person_ids` | For person disambiguation |
| `status` | `pending`, `resolved`, `dismissed`, `deferred` |
| `resolution` | User's answer |
| `confidence` | AI confidence that human input is needed |

**CLI:** `penf review list`, `penf review resolve`

---

### Ingest Jobs

Tracks batch import operations for progress and resume.

| Field | Description |
|-------|-------------|
| `id` | Job ID |
| `source_tag` | User-defined source label |
| `status` | `pending`, `running`, `completed`, `failed`, `cancelled` |
| `total_items` | Total items to process |
| `processed_items` | Items processed so far |
| `failed_items` | Items that failed |
| `last_file_path` | For resume capability |
| `labels` | Labels to apply to ingested content |

**CLI:** `penf ingest status`, `penf ingest queue`

---

### Content Enrichment

Processing status and extracted metadata for each source.

| Field | Description |
|-------|-------------|
| `source_id` | Source being enriched |
| `content_type` | `email`, `calendar`, `document`, `attachment` |
| `content_subtype` | Specific subtype (e.g., `thread`, `notification/jira`, `invite`) |
| `processing_profile` | `full_ai`, `metadata_only`, `state_tracking`, etc. |
| `status` | `pending`, `classifying`, `enriching`, `completed`, `failed` |
| `participants` | Raw participant addresses |
| `resolved_participants` | Participants resolved to person_ids |
| `extracted_links` | URLs with categories |
| `extracted_data` | Type-specific extraction (JSON) |
| `ai_processed` | Whether AI extraction completed |

**CLI:** `penf pipeline status`

---

## Multi-Tenant Configuration

### Tenants

Organizational accounts for multi-tenant isolation.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `name` | Organization name |
| `slug` | URL-safe identifier |
| `is_active` | Whether tenant is active |
| `settings` | Tenant-wide settings (JSON) |

**CLI:** `penf tenant list`, `penf tenant switch`

---

### Tenant Domains

Known domains for internal/external classification.

| Field | Description |
|-------|-------------|
| `tenant_id` | Parent tenant |
| `domain` | Email domain (e.g., "company.com") |
| `domain_type` | `internal`, `external_known`, `external_unknown` |

Used to classify people as internal or external based on email domain.

---

### Tenant Email Patterns

Patterns for detecting bots, distribution lists, and role accounts.

| Field | Description |
|-------|-------------|
| `tenant_id` | Parent tenant |
| `pattern` | Glob pattern (e.g., `*-jira@*`, `team-*@*`) |
| `pattern_type` | `bot`, `distribution_list`, `role_account`, `ignore` |
| `priority` | Lower = checked first |

---

### Tenant Integrations

External service connections.

| Field | Description |
|-------|-------------|
| `tenant_id` | Parent tenant |
| `integration_type` | `jira`, `google_workspace`, `slack`, `confluence`, `github`, `linear` |
| `name` | Integration name |
| `instance_url` | Service URL |
| `config` | Integration config (JSON) |
| `sync_status` | `healthy`, `error`, `syncing`, `never_synced` |

---

## AI Infrastructure

### AI Models

Registry of available AI models (local and remote).

| Field | Description |
|-------|-------------|
| `id` | Model ID (format: `provider/model-name`) |
| `name` | Display name |
| `provider` | `ollama`, `gemini`, `openai`, `anthropic`, `mlx` |
| `model_name` | Provider-specific identifier |
| `capabilities` | Array: `embedding`, `summarization`, `extraction`, `classification` |
| `context_window` | Max input tokens |
| `input_cost_per_1k` | Cost per 1K input tokens (USD) |
| `output_cost_per_1k` | Cost per 1K output tokens (USD) |
| `is_local` | True for local models (ollama, mlx) |
| `is_enabled` | Administrative enable/disable |
| `priority` | Default priority (lower = preferred) |

**CLI:** `penf model list`, `penf model add`, `penf model pull`

---

### AI Routing Rules

Task-based model selection rules.

| Field | Description |
|-------|-------------|
| `name` | Rule name |
| `task_type` | `embedding`, `summarization`, `extraction`, `classification` |
| `preferred_models` | Ordered list of preferred model IDs |
| `fallback_models` | Fallback chain |
| `require_local` | Enforce local-only |
| `max_cost_per_request` | Cost ceiling |
| `optimization_mode` | `latency`, `quality`, `cost`, `balanced` |

---

### Embeddings

**Critical for semantic search.** Vector representations of content for similarity matching.

| Field | Description |
|-------|-------------|
| `id` | Primary key |
| `tenant_id` | Multi-tenant isolation |
| `entity_type` | What was embedded: `source`, `assertion`, `person`, `project`, `team` |
| `entity_id` | ID of the embedded entity |
| `embedding_model` | Model used (e.g., `mlx/mxbai-embed-large-v1`) |
| `model_version` | Model version for cache invalidation |
| `text_content` | Text that was embedded |
| `content_hash` | SHA-256 of text (for deduplication) |
| `embedding` | Vector array (typically 1024 dimensions) |
| `search_count` | How often this embedding was matched in searches |
| `last_searched_at` | For analytics and cache management |

**Why Embeddings Matter:**
- Enable semantic search ("find discussions about scaling" finds "auto-scaling", "horizontal scaling", etc.)
- Power similarity matching for duplicate detection
- Support question-answering over content

**Embedding Sources:**
- `source_id` → Content from emails, meetings, documents
- `assertion_id` → Extracted facts/claims
- `person_id` → Person profile summaries
- `project_id` → Project descriptions
- `team_id` → Team descriptions

---

## Entity Relationships Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           ENTITY RELATIONSHIPS                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│                              ┌────────────┐                                     │
│                              │  Tenants   │                                     │
│                              └─────┬──────┘                                     │
│              ┌─────────────────────┼─────────────────────┐                     │
│              │                     │                     │                      │
│              ▼                     ▼                     ▼                      │
│       ┌───────────┐         ┌───────────┐         ┌───────────┐                │
│       │  People   │◄────────│   Teams   │─────────│ Projects  │                │
│       └─────┬─────┘         └─────┬─────┘         └───────────┘                │
│             │                     │                                             │
│             │    ┌────────────────┘                                             │
│             │    │                                                              │
│             ▼    ▼                                                              │
│       ┌─────────────┐        ┌─────────────┐        ┌─────────────┐            │
│       │  Product    │◄───────│ Product     │───────►│ Product     │            │
│       │  Team Roles │        │ Teams       │        │ Events      │            │
│       └─────────────┘        └──────┬──────┘        └─────────────┘            │
│                                     │                                           │
│                                     ▼                                           │
│                              ┌───────────┐                                      │
│                              │ Products  │◄─── Product Aliases                  │
│                              └───────────┘                                      │
│                                                                                 │
│       ┌───────────┐         ┌───────────┐         ┌───────────┐                │
│       │  Sources  │────────►│ Meetings  │         │ Glossary  │◄─── Linked    │
│       └─────┬─────┘         └───────────┘         └───────────┘     Entities   │
│             │                                                                   │
│             ▼                                                                   │
│       ┌─────────────┐        ┌─────────────┐                                   │
│       │ Content     │───────►│ Content     │───────► People/Products/Terms     │
│       │ Enrichment  │        │ Mentions    │                                   │
│       └─────────────┘        └─────────────┘                                   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## CLI Quick Reference

| Entity | List | Add | Show |
|--------|------|-----|------|
| Products | `penf product list` | `penf product add` | `penf product show <name>` |
| Glossary | `penf glossary list` | `penf glossary add` | `penf glossary show <term>` |
| People | `penf init entities` | - | - |
| Teams | `penf init entities` | - | - |
| AI Models | `penf model list` | `penf model add` | - |
| Review Queue | `penf review list` | - | - |
| Ingest Jobs | `penf ingest status` | `penf ingest email` | - |

---

## See Also

- [vision.md](vision.md) - What Penfold is and why
- [use-cases.md](use-cases.md) - Prioritized use cases
- [interaction-model.md](interaction-model.md) - How users interact via Claude Code
- [../ARCHITECTURE.md](../ARCHITECTURE.md) - System architecture and data flow
- [../infrastructure.md](../infrastructure.md) - Deployment topology and configuration
