# Penfold Vision

> **Last updated:** 2026-01-26

## One-Line Summary

**Penfold is your institutional memory.**

## The Problem

In knowledge work, critical information is scattered:
- **Emails** - decisions buried in threads
- **Meetings** - context lost after transcripts are filed
- **Documents** - tribal knowledge never written down
- **Chat** - important discussions disappear in scroll

The result:
- Decisions get made twice because no one remembers the first time
- New team members take months to get up to speed
- Expertise is invisible - you don't know who knows what
- Context switching costs hours of archaeology

## The Solution

Penfold solves this by:

1. **Aggregating** - Ingest content from all communication channels into one searchable system
2. **Understanding** - Identify the people, products, and concepts mentioned in your content
3. **Connecting** - Link related information through entity resolution and relationship discovery
4. **Surfacing** - Make relevant knowledge findable through intelligent search and AI queries

## Core Principles

### 1. AI-Native Design

Penfold is designed for an AI assistant (Claude Code) to help users, not for direct human CLI use. This means:
- CLI commands are optimized for AI consumption (JSON output, batch processing)
- Documentation helps Claude understand context, not teach humans commands
- Workflows are designed for intelligent batch processing, not one-at-a-time operations

### 2. Entity-Centric

Everything revolves around entities:
- **People** - who is mentioned, who authored, who has expertise
- **Products** - what business products/features are discussed
- **Terms** - domain-specific acronyms and terminology
- **Content** - the raw material (emails, meetings, documents)

### 3. Progressive Enhancement

The knowledge base improves over time:
- Auto-discovered entities start as "needs review"
- Human feedback confirms and refines
- Patterns learned from resolutions auto-apply to new content
- Query expansion improves as glossary grows

### 4. Privacy by Design

- All data stays on infrastructure you control
- Multi-tenant isolation keeps contexts separate
- Local AI models (MLX) avoid sending content to external services
- Source tagging lets you control what's searchable

## The Goal

**Never lose context. Always know who knows what.**

When you ask "What did we decide about X?" or "Who should I talk to about Y?", Penfold should have the answer - not because it remembered, but because it connected the dots from your existing communications.

## See Also

- [interaction-model.md](interaction-model.md) - How users interact with Penfold via Claude Code
- [use-cases.md](use-cases.md) - Detailed use cases with priorities
- [entities.md](entities.md) - Core entity types and relationships
