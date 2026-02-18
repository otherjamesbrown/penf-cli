#!/bin/bash
# SessionStart hook — injects context from Context Palace
#
# Tries `cxp session inject` first (Phase 1 command).
# Falls back to basic session info with existing commands.
# stdout becomes additionalContext in Claude Code.

set -euo pipefail

# Session context: inject + board + focus shard details
cxp session inject --tag main 2>/dev/null && {
  echo ""
  cxp session board 2>/dev/null || true

  # Load playbook
  echo ""
  echo "# Playbook #"
  echo ""
  cxp knowledge show penfold-playbook 2>/dev/null || true

  # Fetch full content of focus shards
  FOCUS_IDS=$(cxp session board -o json 2>/dev/null | jq -r '.focus[]?.id // empty' 2>/dev/null)
  if [ -n "$FOCUS_IDS" ]; then
    echo ""
    echo "# Focus Shard Details #"
    for id in $FOCUS_IDS; do
      echo ""
      cxp shard show "$id" 2>/dev/null || true
    done
  fi

  exit 0
}

# Fallback: basic session info with existing commands
SESSION_JSON=$(cxp session show -o json 2>/dev/null) || true

if [ -n "$SESSION_JSON" ]; then
  SESSION_ID=$(echo "$SESSION_JSON" | jq -r '.id // empty' 2>/dev/null)
  SESSION_DATE=$(echo "$SESSION_JSON" | jq -r '.created_at[:10] // empty' 2>/dev/null)
  echo "[Context Palace] Session ${SESSION_ID} (open since ${SESSION_DATE})"
else
  echo "[Context Palace] No active session"
fi
