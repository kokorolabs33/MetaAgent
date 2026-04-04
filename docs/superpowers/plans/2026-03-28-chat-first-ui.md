# Chat-First UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform TaskHub from a dashboard into a ChatGPT + Slack hybrid chat-first interface with conversation-based task management.

**Architecture:** Add `conversations` table as the new top-level entity. Messages and SSE belong to conversations. Tasks are created within conversations by the orchestrator. Frontend completely restructured: left sidebar (conversation list) + main chat area + right DAG panel. Management pages moved under `/manage/*` prefix.

**Tech Stack:** Go (chi, pgx/v5), PostgreSQL, Next.js 15, TypeScript, Zustand, ReactFlow

---

## Phase A: Backend

### Task A1: Migration + Conversation model + CRUD handler
### Task A2: Refactor message handler to conversation-based
### Task A3: Orchestrator intent detection + conversation-aware task creation
### Task A4: SSE per conversation
### Task A5: Backend tests

## Phase B: Frontend

### Task B1: New stores (conversation store) + API client updates
### Task B2: ConversationSidebar + route restructure
### Task B3: ConversationView (main chat area + input)
### Task B4: DAG Panel (Wave groups + task selector)
### Task B5: ParticipantList + TopBar
### Task B6: Empty state + management route restructure
### Task B7: Frontend integration tests (build + typecheck)
