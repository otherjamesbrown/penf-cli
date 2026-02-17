# Penfold Processes

This file lists available workflows for Claude when assisting with Penfold.
Process definitions are in `~/.penf/processes/` or the repo's `context/workflows/`.

## Available Processes

| Process | Trigger | Description |
|---------|---------|-------------|
| [Acronym Review](processes/acronym-review.md) | "review acronyms", pending questions | Resolve unknown acronyms from transcripts |

## Quick Reference

### Acronym Review
```bash
# Get batch context for intelligent processing
penf process acronyms context -o json

# Batch resolve/dismiss
penf process acronyms batch-resolve --dry-run '{"resolutions":[...]}'
```

### Search
```bash
# Hybrid search (default)
penf search "query" -o json

# Semantic only
penf search "query" --semantic -o json
```

### Glossary Management
```bash
# List all terms
penf glossary list -o json

# Add term
penf glossary add TERM "Expansion" --context tag1,tag2

# Add alias for transcription errors
penf glossary alias EXISTING_TERM NEW_ALIAS
```

## Process Locations

- **User preferences**: `~/.penf/preferences.md` (never overwritten)
- **Process definitions**: `~/.penf/processes/` (updated by `penf update`)
- **Built-in workflows**: Run `penf help <command>` for CLI syntax

## Updating Processes

Process files may be updated when you run `penf update`. Your `preferences.md` is never touched.
