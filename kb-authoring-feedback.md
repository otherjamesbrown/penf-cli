# Feedback on kb-authoring-guide.md

Field notes from running Context Palace's KB system across 4 projects (~70+ shards, 2 authoring agents, 6 weeks of production use). The guide is solid — this feedback covers patterns that emerged in practice and gaps we hit.

---

## 1. The Companion Shard Pattern (missing from guide)

The guide says "reference how to find the value, not the value itself" — correct, but incomplete. In practice, agents frequently need a curated list of current values (timeout keys, queue names, model assignments) without trawling through config tables every time. Telling them "go look it up" works for one-off queries but kills productivity during design and review work where an agent needs to reference 5-6 operational values in a single session.

We solved this with a **companion shard** pattern: a stable architecture article paired with a volatile live-reference article.

### How it works

The architecture shard describes *what* the control surfaces are, *why* they exist, and *how to reason about them*. It changes rarely and is factcheckable.

The live-reference shard lists *current values* — the actual timeout keys, concurrency numbers, queue names. It changes frequently, is expected to drift, and explicitly tells agents to verify before acting.

### Example from production

**Stable shard** — `Pipeline/Config & Operations`:
```markdown
# Pipeline/Config & Operations

Read this when you need to know which control surfaces govern
pipeline behavior, where operational tuning lives, or how to
investigate configuration-driven pipeline issues.

Use this shard for stable operational architecture.
If you need exact current timeout values, queue names, concurrency
numbers, or environment-specific settings, load `pf-2884e7`
(Pipeline Runtime Config Live Reference) and verify against code,
DB config, or runtime state.

## Main Control Surfaces

### Operational Config in the Database
Use this area when behavior should change without a deploy.
Typical examples include:
- throttles
- timeout categories
- operational thresholds
- tuning values that should remain data-driven

### Code-Level Operational Limits
Some behavior may still live in service startup or worker configuration.
These are higher-drift areas and should be treated carefully during
reviews because they may violate the desired DB-driven architecture.
```

**Volatile companion** — `Pipeline Runtime Config Live Reference`:
```markdown
# Pipeline Runtime Config Live Reference

Read this when you need current runtime values for pipeline limits,
timeout keys, queue names, or operational tuning details, and verify
exact live values in code, DB config, or deployed runtime when
precision matters.

## Purpose
This is the volatile companion reference for pipeline operations.
It holds the kinds of values that change more often than the
underlying architecture.

## Verification Rule
When exact runtime values matter, verify against:
- config tables
- current code
- service runtime state
- end-to-end tests where applicable
```

### Suggested addition to the guide

Add a section under "Article Types" called **Companion pairs: stable + volatile**:

> Some subsystems need both a stable reference (how things work) and a volatile reference (what the current values are). Split them into two sibling shards under the same branch. The stable shard links to the volatile one with a "for current values, see..." pointer. The volatile shard carries an explicit verification rule reminding agents not to trust its values blindly.
>
> **When to use:** Any subsystem where agents regularly need current config values during design or review work, but those values change independently of the architecture.
>
> **Naming convention:** `[Subsystem]/Config & Operations` (stable) + `[Subsystem] Runtime Config Live Reference` (volatile).

---

## 2. Version History (not covered at all)

The guide mentions `cxp knowledge update` creates a new version, but never explains the resulting data model or how agents should use it. In practice, versioned articles accumulate `previous-version` edges that form a change history — and this history is valuable during debugging and renovation.

### What we see in production

```
pf-861f0c   Content Processing Pipeline (current, v3)
  ├── previous-version → pf-861f0c-v2 (closed)
  └── previous-version → pf-861f0c-v1 (closed)
```

Each version edge carries metadata:
```json
{
  "changed_at": "2026-03-06 11:00:15+00",
  "changed_by": "agent-penfold",
  "change_summary": "Consolidate 6 decision memories: timeout precedence, ..."
}
```

### What the guide should cover

- **The version graph exists.** When you update an article, the old version is closed and linked via a `previous-version` edge. Agents can traverse this to understand how knowledge evolved.
- **Change summaries matter.** Write them like commit messages — what changed and why. "Updated content" is useless. "Add classify_project stage, replaces legacy keyword matching" tells an agent whether the change is relevant to their current work.
- **Closed versions are still accessible.** `cxp shard show <version-id>` works. This is useful when investigating regressions — "what did this article say before the last update?"
- **Don't version for typo fixes.** Minor corrections don't need a version trail. Version when the conceptual content changes.

---

## 3. The Unsorted Problem (guide pretends this won't happen)

The guide's bootstrap process is clean: identify domains, create branches, write leaves depth-first. In practice, articles get created during fast-moving implementation sessions where the priority is capturing knowledge, not filing it correctly. Result: an "Unsorted" branch that accumulates orphaned leaves.

### Our numbers

