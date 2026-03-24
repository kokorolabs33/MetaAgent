# Build Go binary
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /llmagent ./cmd/llmagent

# Runtime with Node.js + Claude CLI
FROM node:22-alpine
RUN apk add --no-cache ca-certificates
RUN npm install -g @anthropic-ai/claude-code
COPY --from=builder /llmagent /llmagent

# Auth token will be set via environment variable at container start
# Usage: docker run -e CLAUDE_TOKEN=<token> ...
# The entrypoint script sets up auth then starts the agent
COPY cmd/llmagent/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
