# Technology Stack: v2.0 Additions

**Project:** TaskHub v2.0 -- Agent Tool Use, Artifact Rendering, Streaming Output, Inbound Webhooks
**Researched:** 2026-04-06
**Scope:** NEW dependencies only. Existing stack (Go 1.26, Next.js 16, PostgreSQL, pgx, shadcn/ui, Zustand, React Flow, SSE broker) is validated and unchanged.

---

## Recommended Stack Additions

### Backend: OpenAI Official Go SDK (replaces raw HTTP client)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/openai/openai-go/v3` | v3.30.0 | Function calling, streaming, structured tool use | Official SDK with type-safe tool definitions, streaming iterators, and ChatCompletionAccumulator. Replaces the hand-rolled HTTP client in `internal/llm/openai.go` which cannot do function calling or streaming. |

**Confidence:** HIGH -- verified via GitHub releases page (v3.30.0 released 2026-03-25), pkg.go.dev documentation, and official examples.

**Why the official SDK instead of keeping raw HTTP:**
1. The current `internal/llm/openai.go` uses manual `http.NewRequest` + `json.Marshal` for chat completions (123 lines). Adding function calling on top of this means hand-building tool parameter schemas, parsing `tool_calls` arrays from chunked JSON, managing the multi-turn tool call loop, and handling streaming SSE parsing -- all of which the SDK does natively.
2. The SDK provides `ChatCompletionAccumulator` which assembles streaming chunks and fires `JustFinishedToolCall()` callbacks -- exactly what is needed for streaming + tool use combined.
3. The SDK's `ChatCompletionFunctionTool()` helper provides type-safe tool definitions with JSON Schema parameter validation.
4. Requires Go 1.22+ (we have Go 1.26.1, satisfied).

**Migration path:** Replace `internal/llm/openai.go` with SDK-based client. The `llm.Client` interface stays the same externally but gains `ChatWithTools()` and `ChatStreaming()` methods. The `cmd/openaiagent` binary switches from `llm.Client` wrapper to direct SDK usage for tool-calling agents.

**Key SDK patterns needed:**

```go
// Tool definition
openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
    Name:        "web_search",
    Description: openai.String("Search the web for current information"),
    Parameters: openai.FunctionParameters{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]string{"type": "string"},
        },
        "required": []string{"query"},
    },
})

// Streaming with tool call detection via accumulator
stream := client.Chat.Completions.NewStreaming(ctx, params)
acc := openai.ChatCompletionAccumulator{}
for stream.Next() {
    chunk := stream.Current()
    acc.AddChunk(chunk)
    // Forward delta content to SSE broker for real-time UI
    if len(chunk.Choices) > 0 {
        publishTokenDelta(chunk.Choices[0].Delta.Content)
    }
    // Detect when a tool call finishes streaming
    if tool, ok := acc.JustFinishedToolCall(); ok {
        result := executeToolCall(tool.Name, tool.Arguments)
        // Append tool result and continue conversation
    }
}
```

### Backend: No New Dependencies for Webhook Verification

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `crypto/hmac` + `crypto/sha256` (stdlib) | Go 1.26 | Inbound webhook HMAC-SHA256 signature verification | Already in Go stdlib. The existing `internal/webhook/sender.go` already uses these for outbound signatures. Inbound verification uses the same primitives in reverse. |
| `crypto/subtle` (stdlib) | Go 1.26 | Constant-time comparison to prevent timing attacks | `hmac.Equal()` uses `subtle.ConstantTimeCompare` internally -- the standard Go pattern. |

**Confidence:** HIGH -- no new dependencies needed. Inbound webhook verification is the mirror of the existing outbound signing code at `internal/webhook/sender.go:73-84`.

**What NOT to add:** Do not add a webhook verification library (e.g., `svix/svix-webhooks`). The verification is 15 lines of Go stdlib code and adding a dependency for it is unnecessary overhead.

### Backend: Web Search Tool Implementation

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `net/http` (stdlib) | Go 1.26 | HTTP client for web search API calls | The web search tool calls an external search API (Brave Search, SerpAPI, or similar). No SDK needed -- a simple HTTP GET with JSON response parsing. |

