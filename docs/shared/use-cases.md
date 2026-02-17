# Penfold Use Cases

This document describes the vision for Penfold and the key use cases it aims to solve. These are not exhaustive but serve as a **north star** for development priorities.

> **Last updated:** 2026-01-26

---

## Vision

**Penfold is your institutional memory.**

In knowledge work, critical information is scattered across emails, meetings, documents, and chat. Context gets lost, decisions are forgotten, and expertise is siloed. Penfold solves this by:

1. **Aggregating** content from all communication channels into one searchable system
2. **Understanding** the people, products, and concepts mentioned in your content
3. **Connecting** related information through entity resolution and relationship discovery
4. **Surfacing** relevant knowledge when you need it through intelligent search and AI queries

The goal: **Never lose context. Always know who knows what.**

---

## Implementation Tiers

Use cases are prioritized into tiers to guide development focus:

| Tier | Status | Description |
|------|--------|-------------|
| **Tier 1 (Core)** | Building | Essential functionality - must work well |
| **Tier 2 (Important)** | Planned | Key differentiators - build after core |
| **Tier 3 (Future)** | Vision | Aspirational - guides architecture decisions |

### Current Status

| Use Case | Tier | Status |
|----------|------|--------|
| UC-1: Semantic Search | Tier 1 | **Implemented** - hybrid search working |
| UC-2: Meeting Intelligence | Tier 1 | **Implemented** - transcript ingestion working |
| UC-6: Acronym/Terminology | Tier 1 | **Implemented** - glossary and query expansion |
| UC-8: Email Archive | Tier 1 | **Implemented** - .eml ingestion working |
| UC-3: Expertise Discovery | Tier 2 | Partial - entity resolution working, expertise scoring planned |
| UC-4: Product Knowledge | Tier 2 | Partial - products working, timeline events planned |
| UC-7: Question Resolution | Tier 2 | **Implemented** - review queue working |
| UC-5: Daily Review | Tier 3 | Planned - design complete |
| UC-9-12: Secondary | Tier 3 | Vision - architecture allows for these |

---

## Core Problems We Solve

### 1. Lost Institutional Knowledge
> "What was decided about X six months ago?"

Decisions happen in meetings, email threads, and Slack channels. Without Penfold, finding that context means searching multiple systems, asking colleagues who might remember, or simply making the decision again without historical context.

### 2. Expert Discovery
> "Who should I talk to about Y?"

Organizations have deep expertise distributed across people, but it's often invisible. Penfold tracks who discusses what topics, who owns what products, and who has context on specific projects.

### 3. Context Switching Cost
> "I need to get up to speed on project Z."

Joining a project or inheriting responsibilities means piecing together history from scattered sources. Penfold provides a unified view of everything related to a topic, product, or person.

### 4. Information Overload
> "I can't keep up with everything happening."

Knowledge workers are drowning in email, meetings, and notifications. Penfold's review system and AI triage help surface what matters and filter out noise.

---

## Primary Use Cases

### UC-1: Semantic Search Across All Content

**User Story:** As a knowledge worker, I want to search across all my emails, meetings, and documents using natural language, so I can find relevant information regardless of where it lives or what exact words were used.

**Example Queries:**
- "What did we discuss about the database migration?"
- "Conversations with the security team about API authentication"
- "Budget discussions from Q4 planning"

**Key Features:**
- Hybrid search combining semantic understanding with keyword matching
- Query expansion using glossary (e.g., "TER" expands to "Technical Execution Review")
- Filters by content type, date range, participants
- Relevance ranking with optional date sorting

**CLI:** `penf search "database migration discussions"`

---

### UC-2: Meeting Intelligence

**User Story:** As a meeting participant, I want meeting transcripts automatically processed so I can search for what was discussed, find action items, and understand who attended without manually reviewing hours of content.

**Example Questions:**
- "What did we decide about the pricing change in last week's TER?"
- "Show me all meetings where we discussed the API redesign"
- "What action items came out of the product review?"

**Key Features:**
- Automatic transcript ingestion from Webex, Teams, Zoom
- Speaker identification and participant resolution
- Topic extraction and categorization
- Action item detection via SLM extraction (Stage 2)
- Multi-stage SLM/LLM pipeline: triage → extract → deep analysis
- Risk and assertion lifecycle tracking with golden thread

