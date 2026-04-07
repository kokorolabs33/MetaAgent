# Milestones

## v2.0 Wow Moment (Shipped: 2026-04-07)

**Phases completed:** 4 phases, 8 plans, 15 tasks

**Key accomplishments:**

- OpenAI function calling loop with Tavily web search and per-role tool sets in the openaiagent binary
- Real-time SSE tool call events flowing from OpenAI agent through A2A polling to inline chat feed status indicators
- react-markdown with GFM tables and syntax-highlighted code blocks replacing hand-rolled renderer, plus artifact type contracts and executor metadata pipeline
- Rich artifact renderers for search results, code, tables, and data with copy/download actions wired into chat message flow
- Streaming OpenAI client with token batching and platform relay endpoint for real-time agent.streaming_delta SSE events
- Zustand streaming buffer with progressive react-markdown rendering and blinking cursor for real-time agent token display
- HMAC-SHA256 verified webhook ingestion with GitHub/Slack/generic parsers, dual-secret rotation, idempotency deduplication, and LLM content sanitization
- Tabbed webhook management UI with inbound CRUD, secret reveal on create, copy-to-clipboard, and provider-specific setup instructions for GitHub/Slack/Generic

---