**Confidence:** HIGH -- web search is a simple HTTP call, not a library concern.

**Why custom function calling + web search over OpenAI's built-in `web_search_preview` tool:**
OpenAI's Responses API offers a built-in `web_search_preview` tool, but using it locks the implementation to OpenAI-specific tooling. Implementing web search as a custom function call means:
1. It works with any LLM provider (Anthropic, open-source models)
2. Full control over search provider, rate limiting, caching
3. Visible in the UI as a tool call step (educational for the demo)
4. The search results can be stored as artifacts

---

### Frontend: Markdown Rendering Upgrade

| Library | Version | Purpose | Why |
|---------|---------|---------|-----|
| `remark-gfm` | ^4.0.1 | GitHub Flavored Markdown (tables, task lists, strikethrough) for react-markdown | Agents produce structured reports with tables and lists. GFM is the standard for this. `react-markdown@10.1.0` is already installed but unused -- `remark-gfm` enables its table/tasklist support. |
| `rehype-highlight` | ^7.0.2 | Syntax highlighting for code blocks in agent output | Agents will produce code snippets and diffs. Uses highlight.js via lowlight -- fast, 37 languages bundled by default, small footprint. |
| `highlight.js` | ^11.x | CSS themes for syntax highlighting | Import `highlight.js/styles/github-dark.css` for dark theme matching TaskHub's UI. Peer dependency of rehype-highlight via lowlight. May need explicit install if TypeScript cannot resolve the CSS import from the transitive dependency. |

**Confidence:** HIGH for remark-gfm (verified npm, stable at 4.0.1, compatible with react-markdown 10.x). MEDIUM for rehype-highlight (verified at 7.0.2, but integration with react-markdown + Next.js 16 App Router needs build-time testing).

**Why react-markdown + plugins instead of keeping the hand-rolled renderer:**
The current `ChatMessage.tsx` has a 240-line hand-rolled markdown renderer (`renderMarkdown()` function at line 92). It handles headings, bold, lists, tables, and mentions. But it cannot handle:
- Fenced code blocks with syntax highlighting (critical for artifact rendering)
- Nested lists
- Links and images
- Inline code with backticks
- Blockquotes

Rather than extending the hand-rolled renderer for every new markdown feature, switch to `react-markdown` (already installed at v10.1.0, never imported) with `remark-gfm` + `rehype-highlight`. Keep the custom `@mention` rendering via react-markdown's `components` prop override.

**Why rehype-highlight over alternatives:**
- `react-syntax-highlighter`: Deprecated maintainer, security vulnerabilities, very slow initial rendering with multiple code blocks. Confirmed by Vercel team in `vercel/ai-elements` issue #14. Do NOT use.
- `shiki` / `react-shiki`: Better highlighting quality but 695KB-1.2MB bundle, requires WASM loading, and adds complexity. Overkill for a demo platform.
- `rehype-pretty-code`: Shiki-based, same bundle concerns. Better for static build-time highlighting, not for streaming dynamic content.
- `rehype-highlight`: 37 languages out of the box, uses lowlight (fast), small bundle (~50KB), integrates directly with react-markdown's rehype plugin chain. Right-sized for this use case.

**Implementation pattern:**

```tsx
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import "highlight.js/styles/github-dark.css";

function MessageContent({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeHighlight]}
      components={{
        // Custom @mention rendering preserved from existing code
        // Artifact card wrapping for structured outputs
      }}
    >
      {content}
    </ReactMarkdown>
  );
}
```

### Frontend: No New Dependencies for Streaming UI

| Consideration | Decision | Why |
|--------------|----------|-----|
| Token streaming display | Use existing SSE infrastructure | The `web/lib/sse.ts` already handles SSE events with `EventSource`. A new event type `token_delta` flows through the same channel. Zustand store accumulates deltas into message content. |
| Typing animation / cursor | CSS only | Token-by-token arrival is inherently animated. A blinking cursor CSS pseudo-element on the last in-progress message suffices. No animation library needed. |
| Stream state management | Zustand (existing) | Add `streamingMessages` map to existing task/conversation store. Key: subtask_id, value: accumulated text. On stream completion, promote to full message. |