**CLI:** `penf ingest meeting ./transcripts/`, `penf search "pricing change" --type=meeting`

---

### UC-3: People & Expertise Discovery

**User Story:** As someone needing help with a topic, I want to quickly find who has expertise or context on a subject, so I can reach out to the right person.

**Example Questions:**
- "Who has been involved in discussions about Kubernetes networking?"
- "Who is the DRI for the database migration?"
- "Who should I talk to about compliance requirements?"

**Key Features:**
- Entity resolution linking email addresses to canonical people records
- Track participation in topics via content analysis
- Product team roles (DRI, Manager, Lead) with scopes
- Organization chart awareness (teams, departments)

**CLI:** `penf product query "who is the DRI for MTC"`, `penf search "kubernetes networking" --verbose`

---

### UC-4: Product Knowledge Base

**User Story:** As a product stakeholder, I want a single place to understand a product's history, team, decisions, and current status, so I can make informed decisions without chasing down tribal knowledge.

**Example Questions:**
- "What's the history of decisions on LKE Enterprise?"
- "Which teams work on the API Gateway?"
- "What competitor moves have we tracked for managed databases?"
- "When did we decide to deprecate the legacy API?"

**Key Features:**
- Product hierarchy (product → sub-product → feature)
- Timeline events (decisions, milestones, risks, competitor moves)
- Team associations with context labels
- Scoped role tracking with history

**CLI:** `penf product show "LKE Enterprise"`, `penf product timeline "LKE Enterprise"`

---

### UC-5: Daily Review & Triage

**User Story:** As a busy professional, I want an intelligent daily review that surfaces what needs my attention, so I can stay informed without reading everything.

**Example Workflow:**
1. Start daily review session
2. See prioritized items requiring attention
3. Accept, reject, or defer items
4. System learns preferences over time

**Key Features:**
- AI-powered triage and prioritization
- Configurable rules for auto-accept/reject
- Undo/redo support for review actions
- Session tracking and history
- Session bootstrap integration (penf context morning) for project-first briefings
- Radar model: AI tracks periphery, human focuses spotlight

**CLI:** `penf review start`, `penf review queue`

---

### UC-6: Acronym & Terminology Management

**User Story:** As someone joining an organization or project, I want to understand domain-specific acronyms and terminology, so I can comprehend communications without constantly asking what things mean.

**Example Situations:**
- Reading a meeting transcript full of acronyms
- Searching for content but not knowing the official term
- Onboarding to a new team with domain-specific language

**Key Features:**
- Glossary with terms, expansions, and definitions
- Automatic query expansion in search
- Context tagging for term categorization
- AI-assisted acronym detection and suggestion

**CLI:** `penf glossary list`, `penf glossary expand "TER meeting notes"`

---

### UC-7: Question Resolution

**User Story:** As a user, I want the system to ask me clarifying questions when it encounters ambiguous content, so the knowledge base becomes more accurate over time.

**Example Questions from the System:**
- "What does 'TER' mean in the context of MTC meetings?"
- "Is 'John S.' the same person as 'John Smith'?"
- "Should 'LK Enterprise' be linked to 'LKE Enterprise'?"

**Key Features:**
- Review queue for AI questions
- Person disambiguation UI
- Acronym definition workflow
- Duplicate entity detection

**CLI:** `penf review questions`, `penf process acronyms context`

---

### UC-8: Email Archive Search

**User Story:** As someone with years of email history, I want to import and search my email archive, so I can find conversations and context from before I used Penfold.

**Example Scenarios:**
- Searching an exported Outlook backup
- Finding old conversations with a former colleague
- Recovering context from a project years ago

**Key Features:**
- Batch .eml file ingestion
- Source tagging for organization
- Participant extraction and resolution
- Thread reconstruction

**CLI:** `penf ingest email ./archive/ --source "outlook-2020-2023"`

---

## Secondary Use Cases

### UC-9: AI-Powered Content Summarization

**User Story:** As a user with limited time, I want AI-generated summaries of long content, so I can quickly understand the gist without reading everything.

**Example Uses:**
- Summarize a 2-hour meeting transcript
- Get key points from a long email thread
- Understand a complex document

