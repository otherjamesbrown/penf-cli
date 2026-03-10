# Design Review

Run a structured design review before dispatching work to mycroft or steve.

## Arguments: $ARGUMENTS

The design shard ID to review (e.g. pf-XXXXX).

## Instructions

### Step 1: Run the review

Launch the design-reviewer sub-agent with the provided shard ID.

```
Use the Agent tool with subagent_type "general-purpose" and the following prompt:

You are running a design review. Follow the instructions in .claude/agents/design-reviewer.md exactly.

The design shard to review is: $ARGUMENTS

Read .claude/agents/design-reviewer.md first, then execute all steps.
```

### Step 2: Present findings

Show James the agent's report summary:
- Verdict (DISPATCH / FIX AND RE-REVIEW / REWORK DESIGN)
- Any CRITICAL or HIGH findings as a table
- Metrics (criteria coverage, task count, open questions)

### Step 3: Fix-and-iterate loop

If the verdict is **FIX AND RE-REVIEW**:

1. Fix all CRITICAL and HIGH findings yourself:
   - Research the codebase as needed (use Explore sub-agents for unknowns)
   - Update the design shard content via `cxp shard update`
   - Create missing child tasks via `cxp task create`
   - Add missing edges via `cxp shard link`
   - Resolve open questions with concrete answers

2. Re-run the review sub-agent (go back to Step 1)

3. Repeat until the verdict is **DISPATCH** (no CRITICAL or HIGH findings remain)

4. Present the final clean report to James with a summary of what was fixed across iterations.

If the verdict is **REWORK DESIGN**, stop and present findings to James — the design needs fundamental changes that require his input.

### Step 4: DISPATCH verdict reached

Once clean, tell James the design is ready to dispatch and list the child tasks with their assignments.

## Task sizing rule

The review agent checks this, but also verify yourself: **no single task should require changes to more than 5 files or span more than ~300 lines of new code.** If a task is too large, split it. Agents that receive oversized tasks tend to fail on the first pass, wasting cycles. Prefer 2-3 small focused tasks over 1 large one.
