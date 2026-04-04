# Technology Stack

**Project:** TaskHub — A2A Meta-Agent Platform
**Researched:** 2026-04-04
**Scope:** Additions needed for agent status visualization, enhanced chat, multi-task parallel views, task templates/experience accumulation, and open-source readiness

---

## Existing Stack (Do Not Change)

The existing stack is sound and must be maintained. All additions below are additive.

| Layer | Technology | Version |
|-------|------------|---------|
| Backend language | Go | 1.26.1 |
| HTTP router | Chi | v5.2.5 |
| Database driver | pgx | v5.9.1 |
| Frontend framework | Next.js | 16.1.6 |
| UI library | React | 19.2.3 |
| Styling | Tailwind CSS | v4 |
| UI components | shadcn/ui | latest |
| State management | Zustand | v5.0.11 |
| DAG visualization | @xyflow/react (React Flow) | v12.10.1 |
| Markdown rendering | react-markdown | v10.1.0 |
| Animation utilities | tw-animate-css | v1.4.0 |
| Package manager | pnpm | latest |

---

## New Dependencies by Feature

### Feature 1: Agent Status Visualization

**Problem:** Need real-time online/working/idle/offline status indicators on agent nodes and in the agent list. React Flow already exists but lacks status wrapper; the Agent type already has `is_online` and `status` fields.

**React Flow NodeStatus — already in the dependency tree**

React Flow v12 ships a `NodeStatusIndicator` UI component natively:
- States: `"success"` (online/idle), `"loading"` (working), `"error"` (degraded/offline), `"initial"` (unknown)
- Two loading variants: border (spinning ring around node) and overlay (full node spinner)
- Zero additional dependency — already using `@xyflow/react` v12.10.1

**Recommendation:** Use React Flow's built-in `NodeStatusIndicator` wrapper on `SubtaskNode`. No new dependency needed. Map existing subtask status to the four React Flow states.

**Status badge for agent list and detail pages**

Use shadcn/ui `Badge` component (already available via shadcn) + Tailwind CSS v4 `animate-pulse` for the live indicator dot pattern. This is the standard 2025 pattern for status badges in shadcn/ui projects — no additional library.

```tsx
// Pattern: pulse dot + badge text
<span className="relative flex h-2 w-2">
  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75" />
  <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500" />
</span>
```

**Why not Framer Motion (motion/react)?** Framer Motion (now `motion/react`) is superior for complex physics-based animations but adds ~30KB gzipped to the bundle. Status pulse is a pure CSS keyframe animation — `animate-ping` from Tailwind v4 is sufficient and already included in tw-animate-css. Do not add Framer Motion for this feature alone.

**Confidence: HIGH** — React Flow's NodeStatusIndicator is documented in v12 official docs; Tailwind animate-ping is part of v4 core.

---

### Feature 2: Enhanced Chat Interaction (Sub-agent Intervention)

**Problem:** Users need to send messages to specific sub-agents (not just Master Agent) during task execution. The existing SSE + Zustand + GroupChat pattern is the right foundation.

**No new frontend dependencies required.** The existing stack covers:
- `GroupChat.tsx` + `useTaskStore` for message handling
- `@mention` rendering pattern already in place
- SSE for real-time incoming messages

**Backend consideration:** The existing `/api/tasks/:id/messages` endpoint and SSE broker need extension for direct sub-agent addressing. This is a backend logic change, not a new dependency.

**Confidence: HIGH** — Verified against existing codebase. The store already handles `input_required` subtask status and `approval.requested` events.

---

### Feature 3: Multi-Task Parallel View

**Problem:** Dashboard needs to show multiple tasks executing simultaneously in a split-pane or grid layout.

**Add: react-resizable-panels v2 (via shadcn/ui Resizable)**

| Criterion | Value |
|-----------|-------|
| Package | `react-resizable-panels` |
| Current version | 4.x (breaking API changes in v4) |
| shadcn/ui integration | `pnpm dlx shadcn@latest add resizable` |
| Weekly downloads | ~2.75M (vs allotment ~113K) |
| Maintenance | Active (releases <1yr, by bvaughn) |

**Warning on versions:** react-resizable-panels v4 changed export names (`PanelGroup` → `Group`, `PanelResizeHandle` → `Separator`, `direction` → `orientation`). shadcn/ui's `Resizable` component had a v4 compatibility issue (GitHub issue #9136). Before installing, verify that the current shadcn/ui `resizable` component is compatible with v4.x.

**Safer approach:** Install the shadcn `resizable` component and let it pin the exact react-resizable-panels version it needs. Do not add react-resizable-panels directly as a dependency — let shadcn manage it.

```bash
pnpm dlx shadcn@latest add resizable
```

