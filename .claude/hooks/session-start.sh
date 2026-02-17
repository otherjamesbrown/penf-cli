#!/bin/bash
# SessionStart hook — injects context from Context Palace
#
# Tries `cxp session inject` first (Phase 1 command).
# Falls back to basic session info with existing commands.
# stdout becomes additionalContext in Claude Code.

set -euo pipefail

# Try the proper inject command first (requires Phase 1)
if cxp session inject --tag main 2>/dev/null; then
  exit 0
fi

# Fallback: basic session info with existing commands
SESSION_JSON=$(cxp session show -o json 2>/dev/null) || true

if [ -n "$SESSION_JSON" ]; then
  SESSION_ID=$(echo "$SESSION_JSON" | jq -r '.id // empty' 2>/dev/null)
  SESSION_DATE=$(echo "$SESSION_JSON" | jq -r '.created_at[:10] // empty' 2>/dev/null)
  echo "[Context Palace] Session ${SESSION_ID} (open since ${SESSION_DATE})"
else
  echo "[Context Palace] No active session"
fi
