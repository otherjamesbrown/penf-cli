# Penfold Processes

Quick reference for common workflows. Full details in `workflows/` directory.

## Available Workflows

| Workflow | Trigger | Guide |
|----------|---------|-------|
| Acronym Review | "review acronyms", pending questions | [workflows/acronym-review.md](workflows/acronym-review.md) |
| Mention Review | Ambiguous person references | [workflows/mention-review.md](workflows/mention-review.md) |
| Init Entities | Before first import | [workflows/init-entities.md](workflows/init-entities.md) |
| Onboarding | After content import | [workflows/onboarding.md](workflows/onboarding.md) |

## Quick Commands

```bash
# Acronym review
penf process acronyms context -o json
penf process acronyms batch-resolve '{"resolutions":[...]}'

# Search
penf search "query" -o json

# Glossary
penf glossary list -o json
penf glossary add TERM "Expansion"
```

## File Locations

- **User preferences**: `preferences.md` (never overwritten)
- **Workflow guides**: `workflows/`
- **CLI help**: `penf help <command>`