**Why not Next.js Parallel Routes?** Next.js Parallel Routes (`@slot` conventions) enable truly independent navigation for each panel. For a dashboard that needs to show multiple task instances simultaneously — not just different sections — parallel routes add architectural complexity without sufficient benefit. A CSS grid + resizable panels layout is simpler, more maintainable, and faster to build. Parallel routes are well-suited when each panel needs its own URL state; multi-task view is a view-layer concern.

**Why not allotment?** allotment is VS Code's splitter ported to React and is good, but has 24x fewer weekly downloads. react-resizable-panels is more widely adopted and has tighter shadcn/ui integration.

**Confidence: MEDIUM** — Popularity and maintenance data is HIGH confidence. Version compatibility warning is based on verified GitHub issue; recommend validating before coding.

---

### Feature 4: Task Templates and Experience Accumulation

**Problem:** Save successful orchestration patterns; evolve templates from execution data. Schema already exists (`workflow_templates`, `template_versions`, `template_executions` tables). Frontend `WorkflowTemplate` type exists in `web/lib/types.ts`. A `StepEditor.tsx` component stub exists in `web/components/template/`.

**No new backend dependencies required.** The existing JSONB + pgx pattern is exactly correct for this use case:
- `steps JSONB` and `variables JSONB` columns store flexible schema-less orchestration data
- `template_versions` table tracks evolution with `based_on_executions` counter for experience accumulation
- pgx v5.9.1 supports `json.RawMessage` scanning natively (already used in `template.go`)

**Frontend: Zustand persist middleware for template drafts**

For local template editing state (unsaved drafts), use Zustand's built-in `persist` middleware with `localStorage`:

```typescript
import { persist } from "zustand/middleware"
// Persist draft template edits across page refresh
```

This is built into Zustand v5 — no additional package. Saved/published templates live in PostgreSQL via the existing API. Do not attempt to replace PostgreSQL storage with localStorage for templates.

**Frontend: No WYSIWYG template editor library needed.** The `StepEditor.tsx` component should be a structured form (shadcn/ui `Input`, `Select`, `Textarea`) that manipulates the JSONB steps array. A drag-and-drop step reordering can use React's built-in drag events or a minimal utility if needed.

**Optional: @dnd-kit/core for step reordering**

| Criterion | Value |
|-----------|-------|
| Package | `@dnd-kit/core` + `@dnd-kit/sortable` |
| Version | v6.x |
| Bundle size | ~12KB gzipped |
| Why | Accessible, well-maintained, no React version conflicts |
| When | Only if drag-to-reorder steps is in scope |

**Why not react-beautiful-dnd?** Abandoned by Atlassian in 2023; not maintained for React 19. Do not use.
**Why not dnd-kit/core?** It IS @dnd-kit/core — just abbreviated above.

**Confidence: HIGH** for JSONB + pgx pattern (verified against existing schema and models). MEDIUM for @dnd-kit recommendation (widely used but need to verify React 19 compatibility before adding).

---

### Feature 5: Open-Source Readiness

**Problem:** One-click local startup, good README, demo polish, Docker Compose for zero-config dev.

**Add: docker-compose.yml at repo root**

The existing `Dockerfile` is multi-stage. Add a `docker-compose.yml` that wires:
- PostgreSQL 16 service
- Backend Go service (built from Dockerfile)
- Frontend Next.js service

No new dependencies. Docker Compose v2 is standard; the `watch` command (Docker Compose v2.22+) enables file sync for dev without rebuilds.

**Pattern for .env.example:**
```bash
# Auto-generated on first run if missing
cp .env.example .env
```

A `Makefile` target (`make setup`) that copies `.env.example` to `.env` is the standard pattern for developer tools in 2025.

**No documentation generator needed.** For a developer-audience open-source tool, README.md + inline code comments outperforms generated API docs. Do not add Swagger/OpenAPI doc generation unless specifically required.

**Confidence: HIGH** — Standard open-source practice, no library research needed.

---

## Ruled-Out Dependencies

| Library | Why Not |
|---------|---------|
| `framer-motion` / `motion/react` | Bundle cost (~30KB) not justified for status pulse; Tailwind `animate-ping` is sufficient. Add only if complex interactive animations are needed later. |
| `react-beautiful-dnd` | Abandoned by Atlassian in 2023; incompatible with React 19. |
| `recharts` or `Chart.js` | TaskHub does not need time-series charts. The existing DAG (React Flow) and timeline (`TraceTimeline.tsx`) cover visualization needs. Adding a chart library for the dashboard is pre-optimization. |
| `@tanstack/react-query` | The existing Zustand + direct fetch pattern works. Adding React Query would require migrating all existing data-fetching code or running two data-fetching systems in parallel — not worth it for this milestone. |
| `langfuse` / OpenTelemetry | External observability platforms are out of scope for a self-hosted open-source reference implementation. The existing audit log + token cost system covers the observability need. |
| Next.js Parallel Routes for multi-task view | Architectural overhead not justified; CSS grid + resizable panels achieves the same UX with less complexity. |
| Redux Persist | Project already uses Zustand v5; the built-in `persist` middleware covers the same need. No second state library. |

