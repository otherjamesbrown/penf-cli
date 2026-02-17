# Session End

Consolidate today's **Penfold development work** into a handoff for future session-start.

## Arguments: $ARGUMENTS

Optional: Brief note (e.g., "end of day", "switching projects")

## Instructions

### Step 1: Load Current Session

```bash
cxp session show
```

### Step 2: Check Current Penfold State

Capture where things are now:

```bash
penf status
penf health
```

### Step 3: Store Decisions and Lessons

Review the conversation for decisions, lessons learned, or things worth remembering.
Store each one:

```bash
cxp memory add "description" --label decision
cxp memory add "description" --label lesson
```

### Step 4: Create Handoff

```bash
cxp session checkpoint "END OF SESSION. Working on: [Penfold feature/issue]. Did: [what got done today]. State: [what's working/broken]. Blocked: [what we're waiting on]. Next: [priorities for next session]."
```

### Step 5: End Session

```bash
cxp session end
```

### Step 6: Git Reminder

```
Session ended.

Before closing:
  git status && git add <files> && git commit -m "..." && git push

Tomorrow: /session-start
```

## Key Principles

- **Penfold is the subject** - What we built, tested, debugged in the actual product
- **Include actual state** - Run `penf` commands and capture output
- **Decisions to memories** - Don't let decisions die with the session. Use `cxp memory add`.
- **Track paused work** - Note what we set aside, not just current task
- **Context-Palace is just the envelope** - The handoff is stored there, but it's about Penfold
