# Handoff

Save working state before context clears. Supports parallel sessions with optional tags.

## Arguments: $ARGUMENTS

Optional: Tag for parallel work (e.g., `A`, `penfv`). Omit for solo work.

## Instructions

### Step 1: Ensure Session Exists

```bash
cxp session show || cxp session start "Session: $(date +%Y-%m-%d)"
```

### Step 2: Determine Tag

Parse `$ARGUMENTS` for a tag. If provided, use it. If not, no tag.

- Tag provided: checkpoint prefix is `[handoff:TAG]`
- No tag: checkpoint prefix is `[handoff]`

### Step 3: Capture State

What do I need to remember? Be specific and concise:

1. **What** we're working on
2. **Did** what got done this cycle
3. **State** what's working/broken right now
4. **Next** the immediate action when resuming
5. **Found** key discoveries, root causes, decisions (if any)

### Step 4: Save Checkpoint

Write the checkpoint with the tag prefix on the FIRST line:

```bash
cxp session checkpoint "[handoff:TAG] Working on: X. Did: Y. State: Z. Next: W. Found: V."
```

Or without tag:

```bash
cxp session checkpoint "[handoff] Working on: X. Did: Y. State: Z. Next: W."
```

### Step 5: Leave Inbox Alone

**Do NOT read or process inbox messages.** Reading marks them as read, which means /pickup won't see them. The inbox is /pickup's job — that's when the agent can actually act on messages.

If you already know about inbox messages from earlier in this conversation, you can mention them in the checkpoint text. But don't `cxp message read` anything during handoff.

### Step 6: Store Decisions as Memories

If we made decisions or learned lessons this cycle, store them:

```bash
cxp memory add "decision or lesson" --label decision
```

### Step 7: Confirm

If tagged:
```
Saved [TAG]. /pickup TAG to resume.
```

If untagged:
```
Saved. /pickup to resume.
```

Keep it to one line.

## Key Principles

- **Quick** - Working memory dump, not a report
- **Penfold work** - What we're building/testing, not CP state
- **For me, not James** - He knows what we're doing
- **Clear next step** - I should know exactly what to do after /pickup
- **Decisions to memories** - Anything worth remembering long-term goes to CP memory, not just the checkpoint
- **Tag is disposable** - It's just for parallel disambiguation, not a permanent label
