# Workflow: Acronym Review

## Purpose
Resolve unknown acronyms found during content processing (meeting transcripts, emails, documents). Acronyms are added to the glossary for future query expansion.

## When to Use
- `penf review questions stats` shows pending acronym questions
- User asks to "review acronyms" or "process the question queue"
- After ingesting new meeting transcripts or documents

## Batch Data Command
```bash
# Get everything needed for intelligent batch processing
penf process acronyms context --output json
```

Returns:
- All pending acronym questions with source snippets
- Current glossary terms (to avoid duplicates)
- Queue statistics

## Available Actions

| Action | Command | Effect |
|--------|---------|--------|
| Resolve | `penf review questions resolve <id> "<expansion>"` | Adds to glossary, marks resolved |
| Alias | `penf glossary alias <existing_term> <variant>` | Links variant to existing term (for search) |
| Dismiss | `penf review questions dismiss <id> "[reason]"` | Removes from queue, NO glossary entry |
| Defer | `penf review questions defer <id>` | Keeps in queue for later |
| View Source | `penf review questions source <id> --context 1500` | Shows surrounding transcript |

### When to Use Alias vs Dismiss

**Use ALIAS when:**
- Detected term is a plural/variant of existing term (DCs → DC, APIs → API)
- Transcription error but we want to match searches (DBAS → DBA)
- Alternative spelling or abbreviation of existing term

**Use DISMISS when:**
- Not an acronym at all (person's initials, product name like "NVIDIA")
- Transcription garbage that won't appear in searches
- Already an EXACT match in glossary (not a variant)

**Why this matters:** Searching for "emails about DCs" should find content containing "DC". If we dismiss "DCS" instead of aliasing it to "DC", the search won't find those matches.

## Decision Guidelines

### Auto-resolve (Claude can handle)
These are standard tech/business acronyms Claude knows:
- **Web/API**: REST, API, HTTP, HTTPS, JSON, XML, YAML, URL, URI, DNS, CDN, SSL, TLS, WebRTC, WebSocket
- **Development**: MVP, POC, SDK, IDE, CLI, CI/CD, TDD, BDD, OOP, DRY, SOLID, CRUD, MVC
- **Cloud/Infra**: AWS, GCP, Azure, K8s, VM, VPC, IAM, S3, EC2, RDS, ECS, EKS, Lambda
- **Database**: SQL, NoSQL, RDBMS, ORM, ACID, CAP, ETL, CDC
- **Business**: ROI, KPI, OKR, SLA, NDA, B2B, B2C, CRM, ERP

### Needs Human Input
- **Domain-specific**: Acronyms specific to user's company/industry
- **Ambiguous**: Could mean multiple things (e.g., "PM" = Product Manager or Project Manager?)
- **Context-dependent**: Meaning varies by project or team
- **Uncertain**: Claude isn't confident about the expansion

### Potential Mis-transcriptions
Watch for acronyms that might be speech-to-text errors:
- Check if nearby words suggest a different spelling
- "PLD" might be "PLM", "PLC", "PID"
- Single letters like "C" might be "see", "sea", numbers

### Already in Glossary or Variant
Before resolving, check if term exists (context command includes glossary).
- **Exact match exists** → dismiss with "Already in glossary"
- **Variant/plural exists** (e.g., "DCS" when "DC" exists) → alias with `penf glossary alias DC DCS`

## Data Structures

### Context Response (JSON)
```json
{
  "questions": [
    {
      "id": 123,
      "question": "What does 'TER' mean?",
      "suggested_term": "TER",
      "context": "...discussed in the TER meeting yesterday...",
      "source_reference": "meeting-2024-01-15",
      "priority": "medium",
      "question_type": "acronym"
    }
  ],
  "glossary": [
    {
      "term": "MVP",
      "expansion": "Minimum Viable Product",
      "context": ["product", "development"]
    }
  ],
  "stats": {
    "total_pending": 15,
    "by_type": {"acronym": 12, "person": 3}
  }
}
```

### Batch Resolve Format
```bash
penf process acronyms batch-resolve '{
  "resolutions": [
    {"id": 123, "expansion": "Technical Execution Review"},
    {"id": 456, "expansion": "Database as a Service"}
  ],
  "dismissals": [
    {"id": 789, "reason": "Already in glossary"},
    {"id": 101, "reason": "Not an acronym, speaker initials"}
  ]
}'
```

## Intelligent Processing Strategy

When Claude receives the context, it should:

1. **Categorize all questions**:
   - Known tech acronyms → batch resolve
   - Exact duplicates in glossary → batch dismiss
   - Variants/plurals of existing terms (DCS→DC) → batch alias
   - Uncertain/domain-specific → present to user with analysis

2. **Group similar items**:
   - Multiple questions about same term → resolve once
   - Related terms (e.g., TER, TERs) → resolve consistently

3. **Present summary to user**:
   ```
   Found 15 acronym questions:
   - 8 standard tech terms (auto-resolving)
   - 2 exact duplicates (dismissing)
   - 3 plurals/variants (aliasing: DCS→DC, APIs→API, VMS→VM)
   - 2 need your input:
     1. "PLD" in context "...the PLD review..." - could be PLM, PLC, or domain-specific
     2. "AW" in context "...AW mentioned..." - likely person initials, dismiss?
   ```

4. **Execute with single batch command** after user confirms

## Examples

### Full Batch Flow
```bash
# 1. Get full context
penf process acronyms context --output json > /tmp/acronyms.json

# 2. Claude analyzes and prepares batch actions

# 3. Execute batch resolve/dismiss
penf process acronyms batch-resolve '{
  "resolutions": [
    {"id": 24, "expansion": "Minimum Viable Product"},
    {"id": 25, "expansion": "Web Real-Time Communication"}
  ],
  "dismissals": [
    {"id": 26, "reason": "Speaker initials (Adam W)"}
  ]
}'

# 4. Create aliases for plurals/variants (after dismissing the question)
penf glossary alias DC DCS
penf glossary alias API APIS
penf glossary alias VM VMS
```

### Interactive Fallback
For items needing human input:
```bash
# Show specific source context
penf review questions source 26 --context 2000

# After user provides answer
penf review questions resolve 26 "Product Launch Date"
```