**Confidence:** HIGH -- the existing SSE infrastructure is well-designed for this. The broker already supports publishing per-task events with `subtask_id` and `actor_id` fields, which map directly to streaming token attribution.

### Frontend: No New Dependencies for Artifact Cards

| Consideration | Decision | Why |
|--------------|----------|-----|
| Rich card rendering | shadcn/ui Card + existing Tailwind | Artifact cards (tables, reports, code diffs) are rendered using existing shadcn/ui `Card`, `Table`, `Badge` components plus Tailwind styling. No new component library needed. |
| JSON data display | react-markdown code blocks + rehype-highlight | JSON artifacts render as syntax-highlighted code blocks. Covered by the markdown upgrade above. |
| Tabular data | react-markdown + remark-gfm tables | GFM table syntax renders natively. For structured JSON data, agents can output markdown tables which remark-gfm handles. |

**Confidence:** HIGH -- existing component library covers all artifact rendering needs.

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Go OpenAI client | `openai/openai-go/v3` | Keep raw HTTP in `internal/llm/openai.go` | Cannot do function calling or streaming without reimplementing 500+ lines of SSE parsing, tool call loop management, and chunk accumulation that the SDK handles |
| Go OpenAI client | `openai/openai-go/v3` | `sashabaranov/go-openai` | Unofficial community SDK. The official SDK was released July 2024 and is actively maintained (v3.30.0, March 2026). No reason to use unofficial when official exists. |
| Markdown rendering | `react-markdown` + plugins | Keep hand-rolled renderer in ChatMessage.tsx | The hand-rolled renderer would need 200+ more lines to support code highlighting, nested lists, links, images, blockquotes. Diminishing returns on custom code vs. battle-tested library already in package.json. |
| Syntax highlighting | `rehype-highlight` | `react-syntax-highlighter` | Deprecated, security issues, slow rendering confirmed by Vercel team. |
| Syntax highlighting | `rehype-highlight` | `shiki` / `react-shiki` | 695KB-1.2MB bundle, WASM loading complexity. Overkill for demo platform. |
| Syntax highlighting | `rehype-highlight` | `rehype-pretty-code` | Shiki-based, same bundle concerns. Better for static sites, not dynamic streaming content. |
| Webhook verification | Go stdlib `crypto/hmac` | `svix/svix-webhooks` | Adding a dependency for 15 lines of stdlib code is wrong. We already have the pattern in sender.go. |
| Streaming UI | Existing SSE + Zustand | WebSocket library | Native `EventSource` is already working. WebSockets add bidirectional complexity with no benefit for one-way token streaming. |
| Agent tool framework | Custom tool registry in Go | LangChain Go / CrewAI | TaskHub IS the framework. Adding another framework inside it defeats the purpose. Tool use is a simple interface + dispatch loop. |
| Web search | Custom function call | OpenAI `web_search_preview` built-in | Locks to OpenAI, no control over search provider, not visible as tool call step in UI. |

---

## What NOT to Add

These were considered and explicitly rejected:

| Library | Why Not |
|---------|---------|
| `langchain-go` or any agent framework | TaskHub IS the framework. Adding another framework inside it defeats the purpose. Tool use is a simple interface + dispatch loop (~100 lines). |
| `gorilla/websocket` | Not needed. SSE is simpler, sufficient for one-way streaming, and already working. WebSockets add bidirectional complexity with no benefit for token streaming. |
| `react-syntax-highlighter` | Deprecated, slow, security issues. Use `rehype-highlight` instead. |
| `shiki` / `@shikijs/rehype` | Bundle too large (695KB+) for a demo platform. `rehype-highlight` is right-sized. |
| `svix/svix-webhooks` | Stdlib crypto is sufficient for HMAC verification. |
| `eventsource-parser` or SSE client libraries | Native `EventSource` API is already working in `web/lib/sse.ts`. |
| Any animation library (framer-motion, react-spring) | Token streaming is inherently animated. CSS cursor blink is sufficient. |
| `zod` or schema validation library | OpenAI SDK handles tool parameter schema validation. Webhook payload validation is simple JSON field checks. |
| OpenAI Agents SDK (Python) | Wrong language. TaskHub is Go. The Go SDK provides all needed primitives. |
| OpenAI Responses API | While newer (March 2025), the Chat Completions API with function calling is more mature in the Go SDK, simpler to integrate, and does not lock into OpenAI-specific built-in tools. The Responses API is better suited for applications fully committed to OpenAI's agent ecosystem. |
| `@tanstack/react-query` | The existing Zustand + direct fetch pattern works. Adding React Query would require migrating all existing data-fetching or running two systems in parallel. |
| `recharts` / `Chart.js` | No chart needs in this milestone. |

