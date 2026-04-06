.PHONY: build test lint images run clean dev web-install web-dev web-build

# --- Production: single binary serves everything ---

# Build Next.js static export + Go binary
build: web-build
	go build -o bin/zynqel-core ./cmd/zynqel-core

# Build and run (dashboard + API on :8080)
run: build images
	./bin/zynqel-core

# --- Development: hot-reload frontend + Go backend ---

# Run both servers (Next.js :3000 + Go :8080)
dev:
	@echo "Starting Go backend on :8080 and Next.js on :3000..."
	@echo "Open http://localhost:3000/console"
	@trap 'kill 0' EXIT; \
		go run ./cmd/zynqel-core & \
		cd web && npm run dev & \
		wait

# --- CI ---

test:
	go test -race ./... -timeout 600s

lint:
	gofmt -s -l .
	go vet ./...
	$(shell go env GOPATH)/bin/golangci-lint run ./...

# --- Docker images ---

images:
	docker build -t zynqel-base:latest images/base/
	docker build -t zynqel-claude:latest images/claude/
	docker build -t zynqel-opencode:latest images/opencode/
	docker build -t zynqel-codex:latest images/codex/
	docker build -t zynqel-qwen:latest images/qwen/

# --- Web dashboard ---

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

# --- Cleanup ---

clean:
	rm -rf bin/
	rm -rf web/.next/ web/out/
	docker rm -f $$(docker ps --filter "label=zynqel.managed=true" -q) 2>/dev/null || true
	docker volume rm $$(docker volume ls --filter "name=zynqel-ws-" -q) 2>/dev/null || true
	docker rmi $$(docker images --filter "reference=zynqel-ws-*" -q) 2>/dev/null || true
