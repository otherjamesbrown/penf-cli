# Recap

Morning briefing. Show what's in flight, what changed, what needs attention.

## Arguments: $ARGUMENTS

None expected.

## Instructions

### Step 1: Session Housekeeping

```bash
cxp session show || true
```

If there's an open session from a previous day, close it and start a new one:

```bash
cxp session end
cxp session start "Session: $(date +%Y-%m-%d)"
```

If no session exists, start one:

```bash
cxp session start "Session: $(date +%Y-%m-%d)"
```

If there's an open session from today, keep it.

### Step 2: Recent Handoffs

Load the previous session(s) to find recent handoffs. Parse for `[handoff...]` entries.

Display them as a board:

```
Active work:
  yesterday 17:20 [A] email threading — mycroft implementing, waiting on Wave 1
  yesterday 16:31      penfv enhancements — review input, refresh key done
  yesterday 15:46      context review — cleanup complete
```

Show the last 3-5 handoffs. Include tag if present, one-line summary, and current status if known.

### Step 3: Process Inbox

```bash
cxp message inbox
```

Summarize what's waiting:
- Resolutions (work completed by others)
- Reviews needed
- Progress updates

Read each message and process it. Don't just list counts.

### Step 4: System Health

```bash
penf status
penf health
ssh dev02 'nomad status'
```

Only mention if something is wrong. If everything is healthy, skip this section.

### Step 5: Check for Overnight Changes

Look for signs that mycroft or other agents did work:

```bash
cxp memory search "deploy"
cxp memory search "release"
```

Note any new deployments, fixes, or changes since last session.

### Step 6: Summarize for James

**Time-framed, Penfold-focused.** What matters:

- What's in flight (from the handoff board)
- What landed overnight (inbox + deploy memories)
- What's blocked or needs a decision
- What the natural next step is

Good:
> "Yesterday we shipped review input in penfv and handed off email threading + source classification to mycroft. Threading is in progress (3 waves, Wave 1 first). Glossary is clean, 21 real terms. MTC keyword reprocessing still pending.
>
> Inbox: mycroft sent a decomposition plan for source classification. Nothing needs sign-off.
>
> Pick up where you left off, or start something new?"

Bad:
> "Session pf-5264da has 12 checkpoints. You have 1 unread message. The system is healthy."

### Step 7: Offer Options

Based on what's in flight, suggest 2-3 concrete next steps:

```
Want to:
1. Check on mycroft's threading progress?
2. Do the MTC keyword reprocess?
3. Something else?
```

## Key Principles

- **This is for James** - Unlike /pickup (which is for me), /recap is a briefing
- **Penfold-focused** - Features, pipeline, data — not CP bookkeeping
- **Time-framed** - "Yesterday...", "Since Friday..."
- **Actionable** - End with concrete options
- **Brief on good news, loud on bad** - Healthy system = skip it. Crashed job = lead with it.