---

## Installation

### Backend (Go)

```bash
# Add the OpenAI Go SDK -- the ONLY new Go dependency
go get github.com/openai/openai-go/v3@v3.30.0
```

Everything else (HMAC verification, streaming SSE to browser, web search HTTP calls) uses Go stdlib or existing dependencies (chi, pgx, uuid).

### Frontend (pnpm)

```bash
cd web
pnpm add remark-gfm@^4.0.1 rehype-highlight@^7.0.2
```

Note: `highlight.js` is a transitive dependency of `rehype-highlight` via `lowlight`. The CSS theme import (`highlight.js/styles/github-dark.css`) works because lowlight bundles highlight.js. If TypeScript or the build cannot resolve the CSS import from the transitive dependency, add it explicitly:

```bash
pnpm add highlight.js@^11
```

`react-markdown@10.1.0` is already installed (in `web/package.json`) but currently unused in any component. No version change needed.

---

## Integration Points

### 1. OpenAI SDK Integration (Backend)

**Current:** `internal/llm/openai.go` -- raw HTTP client, 123 lines, chat completions only.
**New:** Replace with SDK-based client wrapping `openai.NewClient()`.

| Integration Point | Current Code | Change |
|-------------------|-------------|--------|
| `llm.Client.Chat()` | Raw HTTP POST to `/v1/chat/completions` | `client.Chat.Completions.New()` via SDK |
| `llm.Client.ChatWithHistory()` | Raw HTTP POST with message array | `client.Chat.Completions.New()` with typed `ChatCompletionMessageParamUnion` messages |
| NEW: `llm.Client.ChatWithTools()` | Does not exist | SDK `ChatCompletionNewParams` with `Tools` array + tool call loop |
| NEW: `llm.Client.ChatStreaming()` | Does not exist | SDK `client.Chat.Completions.NewStreaming()` + chunk iteration via `stream.Next()` |
| `cmd/openaiagent/openai.go` | Wraps `llm.Client` | Wraps SDK client directly, gains tool calling + streaming |

**Key consideration:** The SDK client reads `OPENAI_API_KEY` from environment automatically via `openai.NewClient()`. The current manual key handling in `llm.NewClient(apiKey)` can be simplified, but keep the explicit key parameter for testability and multi-key scenarios (master agent vs team agents may use different keys).

### 2. Streaming Pipeline (Backend to Frontend)

**Current flow:** Agent completes full response -> stored in DB -> SSE event `message` sent -> frontend renders complete message.

**New flow for streaming agents:**
1. Agent starts streaming via `client.Chat.Completions.NewStreaming()`
2. Each text delta chunk: publish SSE event `token_delta` via broker with `{subtask_id, content_delta, agent_id}`
3. Frontend Zustand store appends delta to `streamingMessages[subtask_id]`
4. On stream completion: publish SSE event `message` with full content, store in DB
5. Frontend promotes streaming message to permanent message, clears streaming state

