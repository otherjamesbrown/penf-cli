# Handoff

Save working state to a temporary shard before context clears.

## Arguments: $ARGUMENTS

Optional free-text hint from James about what to capture. E.g. "remember we need to rerun that test" or "we were debugging the timeout issue". Use this to guide what you emphasise, but you already know the full context from the conversation — don't limit yourself to just what's mentioned.

## Instructions

### Step 1: Capture State

You know what we've been doing. Write it out — be specific and concise:

1. **Working on** — what we're doing right now
2. **Did** — what got done this cycle
3. **State** — what's working/broken right now
4. **Next** — the immediate action when resuming (pay attention to $ARGUMENTS for hints here)
5. **Found** — key discoveries, root causes, decisions (if any)

### Step 2: Create Handoff Shard

Create a new shard with type `handoff`. Title should be a short description of the work.

```bash
cxp shard create --type handoff --title "Handoff - [short description]" --body "[state from step 1]" --label handoff
```

### Step 3: Store Decisions as Memories

If we made decisions or learned lessons this cycle, store them:

```bash
cxp memory add "decision or lesson" --label decision
```

### Step 4: Confirm

```
Saved. /pickup to resume.
```

Keep it to one line.

## Key Principles

- **Quick** — working memory dump, not a report
- **One shard per handoff** — don't append, create fresh
- **Temporary** — pickup will read it and close it
- **For me, not James** — he knows what we're doing
- **Clear next step** — I should know exactly what to do after /pickup
