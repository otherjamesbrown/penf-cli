# Context Palace Setup

How to get Context Palace working in a new project or agent instance.

---

## Accessing Data

All Context Palace operations go through the `cxp` CLI.

```bash
cxp message inbox
cxp shard create --type task "Fix timeout bug" "Details here"
cxp recall "pipeline errors"
cxp -o json shard list --status open
```

If `cxp` doesn't have a command for what you need, **report it as a gap** to mycroft — don't drop to raw SQL.

---

## Prerequisites

1. **PostgreSQL SSL certificates** in `~/.postgresql/`:
   - `postgresql.crt` — client certificate
   - `postgresql.key` — client private key (must be chmod 600)
   - `root.crt` — CA certificate

   Get these from the secrets repository: `~/github/otherjamesbrown/secrets/`

2. **Network access** to `dev02.brown.chat` on port 5432

---

## Step 1: Install the Binary

The `cxp` binary lives in this directory. If your project directory is different, copy it:

```bash
/bin/cp /path/to/penf-cli/cxp /your/project/dir/cxp
chmod +x /your/project/dir/cxp
```

Add the directory to your PATH, or use an `.envrc`:

```bash
# .envrc
export PATH="$PWD:$PATH"
```

Verify:

```bash
cxp version
# => cxp version 0.1.0
```

---

## Step 2: Create Global Config

Create `~/.cp/config.yaml` with your connection details and agent identity:

```bash
mkdir -p ~/.cp
```

```yaml
# ~/.cp/config.yaml
connection:
  host: dev02.brown.chat
  database: contextpalace
  user: penfold
  sslmode: verify-full

agent: agent-YOURNAME
```

**Required fields:**
- `connection.user` — database user (usually `penfold`)
- `agent` — your agent identity (e.g., `agent-mycroft`, `agent-frontend`)

**Optional fields for semantic search:**

```yaml
embedding:
  provider: google
  model: gemini-embedding-001
  api_key_env: GOOGLE_API_KEY

generation:
  provider: google
  model: gemini-2.0-flash
  api_key_env: GOOGLE_API_KEY
```

These require a `GOOGLE_API_KEY` environment variable. Without them, `cxp recall` and summary generation won't work, but everything else will.

---

## Step 3: Create Project Config

In your working directory, create `.cp.yaml`:

```yaml
# .cp.yaml — project-level config
project: yourproject
```

This tells `cxp` which project namespace to use. The `cxp` CLI walks up the directory tree looking for `.cp.yaml`, so one file covers the whole project.

You can also set the agent here if this directory is single-agent:

```yaml
project: yourproject
agent: agent-yourname
```

Config precedence: **env vars > .cp.yaml > ~/.cp/config.yaml > defaults**

---

## Step 4: Test Connection

```bash
cxp status
```

Expected output:

```
Context Palace
  Host:     dev02.brown.chat
  Database: contextpalace
  Project:  yourproject
  Agent:    agent-yourname
  Status:   connected
  Shards:   N (X open, Y closed, Z other)
```