| Component | Change Required |
|-----------|----------------|
| `internal/events/broker.go` | No change -- already supports publishing arbitrary events per task |
| `internal/models/event.go` | Add `token_delta` event type constant (string, no struct change needed) |
| `internal/executor/executor.go` | Streaming code path: iterate SDK stream, publish `token_delta` events during agent execution |
| `cmd/openaiagent/main.go` | Agent card: set `Capabilities.Streaming: true`. Handle `message/sendSubscribe` for A2A streaming. |
| `web/lib/sse.ts` | No change -- `onmessage` handler already parses any event type from JSON |
| `web/lib/store.ts` | Add `streamingMessages` map + `token_delta` handler that appends content |
| `web/components/chat/ChatMessage.tsx` | Show blinking cursor when message `isStreaming` (CSS class toggle) |

### 3. Tool Use Pipeline (Backend)

**Current:** `cmd/openaiagent` calls `client.ChatWithHistory()` -> gets text response -> returns as A2A artifact.

**New:** `cmd/openaiagent` calls SDK with tools -> handles `tool_calls` in response -> executes tools -> appends tool results -> calls SDK again -> returns final response as A2A artifact with tool call metadata.

| Component | Change Required |
|-----------|----------------|
| `cmd/openaiagent/openai.go` | Replace with SDK-based client, add `ChatWithTools()` method |
| `cmd/openaiagent/roles.go` | Add tool definitions per role (web_search for all roles; role-specific tools later) |
| NEW: `cmd/openaiagent/tools.go` | Tool registry: `Tool` interface + implementations. `web_search` as first tool. |
| `internal/a2a/discovery.go` | `AgentCard.Capabilities` already has `Streaming` field; set it based on agent capability |
| `internal/a2a/protocol.go` | Extend `Artifact` to support `type` field for tool call results (e.g., `"search_results"`) |
| SSE events | New event types: `tool_call_started`, `tool_call_completed` for real-time visibility |

**Tool interface pattern:**

```go
// Tool is the interface for agent tools.
type Tool struct {
    Name        string
    Description string
    Parameters  openai.FunctionParameters
    Execute     func(ctx context.Context, args json.RawMessage) (string, error)
}
```

### 4. Artifact Rendering (Frontend)

**Current:** `ChatMessage.tsx` hand-rolled markdown renderer; code fences render as plain `<pre>` blocks without highlighting.

**New:** Replace `renderMarkdown()` / `renderContent()` with `react-markdown` + `remark-gfm` + `rehype-highlight`. Add artifact card component for structured outputs.

| Component | Change Required |
|-----------|----------------|
| `web/components/chat/ChatMessage.tsx` | Replace `renderContent()` (lines 246-272) with `<ReactMarkdown>` component with plugins |
| NEW: `web/components/chat/ArtifactCard.tsx` | Rich card wrapper for structured agent outputs (search results, reports) |
| NEW: `web/components/chat/ToolCallBadge.tsx` | Inline indicator showing which tool was called and its status |
| `web/lib/types.ts` | Add `Artifact` interface for structured agent outputs, `ToolCall` interface |
| `web/app/layout.tsx` or global CSS | Import `highlight.js/styles/github-dark.css` for syntax theme |

### 5. Inbound Webhooks (Backend)

**Current:** Outbound webhooks exist (`internal/webhook/sender.go` for sending, `internal/handlers/webhooks.go` for CRUD). No inbound webhook receiver.

**New:** Add `POST /api/webhooks/inbound/{source}` endpoint that verifies HMAC signature and creates tasks from external events.

| Component | Change Required |
|-----------|----------------|
| NEW: `internal/handlers/webhooks_inbound.go` | Inbound webhook handler: read raw body, verify HMAC-SHA256 signature, parse payload, create task |
| `internal/webhook/sender.go` | No change (outbound stays as-is) |
| `cmd/server/main.go` | Register inbound webhook route |
| NEW: Database migration | `webhook_sources` table: id, name, source_type (github/slack/generic), secret, template_id, is_active, created_at |

**Inbound webhook verification pattern (mirrors existing outbound signing):**

