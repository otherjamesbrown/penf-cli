---
description: "Review completed work from mycroft and steve. Check needs-review shards, verify with evidence, close or send back."
---

# Review

Check for work that's ready for verification from mycroft and steve.

## Arguments: $ARGUMENTS

Optional: specific shard ID to review (e.g. `pf-1a7a98`). If not provided, show all needs-review items.

## Instructions

### Step 1: Find Review Items

If a specific shard ID was given in $ARGUMENTS, skip to Step 3.

Otherwise, query for all needs-review shards:

```bash
cxp shard list --status needs-review -o json
```

### Step 2: Present Table

Show all needs-review items as a table. Determine the owner to show which agent completed the work:

| # | ID | Type | Agent | Title |
|---|-----|------|-------|-------|
| 1 | pf-xxx | bug | mycroft | ... |
| 2 | pf-yyy | task | steve | ... |

Then offer options:

1. Review all items
2. Pick specific items (list numbers)
3. Review mycroft's items only
4. Review steve's items only

**If no items found:** "Nothing to review. Board is clear." Stop.

Wait for James to choose before continuing.

### Step 3: Review Each Item

For each shard being reviewed:

1. **Read the shard** to understand what was requested and what was done:
   ```bash
   cxp shard show <id>
   ```

2. **Check the evidence.** The shard content should include:
   - Test results (actual stdout, not just "tests pass")
   - Files modified/created
   - For pipeline changes: before/after output
   - For CLI changes: command output showing it works
   - Deployment: commit hash + verification

3. **Verify independently.** Don't just trust the evidence — spot-check:
   - For code changes: pull latest, read the diff, run `go test ./...`
   - For CLI changes: try the command yourself
   - For bug fixes: run the original repro steps
   - For deploys: check the running version

4. **Present findings to James:**
   > **pf-xxx** (mycroft) — Pipeline/Tracing fix
   > Evidence: tests pass, commit abc123, verified on dev02
   > Recommendation: Close / Send back (reason)

### Step 4: Act on James's Decision

**Close:**
```bash
cxp shard close <id>
```

**Send back:**
```bash
cxp shard status <id> open && cxp shard status <id> ready
```
Then append review feedback to the shard body via `cxp shard update`.

### Step 5: Check Phase Completion

After closing a shard, check if it's part of a phased effort:

1. **Check edges** — does the closed shard have a `child-of` edge to a parent with a Phases table?
   ```bash
   cxp shard edges <id>
   ```

2. **If parent has phases** — read the parent shard and check its phase table. Are all shards in the current phase now closed?

3. **If phase is complete** — unblock the next phase:
   - Set next phase's shards to `ready`: `cxp shard status <id> ready`
   - Update the parent's phase table (phase status: `in progress` → `complete`, next phase: `blocked` → `ready`)
   - Tell James: "Phase N complete. Unblocked Phase N+1: [description]."

4. **If phase is not complete** — skip silently.

### Step 6: Summary

After all items reviewed:

> Reviewed N items: X closed, Y sent back.

## Key Principles

- **Evidence required** — no evidence in the shard = send back, don't guess
- **Verify independently** — spot-check at least one claim per shard
- **James decides** — present findings and recommendation, let James close or reject
- **One at a time** — don't batch-close, review each individually
- **Quick for clean work** — if evidence is solid, don't drag it out
