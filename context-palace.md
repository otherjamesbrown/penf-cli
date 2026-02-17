# Context-Palace

A shared memory system for AI agents. Tasks, messages, logs, and data - all in one place.

---

## Quick Reference

```sql
-- Check inbox
SELECT * FROM unread_for('penfold', 'penfold');

-- Inbox summary (for triage)
SELECT * FROM inbox_summary('penfold', 'penfold');

-- Check tasks
SELECT * FROM tasks_for('penfold', 'penfold');

-- Send message
SELECT send_message('penfold', 'penfold', ARRAY['recipient'], 'Subject', 'Body');

-- Reply to message
SELECT send_message('penfold', 'penfold', ARRAY['recipient'], 'Re: Subject', 'Body', NULL, NULL, 'pf-original');

-- Create task
SELECT create_shard('penfold', 'Title', 'Description', 'task', 'penfold');

-- Claim task
SELECT claim_task('pf-xxx', 'penfold');

-- Close task
SELECT close_task('pf-xxx', 'Completed: summary');

-- Add artifact to task (commit, URL, etc.)
SELECT add_artifact('pf-xxx', 'commit', 'abc123', 'Fixed the bug');
SELECT * FROM get_artifacts('pf-xxx');

-- Mark messages read
SELECT mark_read(ARRAY['pf-xxx', 'pf-yyy'], 'penfold');

-- Get thread
SELECT * FROM get_thread('pf-xxx');
```

---

## Common Mistakes

| You might try | Correct name | Notes |
|---------------|--------------|-------|
| `body` | `content` | Column for message/task text |
| `shard_type` | `type` | Column for shard type |
| `issues` table | `shards` table | Use `shards WHERE type='task'` or the `issues` view |
| `tasks` table | `shards` table | Use `shards WHERE type='task'` or the `tasks` view |
| `messages` table | `shards` table | Use `shards WHERE type='message'` or the `messages` view |

**Convenience views exist:** `issues`, `tasks`, `messages`, `logs`, `docs`, `memories`, `sessions`, `backlog` - these filter `shards` by type.

---

## Schema Quick Reference

### shards table
| Column | Type | Notes |
|--------|------|-------|
| id | text | e.g., `pf-a1b2c3` |
| project | text | Project name |
| title | text | Subject/title |
| **content** | text | Body text (NOT `body`) |
| **type** | text | `task`, `message`, `log`, `doc` (NOT `shard_type`) |
| status | text | `open`, `in_progress`, `closed` |
| priority | int | 0=critical, 1=high, 2=normal, 3=low |
| creator | text | Who created it |
| owner | text | Assigned to (for tasks) |
| created_at | timestamptz | When created |
| closed_at | timestamptz | When closed |
| closed_reason | text | Why closed |
| expires_at | timestamptz | Optional expiry (for memories) |
| labels | text[] | Tags like `agent:cli-dev` |

### file_claims table
| Column | Type | Notes |
|--------|------|-------|
| file_path | text | Primary key - the file being claimed |
| shard_id | text | FK to shards - the task claiming it |
| session_id | text | Claude session ID |
| agent_id | text | Agent holding the claim |
| claimed_at | timestamptz | When claimed |
| expires_at | timestamptz | Auto-expires (default 1 hour) |

### Other tables
| Table | Purpose |
|-------|---------|
| `labels` | Tags on shards (shard_id, label) |
| `edges` | Relationships (from_id, to_id, edge_type) |
| `read_receipts` | Read tracking (shard_id, agent_id, read_at) |

---

## Helper Functions

All functions with agent parameters accept shorthand names (e.g., `mycroft` instead of `agent-mycroft`).

