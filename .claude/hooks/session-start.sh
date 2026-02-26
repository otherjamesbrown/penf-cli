#!/bin/bash
# SessionStart hook — injects context from Context Palace
# No session required. Board, playbook, and focus shards work independently.
# stdout becomes additionalContext in Claude Code.

set -euo pipefail

# Instance identity (first 8 chars of conversation UUID)
PROJ_DIR="$HOME/.claude/projects/-Users-james-github-otherjamesbrown-penf-cli"
INSTANCE_ID=$(basename "$(ls -t "$PROJ_DIR"/*.jsonl 2>/dev/null | head -1)" .jsonl 2>/dev/null | cut -c1-8)
export CLAUDE_SESSION_ID="penfold:${INSTANCE_ID:-unknown}"
echo "[Instance: ${CLAUDE_SESSION_ID}]"

# Board
echo ""
cxp session board 2>/dev/null || true

# Playbook
echo ""
echo "# Playbook #"
echo ""
cxp knowledge show pf-34494b 2>/dev/null || true

# Phased work
PHASED_PARENTS=$(cxp shard list --label phased -o json 2>/dev/null | jq -r '.results[]?.id // empty' 2>/dev/null)
if [ -n "$PHASED_PARENTS" ]; then
  echo ""
  echo "# Phased Work #"
  for pid in $PHASED_PARENTS; do
    ptitle=$(cxp shard list --label phased -o json 2>/dev/null | jq -r ".results[] | select(.id == \"$pid\") | .title")
    echo ""
    echo "  $ptitle ($pid)"
    # List children sorted by phase label
    cxp shard edges "$pid" -o json 2>/dev/null | jq -r '
      [ .[] | select(.direction == "incoming" and .edge_type == "child-of") ]
      | sort_by(.title)
      | .[]
      | "    \(.status | if . == "in_progress" then "▶" elif . == "open" then "◻" elif . == "closed" then "✓" else "?" end)  \(.title) (\(.shard_id)) [\(.status)]"
    ' 2>/dev/null
  done
  echo ""
fi

# Last handoff from session ledger
HANDOFF=$(penf ledger list --type handoff --limit 1 -o json 2>/dev/null | jq -r '.entries[0] // empty' 2>/dev/null)
if [ -n "$HANDOFF" ] && [ "$HANDOFF" != "null" ]; then
  echo ""
  echo "# Last Handoff #"
  echo "$HANDOFF" | jq -r '"  \(.title) (\(.session_id)) — \(.created_at.seconds | todate)"' 2>/dev/null || true
fi

# Focus shard details
FOCUS_IDS=$(cxp session board -o json 2>/dev/null | jq -r '.focus[]?.id // empty' 2>/dev/null)
if [ -n "$FOCUS_IDS" ]; then
  echo ""
  echo "# Focus Shard Details #"
  for id in $FOCUS_IDS; do
    echo ""
    cxp shard show "$id" 2>/dev/null || true
  done
fi