If it fails, see [Troubleshooting](#troubleshooting).

---

## Step 5: Register Project (if new)

If this is a brand new project that doesn't exist in Context Palace yet:

```bash
cxp init --project yourproject --prefix yp
```

This creates the project record and writes `.cp.yaml`.

For existing projects (like `penfold`), skip this — just use `cxp status` to confirm you can see it.

---

## Step 6: Verify You Can Read and Write

```bash
# Check your inbox
cxp message inbox

# List open shards
cxp shard list --status open --limit 5

# Create a test memory (optional)
cxp memory add "Test memory from setup"

# Search (requires embedding config)
cxp recall "test"
```

---

## What You Get

### Core Commands

| Command | Purpose |
|---------|---------|
| `cxp status` | Connection health + project info |
| `cxp message inbox` | Check unread messages |
| `cxp message send RECIPIENT "Subject" "Body"` | Send a message |
| `cxp message read ID` | Mark message read |
| `cxp shard list` | List shards (tasks, messages, etc.) |
| `cxp shard show ID` | View shard details |
| `cxp shard create --type task "Title" "Body"` | Create a shard |
| `cxp task get ID` | Get task details + artifacts |
| `cxp task claim ID` | Take ownership of a task |
| `cxp task progress ID "Note"` | Log progress on a task |
| `cxp task close ID "Summary"` | Close a task |

### Memory & Knowledge

| Command | Purpose |
|---------|---------|
| `cxp memory add "TEXT"` | Store a memory |
| `cxp memory list` | List memories |
| `cxp memory search "QUERY"` | Search memories |
| `cxp recall "QUERY"` | Semantic search across all shards |
| `cxp knowledge create "Title" "Content"` | Create versioned doc |
| `cxp knowledge show ID` | View doc (latest version) |
| `cxp knowledge update ID "New content"` | Update doc (new version) |
| `cxp knowledge diff ID` | Diff between versions |

### Work Management

| Command | Purpose |
|---------|---------|
| `cxp backlog list` | View prioritized backlog |
| `cxp epic create "Title"` | Create an epic |
| `cxp epic list` | List epics with progress |
| `cxp focus set EPIC_ID` | Set active epic (persists across sessions) |
| `cxp focus clear` | Clear focus |
| `cxp shard assign ID --owner AGENT` | Assign shard to agent |
| `cxp shard next` | Next item to work on |
| `cxp shard board` | Kanban board view |

### Sessions

| Command | Purpose |
|---------|---------|
| `cxp session start "Title"` | Start a work session |
| `cxp session checkpoint "Summary"` | Save checkpoint |
| `cxp session show` | Show current session |
| `cxp session end` | End session |

### Requirements

| Command | Purpose |
|---------|---------|
| `cxp requirement create "Title" "Body"` | Create requirement |
| `cxp requirement list` | List with status |
| `cxp requirement approve ID` | Approve requirement |
| `cxp requirement verify ID` | Mark verified |
| `cxp requirement dashboard` | Overview of all requirements |

### All Commands Support JSON Output

```bash
cxp --output json message inbox
cxp -o json shard list --status open
cxp -o json task get pf-123
```

---

## Session Workflow

At the start of every session:

```bash
cxp message inbox           # Check for messages
cxp shard next              # What should I work on?
cxp session start "Title"   # Start tracking
```

During work:

```bash
cxp session checkpoint "Summary of progress so far"
cxp task progress TASK_ID "What I just did"
```

End of session:

```bash
cxp session checkpoint "Final state: what's done, what's next"
cxp session end
```

---

## For CLAUDE.md Integration

Add this to your project's `CLAUDE.md`:

```markdown
## Context Palace

You are **agent-YOURNAME** on project **YOURPROJECT**.

- Run `cxp status` to verify connection
- Run `cxp message inbox` at session start
- Run `cxp shard next` to find work
- See `context-palace.md` for the full reference
- See `PREFIX-rules.md` for project conventions
```

---

## Troubleshooting

### "database user is required"

Global config missing or `user` field not set. Check `~/.cp/config.yaml` exists and has `connection.user`.

### "agent identity is required"

Neither `~/.cp/config.yaml` nor `.cp.yaml` has an `agent` field, and `CP_AGENT` env var is not set.

### "cannot connect to Context Palace"

1. Check SSL certs exist: `ls ~/.postgresql/`
2. Check network: `nc -zv dev02.brown.chat 5432`
3. Check cert permissions: `chmod 600 ~/.postgresql/postgresql.key`

### "permission denied for table"

Ask administrator to run:

```sql
GRANT ALL ON ALL TABLES IN SCHEMA public TO "penfold";
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO "penfold";
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO "penfold";
```

### "relation does not exist"

Migrations haven't been run. The database schema is managed centrally — contact the administrator or run migrations from the context-palace repo:

```bash
cd ~/github/otherjamesbrown/context-palace/cp/migrations/
for f in *.sql; do
  psql "host=dev02.brown.chat dbname=contextpalace user=penfold sslmode=verify-full" -f "$f"
done
```

### Embedding/recall not working

Semantic search requires the embedding config in `~/.cp/config.yaml` and a `GOOGLE_API_KEY` env var. Without these, `cxp recall` will fail but all other commands work fine.

---

## Environment Variables

Override any config value:

| Variable | Purpose | Default |
|----------|---------|---------|
| `CP_HOST` | Database host | `dev02.brown.chat` |
| `CP_DATABASE` | Database name | `contextpalace` |
| `CP_USER` | Database user | (none — required) |
| `CP_PROJECT` | Project name | (from .cp.yaml) |
| `CP_AGENT` | Agent identity | (from config) |
| `GOOGLE_API_KEY` | For embeddings + generation | (optional) |
