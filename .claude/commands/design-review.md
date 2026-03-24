# Design Review

Review a design for CoBuild pipeline readiness. Pre-flight check before submission.

## Arguments: $ARGUMENTS

The design work item ID to review (e.g. pf-XXXXX).

## Instructions

### Step 1: Run the review

Launch the design-reviewer sub-agent with the provided ID.

```
Use the Agent tool with subagent_type "general-purpose" and the following prompt:

You are running a design review. Follow the instructions in .claude/agents/design-reviewer.md exactly.

The design work item to review is: $ARGUMENTS

Read .claude/agents/design-reviewer.md first, then execute all steps.
```

### Step 2: Present findings

Show the developer the agent's report:
- Verdict (READY FOR PIPELINE / NOT READY / REWORK DESIGN)
- Any CRITICAL or HIGH findings as a table
- What needs fixing before submission

### Step 3: If NOT READY

List the specific fixes needed. The developer addresses them and runs `/design-review <id>` again.

Do NOT create child tasks — that happens after the design enters the pipeline.
Do NOT fix the design yourself — report findings for the developer.

### Step 4: If READY FOR PIPELINE

> This design passes all readiness criteria.
>
> Submit to the CoBuild pipeline? This will:
> 1. Initialize the pipeline (`cobuild init <design-id>`)
> 2. Run the formal readiness gate (audit trail)
> 3. Decompose into tasks automatically
> 4. Dispatch agents to implement
>
> Submit? (yes/no)

If yes:
```bash
cobuild init <design-id>
```

### Step 5: If REWORK DESIGN

The design needs fundamental changes. Present findings and stop — the developer needs to rethink the approach.
