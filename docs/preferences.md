# Penfold User Preferences

This file stores your personal preferences for how Claude assists you with Penfold.
**This file is never overwritten by updates** - it belongs to you.

## Acronym Review Preferences

### Auto-Resolution Settings
<!-- Claude will auto-resolve standard tech acronyms. Add any domain-specific terms you want auto-resolved: -->

```yaml
auto_resolve:
  # Company-specific acronyms Claude can resolve without asking
  # Example: LKE: "Linode Kubernetes Engine"

  # Set to false to always ask before resolving
  auto_resolve_standard_tech: true
```

### Domain Context
<!-- Help Claude understand your domain for better acronym guessing: -->

```yaml
domain:
  company: ""           # e.g., "Akamai/Linode"
  industry: ""          # e.g., "Cloud Infrastructure"
  products: []          # e.g., ["LKE", "Managed Databases", "Object Storage"]
  common_acronyms: {}   # Domain-specific: { "LKE": "Linode Kubernetes Engine" }
```

### Review Style
<!-- How should Claude present acronym reviews? -->

```yaml
review_style:
  # "batch" = analyze all, present summary, ask once
  # "interactive" = ask about uncertain ones one-by-one
  mode: batch

  # Show source context for uncertain acronyms
  show_context: true

  # Confidence threshold (0-1) - below this, ask user
  ask_threshold: 0.7
```

## Search Preferences

```yaml
search:
  # Default result limit
  default_limit: 10

  # Preferred search mode: hybrid, semantic, keyword
  default_mode: hybrid

  # Always expand acronyms in queries
  expand_acronyms: true
```

## Communication Style

```yaml
style:
  # Brief or detailed responses
  verbosity: normal  # brief, normal, detailed

  # Show command examples in responses
  show_commands: true

  # Explain reasoning for auto-resolutions
  explain_reasoning: false
```

---

## Notes

Add any personal notes or context that would help Claude assist you better:

<!-- Example:
- I work on the LKE team, so most cloud acronyms are Linode-specific
- "TER" in my meetings usually means "Technical Execution Review"
- Person initials: AW = Adam W, JB = James Brown
-->
