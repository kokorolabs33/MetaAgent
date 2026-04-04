# Phase 1: Foundation - Discussion Log (Assumptions Mode)

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the analysis.

**Date:** 2026-04-04
**Phase:** 01-Foundation
**Mode:** assumptions (--auto)
**Areas analyzed:** SSE Race Fix, Docker Compose, LLM Dependency, Replanning Visibility, Trace View, README

## Assumptions Presented

### SSE Subscribe/Replay Race Condition Fix
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Reorder stream.go to subscribe before replay, dedup by event ID | Confident | `internal/handlers/stream.go:53-73`, `internal/events/store.go:61-63`, `internal/events/broker.go:25` |

### Docker Compose One-Click Startup
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Create new docker-compose.yml, fix Next.js standalone output, seed demo agents | Confident | No docker-compose.yml exists, `web/next.config.ts` empty config, `internal/seed/devseed.go` only seeds user |

### LLM Dependency Strategy
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Replace claude CLI with Anthropic Go SDK for Docker compatibility | Likely | `internal/orchestrator/orchestrator.go:134-147`, Dockerfile has no claude CLI, PITFALLS.md Pitfall 9 |

### Replanning Visibility
| Assumption | Confidence | Evidence |
|------------|-----------|----------|
| Add task.replanned handling to TraceTimeline and Zustand store | Confident | `internal/executor/executor.go:986`, zero frontend handling found |

## Corrections Made

No corrections — all assumptions confirmed (auto mode).

## Auto-Resolved

- LLM Dependency Strategy (Likely): auto-selected "Replace claude CLI with Anthropic Go SDK" — more robust for Docker deployment, aligns with existing TODO in codebase

## External Research Flagged

- Anthropic Go SDK maturity and streaming support
- Next.js 16 standalone output configuration with App Router
