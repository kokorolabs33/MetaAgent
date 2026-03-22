# Code Review Skill

Use this skill when reviewing code changes, PRs, or when asked to review work.

## Review Checklist

For every review, check these in order:

### 1. Correctness

- Does the code do what it claims?
- Are edge cases handled (nil, empty, error paths)?
- For Go: are all errors checked and propagated?
- For TypeScript: are types correct (no `any` escapes)?

### 2. Type Contract

If models or API types changed:
- [ ] Go struct JSON tags match TypeScript interfaces
- [ ] SQL migration added for new columns
- [ ] API client (`web/lib/api.ts`) updated
- [ ] SSE event types updated if new events added

### 3. Security

- No SQL string concatenation (must use parameterized queries)
- No secrets or API keys in code
- User input validated at handler boundaries
- CORS not over-broadened

### 4. Architecture

- Handlers are thin — business logic in dedicated packages
- No circular dependencies between internal packages
- New endpoints registered in `cmd/server/main.go`
- State management through Zustand store, not prop drilling

### 5. Quality

- Code formatted (`gofmt` / eslint)
- No unused imports or variables
- Brief comments for non-obvious logic only
- Consistent naming with existing code

## Bug Fix Reviews

Bug fixes MUST include evidence before merge:

1. **Symptom** — What was broken? (error message, wrong behavior, repro steps)
2. **Root cause** — Which file/function, and WHY it failed (not just WHERE)
3. **Fix** — The fix touches the implicated code path
4. **Regression guard** — Test added, or clear reason why not

Do NOT approve bug fixes that only show "it works now" without explaining the root cause.

## Review Response Format

Structure your review as:

```
## Summary
One-line: what this change does.

## Issues Found
- [severity] file:line — description

## Suggestions
- file:line — optional improvement (not blocking)

## Verdict
APPROVE / REQUEST_CHANGES / COMMENT
```

Severity levels:
- **[critical]** — Must fix. Bug, security issue, data loss risk.
- **[major]** — Should fix. Logic error, missing error handling, type unsafety.
- **[minor]** — Nice to fix. Style, naming, minor improvement.
