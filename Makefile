.PHONY: dev dev-backend dev-frontend build build-backend build-frontend \
       lint lint-backend lint-frontend check test clean

# ─── Development ─────────────────────────────────────────────
dev: dev-backend dev-frontend

dev-backend:
	go run ./cmd/server

dev-frontend:
	cd web && pnpm dev

# ─── Build ───────────────────────────────────────────────────
build: build-backend build-frontend

build-backend:
	go build -o server ./cmd/server

build-frontend:
	cd web && pnpm build

# ─── Lint ────────────────────────────────────────────────────
lint: lint-backend lint-frontend

lint-backend:
	golangci-lint run ./...

lint-frontend:
	cd web && pnpm lint

# ─── Format ──────────────────────────────────────────────────
fmt: fmt-backend fmt-frontend

fmt-backend:
	gofmt -w cmd/ internal/

fmt-frontend:
	cd web && pnpm prettier --write .

fmt-check: fmt-check-backend fmt-check-frontend

fmt-check-backend:
	@test -z "$$(gofmt -l cmd/ internal/)" || (echo "Go files need formatting:" && gofmt -l cmd/ internal/ && exit 1)

fmt-check-frontend:
	cd web && pnpm prettier --check .

# ─── Check (pre-push gate) ──────────────────────────────────
# Runs all quality gates: format, lint, type-check, build
check: fmt-check lint typecheck build

typecheck:
	cd web && pnpm tsc --noEmit

# ─── Test ────────────────────────────────────────────────────
test: test-backend test-frontend

test-backend:
	go test ./...

test-frontend:
	cd web && pnpm test

# ─── Database ────────────────────────────────────────────────
db-create:
	createdb taskhub 2>/dev/null || true

db-reset:
	dropdb taskhub 2>/dev/null || true
	createdb taskhub

# ─── Clean ───────────────────────────────────────────────────
clean:
	rm -f server
	rm -rf web/.next web/out

# ─── Install ─────────────────────────────────────────────────
install: install-backend install-frontend install-hooks

install-backend:
	go mod download

install-frontend:
	cd web && pnpm install

install-hooks:
	@command -v pre-commit >/dev/null 2>&1 && pre-commit install || echo "pre-commit not installed, skipping hooks setup"

# ─── Docker ──────────────────────────────────────────────────
docker-build:
	docker build -t taskhub .

docker-run:
	docker run --env-file .env -p 8080:8080 taskhub