---

## Final Recommended Additions Summary

| Package | Version | Install Command | Feature |
|---------|---------|-----------------|---------|
| shadcn `resizable` component | (pins react-resizable-panels internally) | `pnpm dlx shadcn@latest add resizable` | Multi-task parallel view |
| `@dnd-kit/core` + `@dnd-kit/sortable` | v6.x | `pnpm add @dnd-kit/core @dnd-kit/sortable` | Template step reordering (if in scope) |
| Zustand `persist` middleware | built-in to Zustand v5.0.11 | (already installed) | Template draft persistence |

All other features (agent status visualization, enhanced chat, experience accumulation backend, open-source readiness) are achievable with zero new dependencies by extending existing code.

---

## A2A Protocol Alignment

**Current A2A protocol version: v1.0.0** (released March 12, 2026, Linux Foundation, Apache 2.0)

Official Go SDK: `github.com/a2aproject/a2a-go` (available via `go get`).

TaskHub currently implements A2A via its own HTTP polling and native adapters (`internal/adapter/`). This is valid — the official Go SDK is a convenience wrapper, not a requirement. The existing adapter layer already implements the JSON-RPC 2.0 message format A2A requires.

**Recommendation:** Do not replace the existing adapter with the official SDK in this milestone. The current implementation is compatible with the A2A spec. Consider adding the SDK only if interoperability with external A2A agents becomes a specific requirement.

**Confidence: HIGH** — Verified against official GitHub repo at `a2aproject/A2A` (v1.0.0, March 2026).

---

## Version Verification Status

| Technology | Claimed Version | Verification Source | Confidence |
|------------|-----------------|---------------------|------------|
| React Flow NodeStatusIndicator | v12 (in @xyflow/react 12.10.1) | reactflow.dev official docs | HIGH |
| react-resizable-panels | v4.x (latest) | npmjs.com package page | HIGH |
| @dnd-kit/core | v6.x | npm + GitHub | MEDIUM (React 19 compat not explicitly verified) |
| A2A protocol | v1.0.0 | github.com/a2aproject/A2A | HIGH |
| Tailwind animate-ping | v4 core | Tailwind v4 docs | HIGH |
| Zustand persist middleware | built-in v5 | zustand.docs.pmnd.rs | HIGH |

---

## Sources

- React Flow NodeStatusIndicator: [reactflow.dev/ui/components/node-status-indicator](https://reactflow.dev/ui/components/node-status-indicator)
- react-resizable-panels: [npmjs.com/package/react-resizable-panels](https://www.npmjs.com/package/react-resizable-panels)
- shadcn/ui Resizable: [ui.shadcn.com/docs/components/radix/resizable](https://ui.shadcn.com/docs/components/radix/resizable)
- shadcn/ui + Tailwind v4: [ui.shadcn.com/docs/tailwind-v4](https://ui.shadcn.com/docs/tailwind-v4)
- Zustand persist middleware: [zustand.docs.pmnd.rs/reference/middlewares/persist](https://zustand.docs.pmnd.rs/reference/middlewares/persist)
- A2A Protocol v1.0.0: [github.com/a2aproject/A2A](https://github.com/a2aproject/A2A)
- A2A Linux Foundation: [linuxfoundation.org/press/linux-foundation-launches-the-agent2agent-protocol-project](https://www.linuxfoundation.org/press/linux-foundation-launches-the-agent2agent-protocol-project-to-enable-secure-intelligent-communication-between-ai-agents)
- PostgreSQL JSONB with Go: [alexedwards.net/blog/using-postgresql-jsonb](https://www.alexedwards.net/blog/using-postgresql-jsonb)
- shadcn Resizable v4 bug: [github.com/shadcn-ui/ui/issues/9136](https://github.com/shadcn-ui/ui/issues/9136)
- Next.js Parallel Routes: [nextjs.org/docs/app/api-reference/file-conventions/parallel-routes](https://nextjs.org/docs/app/api-reference/file-conventions/parallel-routes)
- Motion (Framer Motion): [motion.dev](https://motion.dev)
- LogRocket React Chart Libraries 2025: [blog.logrocket.com/best-react-chart-libraries-2025](https://blog.logrocket.com/best-react-chart-libraries-2025/)

---

*Stack research: 2026-04-04*