```go
func verifySignature(body []byte, signature, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

**Critical implementation detail:** Read the raw request body BEFORE parsing JSON. If JSON is parsed and re-serialized, byte-level content may change and the HMAC comparison will fail. Use `io.ReadAll(r.Body)` first, verify signature, then `json.Unmarshal`.

---

## Environment Variables

| Variable | New/Existing | Default | Description |
|----------|-------------|---------|-------------|
| `OPENAI_API_KEY` | Existing | (none) | Used by SDK `openai.NewClient()` -- reads from env automatically |
| `OPENAI_MODEL` | Existing | `gpt-4o-mini` | Model for agent completions. SDK supports all model constants. |
| `SEARCH_API_KEY` | New | (none) | API key for web search provider (Brave Search recommended). Only needed if web_search tool is enabled. |
| `SEARCH_API_URL` | New | `https://api.search.brave.com/res/v1/web/search` | Web search endpoint. Defaults to Brave Search API. |

No changes to `DATABASE_URL`, `PORT`, `FRONTEND_URL`, or any other existing variables.

---

## Version Compatibility Matrix

| Dependency | Required By | Min Version | Our Version | Compatible |
|------------|-------------|-------------|-------------|------------|
| Go | openai-go/v3 | >= 1.22 | 1.26.1 | Yes |
| Node.js | remark-gfm 4.x | >= 16 | 22.x | Yes |
| react-markdown | remark-gfm 4.x | >= 9.0 | 10.1.0 | Yes |
| React | react-markdown 10.x | >= 18 | 19.2.3 | Yes |
| Next.js | rehype-highlight | any | 16.1.6 | Yes (ESM) |

---

## Final Summary: New Dependencies

| Layer | Package | Version | Install |
|-------|---------|---------|---------|
| Backend | `github.com/openai/openai-go/v3` | v3.30.0 | `go get github.com/openai/openai-go/v3@v3.30.0` |
| Frontend | `remark-gfm` | ^4.0.1 | `pnpm add remark-gfm` |
| Frontend | `rehype-highlight` | ^7.0.2 | `pnpm add rehype-highlight` |
| Frontend | `highlight.js` (if needed for CSS import) | ^11 | `pnpm add highlight.js` |

**Total new dependencies: 1 backend, 2-3 frontend.** Everything else uses existing libraries, Go stdlib, or extends existing code.

---

## Sources

- [openai/openai-go GitHub](https://github.com/openai/openai-go) -- Official Go SDK, v3.30.0 (2026-03-25)
- [openai/openai-go releases](https://github.com/openai/openai-go/releases) -- Version history verified
- [openai-go tool calling example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-tool-calling/main.go) -- Function calling pattern
- [openai-go streaming example](https://github.com/openai/openai-go/blob/main/examples/chat-completion-streaming/main.go) -- Streaming pattern
- [DeepWiki: openai-go streaming](https://deepwiki.com/openai/openai-go/3.2-streaming-responses) -- ChatCompletionAccumulator docs
- [OpenAI Function Calling guide](https://developers.openai.com/api/docs/guides/function-calling) -- Official docs
- [OpenAI Streaming guide](https://developers.openai.com/api/docs/guides/streaming-responses) -- Official docs
- [OpenAI Web Search tool](https://platform.openai.com/docs/guides/tools-web-search) -- Built-in web search (considered, rejected in favor of custom function call)
- [remark-gfm npm](https://www.npmjs.com/package/remark-gfm) -- v4.0.1, stable
- [remark-gfm GitHub](https://github.com/remarkjs/remark-gfm) -- GFM plugin
- [rehype-highlight npm](https://www.npmjs.com/package/rehype-highlight) -- v7.0.2
- [rehype-highlight GitHub](https://github.com/rehypejs/rehype-highlight) -- Documentation, language support
- [react-markdown GitHub](https://github.com/remarkjs/react-markdown) -- Plugin architecture, v10.x
- [Vercel ai-elements shiki issue #14](https://github.com/vercel/ai-elements/issues/14) -- Performance evidence against react-syntax-highlighter
- [Webhook HMAC verification](https://hookdeck.com/webhooks/guides/how-to-implement-sha256-webhook-signature-verification) -- Best practices
- [Webhook security patterns](https://webhooks.fyi/security/hmac) -- HMAC standard reference

---

*Stack research for v2.0 milestone: 2026-04-06*
