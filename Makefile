.PHONY: build test lint images run clean web-dev web-build web-install

# Build the Go binary
build:
	go build -o bin/zynqel-core ./cmd/zynqel-core

# Run all tests with race detector
test:
	go test -race ./... -timeout 600s

# Run CI checks (same as GitHub Actions)
lint:
	gofmt -s -l .
	go vet ./...
	$(shell go env GOPATH)/bin/golangci-lint run ./...

# Build all Docker images
images:
	docker build -t zynqel-base:latest images/base/
	docker build -t zynqel-claude:latest images/claude/
	docker build -t zynqel-qwen:latest images/qwen/

# Build and run locally
run: build images
	./bin/zynqel-core

# --- Web dashboard ---

# Install web dependencies
web-install:
	cd web && npm install

# Run Next.js dev server (port 3000, proxies to Go on 8080)
web-dev:
	cd web && npm run dev

# Build Next.js static export → web/out/
web-build:
	cd web && npm run build

# Build everything: web static export + Go binary
build-all: web-build build

# Clean build artifacts and Docker resources
clean:
	rm -rf bin/
	rm -rf web/.next/ web/out/
	docker rm -f $$(docker ps --filter "label=zynqel.managed=true" -q) 2>/dev/null || true
	docker volume rm $$(docker volume ls --filter "name=zynqel-ws-" -q) 2>/dev/null || true
	docker rmi $$(docker images --filter "reference=zynqel-ws-*" -q) 2>/dev/null || true
