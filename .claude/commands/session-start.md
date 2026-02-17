# Session Start (Retired)

**This command is replaced by automatic hooks + `/recap`.**

- **Session context** is now injected automatically by the SessionStart hook on startup/resume
- **Morning briefing** (health checks, inbox, handoff board) → use `/recap`
- **Resume specific work** → use `/pickup [tag]`

The hook lives in `.claude/settings.json` and calls `cxp session inject` on every startup/resume.
