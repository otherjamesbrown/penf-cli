# Remember

Store something in Context Palace memory. Persists across sessions, searchable with `cxp recall`.

## Arguments: $ARGUMENTS

Required: The thing to remember. Optionally include labels after the content.

## Instructions

### Step 1: Parse Arguments

Extract:
- **Content**: The thing to remember
- **Labels**: Topic tags (e.g., decision, lesson, process, tooling, deployment)
- **References**: Any shard IDs mentioned (pf-xxx)

### Step 2: Store in Context Palace

```bash
cxp memory add "<content>" --label <labels>
```

If references to shards are mentioned:
```bash
cxp memory add "<content>" --label <labels> --references <shard-ids>
```

### Step 3: Confirm

```
Remembered: <brief summary>
Labels: <labels>
ID: <memory-id>

Search later: cxp recall "<topic>"
```

## Examples

```
/remember We decided against child shards for observability — too much context overhead
→ cxp memory add "..." --label decision,observability

/remember Nomad deploys are unreliable, always verify version after deploy
→ cxp memory add "..." --label deployment,lesson

/remember Fix agent discipline issues before feature work
→ cxp memory add "..." --label process,decision
```

## Label Conventions

| Label | Use For |
|-------|---------|
| `decision` | Design/architecture decisions with rationale |
| `lesson` | Things that went wrong and what we learned |
| `process` | How we work, workflow decisions |
| `tooling` | CLI, config, infrastructure decisions |
| `deployment` | Deploy/release lessons |
| `spec-writing` | Spec quality, review process |
| `backend-gaps` | What penfold needs from the backend |
| `system-state` | Current state snapshots |
| `status` | Point-in-time status updates |
