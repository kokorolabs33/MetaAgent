# ─── Stage 1: Build Go backend ───────────────────────────────
FROM golang:1.23-bookworm AS backend-build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# ─── Stage 2: Build Next.js frontend ────────────────────────
FROM node:22-bookworm-slim AS frontend-build

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

# ─── Stage 3: Runtime ───────────────────────────────────────
FROM node:22-bookworm-slim AS runtime

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app

# Copy Go binary
COPY --from=backend-build /server /app/server

# Copy Next.js standalone output
COPY --from=frontend-build /app/web/.next/standalone /app/web
COPY --from=frontend-build /app/web/.next/static /app/web/.next/static
COPY --from=frontend-build /app/web/public /app/web/public

EXPOSE 8080 3000

# Start both services
COPY <<'EOF' /app/start.sh
#!/bin/sh
set -e
/app/server &
cd /app/web && node server.js &
wait
EOF
RUN chmod +x /app/start.sh

CMD ["/app/start.sh"]