| Function | Purpose |
|----------|---------|
| `unread_for(project, agent)` | Your unread messages |
| `inbox_summary(project, agent)` | Triage view: counts by kind, urgent count |
| `tasks_for(project, agent)` | Your assigned open tasks |
| `ready_tasks(project)` | Open tasks not blocked |
| `get_thread(shard_id)` | Conversation thread |
| `send_message(project, sender, recipients[], subject, body, cc[], kind, reply_to)` | Send message with labels/edges |
| `create_shard(project, title, content, type, creator, owner, priority)` | Create any shard |
| `create_task_from(project, creator, source_id, title, desc, priority, owner)` | Task from bug report |
| `claim_task(task_id, agent)` | Atomically claim a task |
| `close_task(task_id, reason)` | Close task with reason |
| `add_artifact(task_id, type, reference, description)` | Attach commit/URL/file to task |
| `get_artifacts(task_id)` | List artifacts for a task |
| `mark_read(shard_ids[], agent)` | Bulk mark as read |
| `mark_all_read(project, agent)` | Clear inbox |
| `link(from, to, type)` | Create edge |
| `add_labels(shard_id, labels[])` | Add multiple labels |
| `memories_for(project, agent)` | Get active memories for agent |
| `expired_memories(project)` | Get memories past expiry |
| `create_memory(project, owner, title, trigger, context_id, expires_at)` | Create memory with optional trigger edge |
| `close_memory(memory_id, resolution)` | Close a triggered memory |
| `current_session(project, agent)` | Get most recent open session |
| `start_session(project, owner, title)` | Start a new session |
| `add_checkpoint(session_id, summary)` | Add checkpoint to session |
| `end_session(session_id, summary)` | Close session with optional summary |
| `close_stale_sessions(project, interval)` | Auto-close inactive sessions (default 24h) |
| `backlog_for(project, agent)` | Get open backlog items for agent |
| `create_backlog_item(project, owner, title, content, priority, depends_on[])` | Create backlog item with dependencies |
| `claim_files(shard_id, session_id, agent_id, files[])` | Atomically claim files for parallel work |
| `release_claims(shard_id)` | Release all file claims for a shard |
| `check_conflicts(files[], my_shard_id)` | Find files claimed by other shards |
| `cleanup_expired_claims()` | Remove expired file claims |
| `extend_claims(shard_id, duration)` | Extend claim expiry (default 1 hour) |
| `create_impl_shard(project, creator, agent_type, title, content, files[], depends_on[], parent_id)` | Create implementation shard with labels, dependencies, and file claims |
| `impl_status(parent_id)` | View all child implementation shards with status |

---

## Connection

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -c "SQL"
```

SSL certificates in `~/.postgresql/` provide authentication.

### Handling Complex Content

For content with backticks, quotes, or special characters, use heredoc + PostgreSQL dollar-quoting:

```bash
psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" <<'EOSQL'
SELECT create_shard('penfold', 'Title', $md$
Content with `backticks` and 'quotes' - no escaping needed.

```code
Even code blocks work fine.
```
$md$, 'task', 'penfold');
EOSQL
```

- `<<'EOSQL'` (single quotes) prevents shell from expanding anything
- `$md$...$md$` is PostgreSQL dollar-quoting - no SQL escaping needed inside

---

## Agent Identity

You are **agent-NAME** working on project **PROJECT** with prefix **pf-**.

Your project rules are in `pf-rules` (fetch with `SELECT content FROM shards WHERE id = 'pf-rules'`).

### Agent Name Shorthand

All functions automatically add the `agent-` prefix if omitted:

```sql
-- These are equivalent:
SELECT send_message('penfold', 'mycroft', ARRAY['cxp'], 'Subject', 'Body');
SELECT send_message('penfold', 'agent-mycroft', ARRAY['agent-cxp'], 'Subject', 'Body');

-- Works everywhere:
SELECT * FROM unread_for('penfold', 'mycroft');
SELECT claim_task('pf-xxx', 'cxp');
SELECT * FROM tasks_for('penfold', 'mycroft');
```

---

## Syncing This File

To get the latest version with your values filled in:

```bash
# Create .palace.yaml in your working directory
cat > .palace.yaml << 'EOF'
project: yourproject
agent: agent-yourname
prefix: xx
EOF

# Run sync script
palace-sync-docs
```

This fetches the latest documentation and replaces template values.

---

## Common Operations

### Check Your Inbox

```sql
SELECT * FROM unread_for('penfold', 'penfold');
```

### Inbox Summary (Triage)

Get a quick overview before diving into individual messages:

```sql
SELECT * FROM inbox_summary('penfold', 'penfold');
```

Returns:
| Column | Description |
|--------|-------------|
| total_unread | Count of unread messages |
| by_kind | JSON object: `{"kind:bug-report": 2, "kind:question": 1}` |
| urgent_count | Messages with priority 0 or 1 |
| oldest_unread | Timestamp of oldest unread |

### Read Full Message

```sql
SELECT * FROM shards WHERE id = 'pf-xxx';
-- Or use the view:
SELECT * FROM messages WHERE id = 'pf-xxx';
```

### Mark as Read

```sql
-- Single
SELECT mark_read(ARRAY['pf-xxx'], 'penfold');

-- Multiple
SELECT mark_read(ARRAY['pf-xxx', 'pf-yyy'], 'penfold');

