# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in TaskHub, please report it responsibly.

**Email:** security@taskhub.dev (or open a private GitHub security advisory)

**Include:**
- Description of the vulnerability
- Steps to reproduce
- Affected component (backend API, frontend, agent system)
- Potential impact

We will acknowledge reports within 48 hours and aim to resolve critical issues within 7 days.

## Trust Model

### Architecture boundaries

TaskHub operates with these trust boundaries:

1. **Backend API** — Trusted. Runs on server, has database access and LLM API keys.
2. **Frontend** — Untrusted input source. All user input validated at handler boundaries.
3. **LLM Responses** — Semi-trusted. Agent outputs are displayed but not executed as code.
4. **Database** — Trusted store. Accessed only through parameterized queries.

### What is in scope

- SQL injection via API endpoints
- Authentication/authorization bypass
- Secrets exposure (API keys, database credentials)
- Cross-site scripting (XSS) in rendered agent output
- Server-side request forgery (SSRF)
- Unauthorized access to other users' tasks/channels

### What is out of scope

- Prompt injection that only affects LLM output quality (not a security boundary)
- Denial of service via excessive task creation (rate limiting not yet implemented)
- Issues requiring physical access to the server
- Vulnerabilities in dependencies without a demonstrated exploit path in TaskHub

## Security Practices

### Code

- All database queries use parameterized statements (pgx)
- No SQL string concatenation
- User input validated at handler boundaries
- CORS restricted to specific origins
- Secrets loaded from environment variables, never committed

### Infrastructure

- `.env` files excluded via `.gitignore`
- `detect-secrets` runs in pre-commit hooks
- Private keys detected by pre-commit hooks
- CI runs `go vet` and `golangci-lint` with security-relevant checks (`sqlclosecheck`, `bodyclose`)

### Agent System

- Agent outputs are text-only — no code execution
- LLM API keys are server-side only, never exposed to frontend
- Audit logger tracks all LLM calls with cost/token metadata
