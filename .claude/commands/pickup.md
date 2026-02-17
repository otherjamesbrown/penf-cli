# Pickup

Restore working state after context clear. Supports parallel sessions with optional tags.

## Arguments: $ARGUMENTS

Optional: Tag to restore (e.g., `A`, `penfv`). Omit to choose from recent handoffs.

## Instructions

### Step 1: Load Session

```bash
cxp session show
```

### Step 2: Find the Right Checkpoint

Parse the session output for lines starting with `[handoff...]`.

**If a tag was provided** (`$ARGUMENTS` is not empty):
- Find the LATEST checkpoint matching `[handoff:TAG]`
- Restore from it directly

**If no tag was provided:**
- Extract all recent `[handoff...]` entries (tagged and untagged)
- If only ONE recent handoff exists: show it and ask "This one?" before restoring
- If MULTIPLE exist: list them with timestamps and one-line summaries, ask which one

Display format for the list:
```
Recent handoffs:
  14:30 [A] penfv tab rendering
  14:25 [B] email threading
  13:00     glossary cleanup
Which one?
```

Wait for James to pick before continuing.

### Step 2.5: Load Playbook

If you haven't already read your playbook this session, do it now before continuing:

```bash
cxp knowledge show penfold-playbook
```

### Step 3: Restore Context

From the matched checkpoint, extract:

1. **What we're working on** - Penfold feature/issue
2. **What's done** - Already completed
3. **Current state** - What's working/broken
4. **Next step** - My immediate action
5. **Key context** - Decisions, blockers

Also load relevant memories:

```bash
cxp memory search "status"
```

### Step 4: Process Inbox

```bash
cxp message inbox
```

If there are unread messages:
1. Read **resolutions** first — these close out work items
2. Read **pre-deploy reviews** — need sign-off
3. Skim acks/progress for anything unexpected
4. Mark all as read

**Flag to James** anything that needs a decision or changed something unexpected.

If inbox is empty, move on silently.

### Step 5: Quick Health Check

```bash
penf status
penf health
ssh dev02 'nomad status'
```

**Flag immediately** if any Nomad job is not `running`.

### Step 6: Resume Quietly

**Don't explain to James.** He knows what we're doing.

Good:
> "Picked up. Continuing with enrichment investigation."

Bad:
> "I've loaded the checkpoint. We were working on X. The state is Y..."

### Step 7: Flag Changes Only

Only speak up if something changed since the handoff:

> "Picked up. Note: 2 new messages from mycroft — threading Wave 1 deployed. Continue?"

## Key Principles

- **Ask if ambiguous** - No tag = show options, don't guess
- **Silent load** - Don't narrate the restore process
- **Quick confirmation** - Signal readiness in one line
- **Flag actual changes** - In Penfold state, not CP bookkeeping
- **Health check** - Catch crashes early, especially cost-burning ones
