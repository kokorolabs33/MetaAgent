# Phase 8: Artifact Rendering - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Structured agent outputs render as rich interactive UI cards instead of raw text. Typed artifact schema with dedicated renderers for search results, code, tables, and data. Markdown rendering upgraded with GFM and syntax highlighting. Copy/download actions on artifacts.

</domain>

<decisions>
## Implementation Decisions

### Artifact Type Schema
- **D-01:** Use A2A protocol's existing ArtifactPart type with a discriminated `type` field added to the Data payload. Types: `search_results`, `code`, `table`, `data`, `markdown`.
- **D-02:** Agent produces artifacts by returning ArtifactPart in the A2A response. The `type` field tells the frontend which renderer to use.
- **D-03:** Define a TypeScript discriminated union for artifact types so each renderer gets type-safe props.

### Artifact Storage
- **D-04:** Artifacts flow through existing A2A ArtifactPart mechanism — stored as part of the message in the messages table via the existing metadata/content fields. No new database table.
- **D-05:** Frontend parses ArtifactPart from message data and routes to the correct renderer component.

### Rich Card Design
- **D-06:** Artifact cards embedded inline in the chat flow, directly below the agent message that produced them. Not a sidebar or floating panel.
- **D-07:** SearchResultsCard — clickable source cards with title, URL, snippet. Compact list style.
- **D-08:** CodeBlock — syntax-highlighted code with language label and copy button. Uses rehype-highlight.
- **D-09:** TableCard — GFM table rendered as formatted HTML table with proper styling.
- **D-10:** DataCard — key-value data display for structured analysis results.

### Markdown Upgrade
- **D-11:** Replace current markdown rendering with react-markdown + remark-gfm + rehype-highlight. Already have react-markdown@10.1.0 installed.
- **D-12:** Add remark-gfm for GFM table and strikethrough support. Add rehype-highlight for syntax-highlighted code blocks.
- **D-13:** Apply markdown upgrade to all message rendering (not just artifacts) — global improvement.

### Copy/Download Actions
- **D-14:** Each artifact card has a copy-to-clipboard button (copies the raw data/text).
- **D-15:** Code blocks and data cards have a download button (download as .txt/.json/.md file).
- **D-16:** Use a floating action bar at the top-right of each artifact card (similar to code block hover actions in ChatGPT).

### Claude's Discretion
- Exact card border/shadow styling within shadcn/ui conventions
- rehype-highlight theme selection (dark theme to match existing UI)
- How to handle very large artifacts (truncation threshold)
- DataCard layout (horizontal vs vertical key-value pairs)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### A2A Protocol
- `internal/a2a/protocol.go` — ArtifactPart type definition, Message structure
- `cmd/openaiagent/main.go` — Where agent responses are constructed

### Frontend Chat
- `web/components/chat/ChatMessage.tsx` — Current message rendering with react-markdown
- `web/components/chat/GroupChat.tsx` — Chat feed container

### Dependencies
- `web/package.json` — react-markdown@10.1.0 already installed
- `.planning/research/STACK.md` — remark-gfm@4.0.1, rehype-highlight@7.0.2 recommendations

### Research
- `.planning/research/ARCHITECTURE.md` — Artifact rendering architecture
- `.planning/research/PITFALLS.md` — Pitfall on artifact schema contract

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `react-markdown@10.1.0` already installed
- `ChatMessage.tsx` already renders markdown — extend with artifact renderers
- A2A `ArtifactPart` already defined in protocol.go

### Established Patterns
- shadcn/ui Card component for consistent card styling
- Tailwind CSS for all styling
- Chat message rendering via react-markdown

### Integration Points
- `ChatMessage.tsx` — Add artifact renderer dispatch based on ArtifactPart type
- `cmd/openaiagent` — Produce ArtifactPart in A2A responses when agent has structured data
- `web/lib/types.ts` — Add artifact type definitions

</code_context>

<specifics>
## Specific Ideas

- Artifact cards should feel like a natural extension of the message, not a separate UI element
- Search results should look like Google search result snippets — familiar pattern

</specifics>

<deferred>
## Deferred Ideas

- Cross-agent artifact passing (agent A's artifact used by agent B) — future
- Artifact versioning/history — not needed for demo
- Interactive artifacts (editable tables, runnable code) — future

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-artifact-rendering*
*Context gathered: 2026-04-07*
