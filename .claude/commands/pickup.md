# Pickup

Resume from a handoff shard. Read it, restore context, close it.

## Arguments: $ARGUMENTS

Ignored.

## Instructions

### Step 1: Find Handoff

First check the session ledger:

```bash
penf ledger list --type handoff --limit 1 -o json
```

If a recent handoff is found in the ledger, use it. Otherwise fall back to CXP:

```bash
cxp shard list --type handoff --status open
```

**If none found in either:** "No handoff to pick up." Stop.

**If one found:** Load it directly.

**If multiple found:** List them with titles and ages, ask James which one. Wait for answer before continuing.

### Step 2: Read and Restore

```bash
cxp shard show <shard-id>
```

Extract the working state: what we're doing, what's done, what's next.

### Step 3: Close the Handoff Shard

It's been consumed. Close it so it doesn't show up again.

```bash
cxp shard close <shard-id>
```

### Step 4: Check Board

The session board is already injected by the startup hook. Glance at it for changes since the handoff:

- Shards with status `needs-review` → mycroft finished something, flag to James
- Shards with label `blocked` → mycroft needs guidance, flag to James
- Otherwise move on silently

### Step 5: Resume Quietly

**Don't explain to James.** He knows what we're doing.

Good:
> "Picked up. Continuing with enrichment investigation."

Bad:
> "I've loaded the checkpoint. We were working on X. The state is Y..."

Only speak up if something changed since the handoff:

> "Picked up. Note: pf-860f83 needs review — mycroft fixed the trace ID bug. Continue?"

## Key Principles

- **Read and close** — handoff shards are temporary memory, consumed on pickup
- **Silent load** — don't narrate the restore process
- **Quick confirmation** — signal readiness in one line
- **Flag board changes** — needs-review and blocked items, not bookkeeping