**CLI:** `penf ai summarize <content-id>`

---

### UC-10: Relationship Mapping

**User Story:** As someone trying to understand organizational dynamics, I want to see who communicates with whom and about what topics, so I can understand informal networks and collaboration patterns.

**Key Features:**
- Communication frequency tracking
- Topic co-occurrence analysis
- Team collaboration visualization

**CLI:** `penf relationship show <person>` (future)

---

### UC-11: Multi-Tenant Organization Support

**User Story:** As someone who works across multiple organizations or contexts, I want to keep my knowledge bases separate, so I can switch contexts cleanly.

**Key Features:**
- Tenant isolation for all data
- Per-tenant configuration (domains, patterns, integrations)
- Easy tenant switching

**CLI:** `penf tenant list`, `penf tenant switch <tenant>`

---

### UC-12: External Integration Intelligence

**User Story:** As a user, I want links to external systems (Jira, Confluence, Google Docs) automatically enriched, so I can understand what was linked without clicking through.

**Example Enrichments:**
- Jira ticket status, assignee, summary
- Confluence page title and last modified
- Google Doc owner and sharing status

**Key Features:**
- Link extraction and categorization
- External API enrichment (when configured)
- Link occurrence tracking

---

## Future Vision Use Cases

### UC-F1: Proactive Insights

**Vision:** The system proactively surfaces relevant information without being asked.

**Examples:**
- "You're meeting with Sarah tomorrow. Here's recent context from your communications."
- "This topic came up 6 months ago. Here's what was decided."
- "Three people in this thread also discussed this topic separately."

---

### UC-F2: Knowledge Graph Queries

**Vision:** Ask complex relationship questions across the knowledge graph.

**Examples:**
- "Who has context on both Kubernetes and compliance?"
- "What topics do the platform and security teams both discuss?"
- "Show me the decision trail that led to the current architecture."

---

### UC-F3: Automated Briefings

**Vision:** Generate personalized briefings for specific contexts.

**Examples:**
- "Prepare me for my 1:1 with Alex"
- "What do I need to know before the board meeting?"
- "Summarize everything that happened while I was on vacation"

---

## Success Metrics

| Use Case | Success Indicator |
|----------|-------------------|
| UC-1: Search | Time to find relevant content < 30 seconds |
| UC-2: Meetings | Key meeting content findable without re-watching |
| UC-3: Expertise | Correct expert identified within top 3 suggestions |
| UC-4: Products | Complete product context accessible in one place |
| UC-5: Review | Daily review time reduced by 50% |
| UC-6: Terminology | New team members productive faster |
| UC-7: Questions | Knowledge base accuracy improves over time |

---

## Non-Goals

To maintain focus, Penfold explicitly does **not** aim to:

1. **Replace email clients** - Penfold is for search and knowledge management, not day-to-day email workflow
2. **Be a project management tool** - Integrate with Jira/Linear, don't replace them
3. **Provide real-time collaboration** - Focus is on historical knowledge, not live editing
4. **Be a CRM** - We track relationships for knowledge context, not sales pipelines
5. **Store files as primary content** - Attachments are metadata; use Drive/Dropbox for files

---

## User Personas

### Alex - Engineering Manager
- Manages a team of 8 engineers
- Needs to stay informed across multiple projects
- Uses Penfold to: track decisions, find experts, prepare for 1:1s

### Jordan - Product Manager
- Owns a product with multiple sub-products
- Needs historical context for roadmap decisions
- Uses Penfold to: track product timeline, find past decisions, understand competitive landscape

### Sam - New Team Member
- Just joined 3 months ago
- Struggling to understand acronyms and history
- Uses Penfold to: learn terminology, find old context, discover who knows what

### Casey - Executive Assistant
- Supports multiple executives
- Needs to quickly find and summarize information
- Uses Penfold to: search across all communications, prepare briefings, track action items

---

## See Also

- [vision.md](vision.md) - Core vision and principles
- [entities.md](entities.md) - Data model and entity relationships
- [interaction-model.md](interaction-model.md) - How users interact via Claude Code
- [../ARCHITECTURE.md](../ARCHITECTURE.md) - System architecture
- [../infrastructure.md](../infrastructure.md) - Deployment and operations