-- Clear inbox
SELECT mark_all_read('penfold', 'penfold');
```

### Send a Message

```sql
-- Simple
SELECT send_message('penfold', 'penfold', ARRAY['recipient'], 'Subject', 'Body text');

-- With CC and kind
SELECT send_message('penfold', 'penfold',
  ARRAY['recipient'],
  'Subject', 'Body text',
  ARRAY['cc-agent'],    -- cc
  'bug-report'          -- kind
);
```

### Reply to a Message

```sql
SELECT send_message('penfold', 'penfold',
  ARRAY['original-sender'],
  'Re: Subject', 'Reply text',
  NULL,                 -- cc
  'ack',                -- kind
  'pf-ORIGINAL'     -- reply_to (creates edge, marks original read)
);
```

### Get Conversation Thread

```sql
SELECT * FROM get_thread('pf-ROOT-MESSAGE');
```

Returns root message + all replies, ordered by depth then time.

### Check Your Tasks

```sql
SELECT * FROM tasks_for('penfold', 'penfold');
```

### Find Claimable Tasks

```sql
SELECT * FROM ready_tasks('penfold') WHERE owner IS NULL;
```

### Claim a Task

```sql
SELECT claim_task('pf-xxx', 'penfold');
-- Returns true if claimed, false if already taken
```

### Close a Task

```sql
SELECT close_task('pf-xxx', 'Completed: summary of what was done');
```

### Add Artifacts to a Task

Track what you did - commits, deployments, related shards, URLs:

```sql
-- Add artifacts
SELECT add_artifact('pf-xxx', 'commit', 'abc123def', 'Fixed null pointer bug');
SELECT add_artifact('pf-xxx', 'url', 'https://github.com/org/repo/pull/42', 'PR link');
SELECT add_artifact('pf-xxx', 'shard', 'pf-yyy', 'Related bug report');
SELECT add_artifact('pf-xxx', 'deploy', 'prod-2026-01-31', 'Deployed to production');

-- View artifacts
SELECT * FROM get_artifacts('pf-xxx');
```

Artifact types: `commit`, `url`, `shard`, `file`, `deploy` (or any string).

### Create a Task

```sql
-- Simple
SELECT create_shard('penfold', 'Task title', 'Description', 'task', 'penfold');

-- With owner and priority
SELECT create_shard('penfold', 'Task title', 'Description', 'task', 'penfold', 'target-agent', 2);
```

### Create Task from Bug Report

```sql
SELECT create_task_from(
  'penfold',
  'penfold',
  'pf-BUG-MESSAGE',    -- source
  'fix: Bug title',
  'Description',
  1,                       -- priority
  'agent-to-assign'        -- owner
);
```

This auto-links to source, copies labels, and closes the source message.

---

## Labels

### Recipients
- `to:agent-backend` - Send to agent
- `cc:agent-cli` - Copy to agent

### Message Kinds
- `kind:bug-report`
- `kind:feature-request`
- `kind:question`
- `kind:status-update`

### Task Routing
- `for:backend` - Backend agent should take
- `for:frontend` - Frontend agent should take

### Components
- `backend`, `frontend`, `database`, `infra`

### Agent Types (for implementation shards)
- `agent:cli-dev`, `agent:service-dev`, `agent:worker-dev`, `agent:data-dev`, `agent:ai-dev`

---

## Edge Types

| Edge | Meaning |
|------|---------|
| `replies-to` | Message reply |
| `relates-to` | Loose association |
| `discovered-from` | Created from source |
| `blocks` | Dependency |
| `blocked-by` | Blocked by dependency |
| `has-artifact` | Work artifact (commit, URL, etc.) - metadata contains details |
| `triggered-by` | Memory triggered by context |
| `depends-on` | Backlog item dependency |

---

## Synchronous Conversations (poll_hint)

For real-time back-and-forth, use `sync:true` label and poll_hint protocol.

### Message Format

Include JSON frontmatter in content:

```
{
  "poll_hint": "continue",
  "type": "question",
  "session": "abc-123"
}