After 6 weeks:
- 15 top-level items (guide recommends 3-10)
- 10 shards explicitly in an "Unsorted" branch
- 6 top-level singletons with 0 children (leaves masquerading as branches)
- ~50 orphaned leaves at depth 1 under no meaningful branch

The guide's advice — "write articles as they become needed" — is correct but needs a companion prescription for what happens when creation outpaces organization.

### Suggested addition

Add a section called **Managing the unsorted backlog**:

> In active projects, KB articles are often created faster than they can be properly parented. This is fine — capturing knowledge quickly is more important than filing it correctly in the moment.
>
> **Accept the unsorted bucket.** Create an explicit "Unsorted" branch. New articles that don't have an obvious home go here rather than being force-fitted into the wrong branch or left as top-level orphans.
>
> **Triage weekly.** During the weekly triage cadence:
> 1. Review all shards in Unsorted
> 2. For each: move to the correct branch, merge into an existing article, or close if redundant
> 3. Check for top-level singletons (0 children, depth 0) — these are usually leaves that should be under a branch
>
> **Watch the ratio.** If Unsorted holds more than ~20% of your total shards, your branch structure probably doesn't match your actual work patterns. Restructure branches before triaging individual articles.
>
> **Distinguish branches from leaves.** A shard with 0 children at depth 0 is a leaf pretending to be a branch. Either give it children (it's actually a branch that hasn't been fleshed out) or move it under the right parent.

---

## 4. Multi-Agent Authoring (not addressed)

The guide assumes one agent writes and maintains the KB. We have two: `agent-penfold` (primary author, writes during working sessions) and `agent-mycroft` (maintenance agent, runs factchecks, drift scans, and renovations). This creates coordination problems the guide doesn't anticipate.

### Problems we've hit

1. **Ownership ambiguity.** When mycroft updates an article during a drift scan, penfold doesn't know the content changed until it reads the article again. If penfold is mid-session with cached knowledge from the old version, it can make decisions on stale data.

2. **Conflicting update styles.** Penfold writes articles in the context of a feature being built — rich, detailed, connected to the work. Mycroft updates articles during maintenance — mechanical, anchor-focused, sometimes strips context that was there for a reason.

3. **Version attribution.** Our metadata tracks `last_changed_by`, which is essential. Without it, there's no way to know whether an article's current state reflects a deliberate authoring choice or an automated maintenance pass.

### Suggested addition

Add a section called **Multi-agent KB authoring**:

> When multiple agents write or maintain the KB, establish clear ownership:
>
> **Author vs. maintainer roles:**
> - The *author* creates articles and makes substantive content changes (new sections, revised explanations, updated architecture descriptions)
> - The *maintainer* updates anchors, fixes drift, and flags articles for rewrite — but does not change the narrative without escalation
>
> **Track attribution in metadata.** Every update should record `last_changed_by` and `last_change_summary`. This lets any agent distinguish between "this article was deliberately rewritten" and "an automated scan updated a file path."
>
> **Maintenance updates should be conservative.** When a maintainer finds a stale anchor, the correct action is usually to update the anchor and note the change — not to rewrite the surrounding explanation. If the surrounding content is also wrong, escalate to the author or flag it in the escalation queue.
>
> **Notify on substantive changes.** If a maintenance agent rewrites more than just anchors, it should create a notification (e.g., a shard note, a gap log entry, or a direct flag) so the author can review.

---

## 5. Meta-KB Shards (mentioned but not formalized)

The guide references canary questions and gap logging in passing but doesn't define them as a category. We have three meta-KB shards that are essential to KB health:

| Shard | Purpose | Content |
|-------|---------|---------|
| `pf-kb-canaries` | 25 retrieval test questions with expected facts and source KB references | Seeded on bootstrap, verified by running `cxp kb search` |
| `pf-kb-gaps` | Log of KB failures: hallucinations, omissions, drift, retrieval failures, coverage holes | Entries categorized by type with date and source |
| `pf-kb-escalations` | Issues no agent can resolve autonomously | Reviewed weekly by a human |

### Why they need formal treatment

These aren't regular KB articles — they're operational infrastructure for the KB itself. They:
- Don't describe a subsystem (so the reference article template doesn't fit)
- Don't describe a procedure (so the runbook template doesn't fit)
- Are never "done" — they accumulate entries over time
- Need their own maintenance cadence (canary questions need re-seeding after renovation, gaps need weekly triage)

### Suggested addition

Add a third article type under "Article Types" called **Meta-KB operational shards**:

> These shards track the health and coverage of the KB itself. They are not subject to the same authoring rules as reference articles — they are append-only logs or structured test fixtures.
>
> **Required meta-KB shards:**
>
> 1. **Canary questions** — A set of retrieval test queries with expected facts and source article IDs. Used to verify that search returns the right articles. Re-seed after any renovation or major restructuring.
>
> 2. **Gap tracker** — Append-only log of KB failures. Categories: hallucination (factcheck caught a wrong claim), omission (judge found missing coverage), drift (nightly scan found anchor rot), retrieval-failure (canary question returned wrong article), coverage-hole (agent couldn't find what it needed).
>
> 3. **Escalation queue** — Issues that require human review. Agents add entries; humans resolve them during weekly triage.
>
> **Labeling:** Use a consistent label (e.g., `kb-maintenance`) so these shards are easy to find and exclude from content searches.
>
> **Placement:** These shards should not be children of any content branch. Either make them top-level with a `kb-maintenance` label, or create a dedicated "KB Health" branch.

### Example: canary question format

From our production canary shard:
```yaml
- q: "What model does the classify_project stage use?"
  expected_facts: ["gemini-2.5-flash", "classify_project", "ai_routing_rules"]
  source_kb: pf-861f0c

- q: "Why does classify_project run for all content regardless of triage skip decisions?"
  expected_facts: ["skip_when_low", "PERSONAL", "project attribution"]
  source_kb: pf-d7b678
```

Each question targets a specific article. If search returns a different article or the expected facts aren't in the result, the retrieval system has a problem.

---

## 6. Access Counts as a Signal (underweighted)

The guide mentions access counts once, in the renovation section ("pick the most-used branch"). They deserve more treatment — they're one of the few quantitative signals for KB health.

### What access counts tell you

Our `Content Processing Pipeline` article (pf-861f0c) has 16 accesses across two agents. Our `Pipeline Runtime Config Live Reference` (pf-2884e7) has far fewer. This tells us:
- The architecture article is the entry point; the live reference is a secondary lookup
- Agents use the stable shard for orientation, then drill into specifics
- If the stable shard had 0 accesses, it would be a sign that agents are bypassing the KB and going straight to code

### What access counts DON'T tell you

- **Zero accesses doesn't mean the article is bad** — it might cover a subsystem nobody has worked on recently
- **High accesses doesn't mean the article is good** — agents might be loading it repeatedly because it doesn't answer their question and they keep retrying

### Suggested additions

In the "Ongoing Authoring" section:
> **Monitor access counts during triage.** Articles with zero accesses after 4+ weeks of active development in their subsystem are candidates for review — either the trigger isn't matching, the article isn't at the right abstraction level, or the subsystem genuinely hasn't been touched.
>
> **Pair access counts with gap logs.** If agents are logging gaps in an area that has KB coverage, the existing articles aren't working. High gap log entries + existing articles = rewrite needed, not new articles.

---

## 7. Labels (not mentioned)

The guide never discusses labeling strategy. We use labels to cross-cut the tree structure:

- `operations` — operational runbooks and config references
- `kb-maintenance` — meta-KB shards (canaries, gaps, escalations)
- `reference` — volatile reference shards (companion pattern)

Labels are useful for:
- Filtering search results by category
- Identifying all operational content regardless of which branch it's under
- Tagging companion shards so agents know they're volatile

### Suggested addition

Brief section under "Tree Structure":

> **Use labels for cross-cutting concerns.** The tree structure organizes by subsystem, but some properties cut across subsystems — "this is operational," "this is volatile," "this is a maintenance artifact." Use labels for these. Keep the label vocabulary small (5-10 labels) and document what each label means.

---

## 8. Minor Suggestions

**Article size guidance needs a caveat.** The guide says 100-400 lines for leaves. Our `Content Processing Pipeline` article is well over 400 lines because it covers 6 pipelines with stage tables, routing rules, and classification logic. The right answer was one big article, not 6 thin ones — the pipelines are only meaningful in comparison. Add: "If splitting would force agents to load 3+ shards to answer a single question, keep it together even if it exceeds 400 lines."

**The "don't write articles for unstable code" rule needs nuance.** We wrote KB articles during CoBuild implementation sessions — the code was actively being built. The articles were accurate *at the point of delivery* and became the stable reference. The real rule is: don't write articles for code that's being *rewritten*, but do write them for code that's being *built for the first time* once it passes review.

**Cross-referencing between articles could use a convention.** We use inline references like "see: **Model Selection Architecture** (pf-bc537d, under Infrastructure)" — shard title, ID, and parent branch. This pattern appears in several of our articles but the guide doesn't prescribe a format for inter-article links.

---

## Summary

The guide covers the "what" and "why" of KB authoring well. The gaps are in the "what happens after 6 weeks" territory:

| Gap | Impact | Effort to add |
|-----|--------|---------------|
| Companion shard pattern | Agents either hardcode values or waste time looking them up | Medium — new article type section |
| Version history | Agents can't reason about KB evolution | Small — explain the data model |
| Unsorted management | Tree structure degrades over time | Small — prescribe triage cadence |
| Multi-agent authoring | Conflicting updates, stale cached knowledge | Medium — new section |
| Meta-KB shards | Operational infrastructure is ad-hoc | Medium — formalize as article type |
| Access count signals | Quantitative health signal underused | Small — add to triage section |
| Labels | No cross-cutting organization | Small — brief section |

The foundation is strong. These additions would close the gap between "bootstrapping a KB" and "operating one at scale."
