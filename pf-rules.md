# pf-rules.md - Context-Palace Rules for penfold

## Project Identity

- **Project:** penfold
- **Prefix:** pf-
- **Agents:**
  - **agent-mycroft** - Primary development agent
  - **agent-penfold** - Development agent
  - **agent-cxp** - Context-Palace maintainer

## Messaging

### Use the penf CLI for messages

**Always use the CLI** - never raw SQL INSERTs for messages:

```bash
# Send a message
penf message send mycroft "Subject line" --body "Message content"

# Check inbox
penf message inbox
penf message inbox --unread

# Read a message
penf message show pf-xxx

# Reply to a message
penf message reply pf-xxx --body "Reply content"
```

The CLI ensures proper labels and conventions. Raw INSERTs will cause messages to get lost.

### Who to contact

| Topic | Send to |
|-------|---------|
| Penfold feature requests | mycroft |
| Penfold bugs | mycroft |
| Context-Palace issues | cxp |
| Infrastructure issues | cxp |

## Message → Task Workflow

Messages are for communication. Tasks are for trackable work.

### When you receive a message that requires work:

1. **Read and acknowledge** the message
2. **Discuss with user** if clarification needed
3. **Create task(s)** linked to the message
4. **Claim the task** with `penf task claim pf-xxx`
5. **Work the task** and close when done
6. **Reply to original message** to confirm completion

## File Claims (Multi-Agent Coordination)

When multiple Claude sessions work in parallel, use file_claims to prevent conflicts.

```bash
# Or via SQL:
SELECT claim_files('pf-xxx', 'session-id', 'mycroft', ARRAY['file1.go', 'file2.go']);
SELECT * FROM check_conflicts(ARRAY['file1.go'], 'my-shard-id');
```

Claims are automatically released by close_task().

### Rules

- Claim files BEFORE reading or writing
- If claim fails, STOP and report conflict
- Claims expire after 1 hour (extend if needed)

## Task Conventions

### Priority

| Priority | Use for |
|----------|---------|
| 0 (Critical) | Production down, security issues |
| 1 (High) | Blocking other work, user-facing bugs |
| 2 (Normal) | Standard features and fixes |
| 3 (Low) | Nice-to-haves, cleanup |

### Naming

- Bug fixes: `fix: description`
- Features: `feat: description`
- Refactoring: `refactor: description`
- Documentation: `docs: description`

## Session Start Checklist

```bash
# 1. Check inbox
penf message inbox --unread

# 2. Check tasks
penf task list

# 3. Process messages before starting new work
# 4. Check file_claims for conflicts
```

## Component Labels

- `cli` - CLI commands (cmd/penf/)
- `gateway` - Gateway service
- `worker` - Worker service
- `database` - Schema, migrations
- `infra` - Infrastructure, deployment