Your message here...
```

### poll_hint Values

| Value | Meaning |
|-------|---------|
| `continue` | Keep polling |
| `done` | Conversation complete |
| `pause` | Sleep then resume |
| `typing` | Still composing |

### Sending Sync Message

```sql
SELECT send_message('penfold', 'penfold', ARRAY['recipient'], 'Subject',
  $body${
  "poll_hint": "continue",
  "type": "question",
  "session": "sess-123"
}

Your question here
$body$
);
SELECT add_labels('pf-NEWID', ARRAY['sync:true', 'sync:session-123']);
```

---

## Session Workflow

```
1. CHECK INBOX     SELECT * FROM unread_for(...)
2. PROCESS         Read, mark read, reply/create tasks
3. CHECK TASKS     SELECT * FROM tasks_for(...)
4. CLAIM/WORK      claim_task() then do the work
5. COMPLETE        close_task() with summary
6. REPEAT
```

---

## Memory, Session & Backlog

### Memories

Memories are things to remember across sessions - reminders, pending actions, context.

```sql
-- Create a memory with trigger condition
SELECT create_memory('penfold', 'penfold',
  'Delete test data when content delete available',
  'content delete implemented',  -- trigger condition
  'pf-context-id',           -- optional context
  NOW() + INTERVAL '7 days'      -- optional expiry
);

-- Check your memories
SELECT * FROM memories_for('penfold', 'penfold');

-- Close a memory when triggered
SELECT close_memory('pf-xxx', 'Done: deleted test data');

-- Find expired memories (for cleanup)
SELECT * FROM expired_memories('penfold');
```

### Sessions

Sessions track work with checkpoints.

```sql
-- Start a session
SELECT start_session('penfold', 'penfold', 'Working on feature X');

-- Add checkpoints as you work
SELECT add_checkpoint('pf-session-id', 'Completed auth module');
SELECT add_checkpoint('pf-session-id', 'Fixed TLS bugs');

-- Get current session
SELECT * FROM current_session('penfold', 'penfold');

-- End session
SELECT end_session('pf-session-id', 'Feature X complete');

-- Auto-close stale sessions (run periodically)
SELECT close_stale_sessions('penfold', '24 hours');
```

### Backlog

Backlog items are development work items with dependencies.

```sql
-- Create backlog item
SELECT create_backlog_item('penfold', 'penfold',
  'Implement caching layer',
  'Add Redis caching for API responses',
  2,                              -- priority
  ARRAY['pf-dependency-id']   -- depends on
);

-- Get your backlog
SELECT * FROM backlog_for('penfold', 'penfold');
```

### File Claims (Multi-Agent Coordination)

Prevent conflicts when multiple agents work in parallel:

```sql
-- Claim files before editing
SELECT claim_files('pf-task', 'session-123', 'penfold',
  ARRAY['cmd/app/main.go', 'pkg/service/handler.go']);

-- Check if files are available
SELECT * FROM check_conflicts(
  ARRAY['pkg/service/handler.go', 'pkg/service/new.go'],
  'pf-my-task'
);

-- Extend claims for long-running work
SELECT extend_claims('pf-task', interval '2 hours');

-- Release when done (auto-called by close_task)
SELECT release_claims('pf-task');

-- Cleanup expired claims
SELECT cleanup_expired_claims();
```

Claims auto-expire after 1 hour and are auto-released when `close_task()` is called.

### Implementation Workflow

Create coordinated implementation shards with dependencies:

```sql
-- Create implementation shard with file claims and dependencies
SELECT create_impl_shard(
  'penfold',
  'penfold',
  'cli-dev',                          -- agent_type label
  'Implement pipeline command',
  'Task description...',
  ARRAY['cmd/app/pipeline.go'],       -- files to claim
  ARRAY['pf-schema-task'],        -- blocked by these
  'pf-parent-feature'             -- parent shard
);

-- Check implementation progress
SELECT * FROM impl_status('pf-parent-feature');
-- Returns: id, title, status, owner, agent_type, blocked_by[], files[]
```

Agent types: `cli-dev`, `service-dev`, `worker-dev`, `data-dev`, `ai-dev`

---

## Priorities

| Priority | Meaning |
|----------|---------|
| 0 | Critical - drop everything |
| 1 | High - do today |
| 2 | Normal - this week |
| 3 | Low - when possible |

---

## Palace CLI (for Sub-Agents)

Sub-agents can use the `palace` CLI instead of raw SQL for task operations:

```bash
palace task get pf-xxx        # Get task details
palace task claim pf-xxx      # Claim a task
palace task progress pf-xxx "note"   # Log progress
palace task close pf-xxx "summary"   # Close task
palace artifact add pf-xxx commit abc123 "description"
```

Configure via environment or `~/.palace.yaml`:
```bash
export PALACE_USER=penfold
export PALACE_AGENT=agent-NAME
```

See `palace-cli.md` for full documentation.

---

## Reporting Issues

Context-Palace is maintained by **agent-cxp**. Report bugs:

```sql
SELECT send_message('penfold', 'agent-YOURNAME',
  ARRAY['agent-cxp'],
  'Bug: Description',
  'Details...',
  NULL, 'bug-report'
);
```
