# Create Design

Create a design shard that meets the pipeline readiness check criteria.

## Arguments: $ARGUMENTS

A brief description of what to design. Can include a parent shard ID (e.g. "Config-driven gating --parent pf-0ef956").

## Instructions

### Step 1: Parse arguments

Extract:
- **Description** — what the design is about
- **--parent pf-XXXXX** — optional parent shard to link to

### Step 2: Gather context

Before writing, understand the problem:
- If James described the problem in conversation, use that context
- If a parent shard exists, read it for outcome context: `cxp shard show <parent-id>`
- Search the codebase for the relevant code: file paths, line numbers, current behaviour
- Check the KB for related designs: `cxp kb search "<topic>"`

### Step 3: Write the design

Follow this structure exactly. Designs that don't meet these criteria will be sent back by the pipeline readiness check.

#### Required Sections

**1. Problem**
What is broken, missing, or painful? Be concrete:
- Reference specific files, functions, or line numbers
- Show current behavior vs desired behavior
- If triggered by an incident or bug, reference the shard ID

Bad: "The notification system is inflexible."
Good: "Notification sender detection is hardcoded in `pipeline.go:412-445` with a switch statement over 8 known senders. Adding a new sender requires a code change and redeploy."

**2. User / Consumer**
Who benefits? End users, developers, operators, or other agents.

**3. Success Criteria**
3-5 measurable, independently testable conditions. An agent should be able to write a test for each one.

Bad: "The system works correctly."
Good:
- "New pipeline stages can be added by creating a single file implementing the `StageExecutor` interface"
- "`go test ./pipeline/...` passes with the new stage registered"

**4. Scope Boundaries**
What is explicitly NOT included. In-scope and deferred/out-of-scope.

**5. Technical Approach**
Pick ONE approach. If alternatives were considered, briefly note why this one was chosen.

Include:
- **Architecture:** How the pieces fit together
- **Code locations:** File paths, package names, function names (with line numbers where helpful)
- **Data model:** Schema changes, new types, modified interfaces (with field names and types)
- **API surface:** New or changed commands, endpoints, function signatures
- **Dependencies:** What must exist first (reference shard IDs)

**6. Migration / Rollout**
- Single PR or phased?
- Backward-compatible?
- Existing data migration needed?
- In-flight work handling?

#### Optional Sections (include when relevant)

- **Edge Cases / Error Handling** — what happens when things go wrong
- **Langfuse Observability** — what gets traced, logged, or reported
- **Dependencies** — other designs that must land first

### Step 4: Implementability self-check

Before creating the shard, verify:

> Could an implementing agent write code from this design without asking any questions?

Common gaps to fix:

| Gap | Fix |
|-----|-----|
| "Somewhere in the pipeline code" | Specific file path and line numbers |
| "We need a new config format" | Show the schema with field names and types |
| "Option A or B" | Pick one and state why |
| "Handle errors appropriately" | Define what "appropriately" means |
| "Should be backward-compatible" | Describe the specific constraint |

If any gap exists, fix it before creating.

### Step 5: Create the shard

```bash
# Write content to a temp file for body-file
cat > /tmp/design.md << 'EOF'
<design content>
EOF

cxp design create "<title>" --body-file /tmp/design.md
```

If a parent was specified:
```bash
cxp shard link <design-id> --child-of <parent-id>
```

### Step 6: Confirm

Show James:
- The shard ID and title
- A 2-3 line summary of what it covers
- Any decisions you made (option selection, scope trade-offs)
- Ask if he wants to adjust anything before it goes through design review

## Title Convention

Format: `<What changes> — <one-line summary>`

Good:
- "Config-driven contribution gating — move skip thresholds to pipeline_definitions"
- "Stage-kind-driven workflow execution — replace sequential branching with config-driven dispatch"

Bad:
- "Fix the pipeline"
- "Improvements to notification handling"
- "Phase 2 of the refactor"

## Key Principles

- **Extend, don't invent.** Prefer adding columns to existing tables over creating new ones. Prefer extending existing patterns over new abstractions.
- **Langfuse is first-class.** Every design that touches pipeline execution should address what gets traced.
- **Config over code.** The goal is a system where new content types need only DB configuration, not Go code.
- **Fail visibly.** Silent fallbacks and swallowed errors are bugs. Missing config should produce clear errors.
