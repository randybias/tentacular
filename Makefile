REGISTRY   := ghcr.io/randybias
IMAGE      := $(REGISTRY)/tentacular-engine
TAG        ?= latest
PLATFORMS  := linux/amd64,linux/arm64

.PHONY: help build build-local push test lint clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## ── Engine Image ────────────────────────────────────────────────────────────

build: ## Multi-arch build and push to GHCR (linux/amd64 + linux/arm64)
	docker buildx build \
		--platform $(PLATFORMS) \
		--file engine/Dockerfile \
		--tag $(IMAGE):$(TAG) \
		--tag $(IMAGE):$(shell git rev-parse --short HEAD) \
		--push \
		.

build-local: ## Single-arch build into local daemon (no push, for testing)
	docker build \
		--file engine/Dockerfile \
		--tag $(IMAGE):local \
		.

## ── CLI Binary ──────────────────────────────────────────────────────────────

build-cli: ## Build tntc binary for the current platform (output: ./tntc)
	go build -o tntc \
		-ldflags "-s -w \
		  -X github.com/randybias/tentacular/pkg/version.Version=$$(git describe --tags --always --dirty) \
		  -X github.com/randybias/tentacular/pkg/version.Commit=$$(git rev-parse --short HEAD) \
		  -X github.com/randybias/tentacular/pkg/version.Date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		./cmd/tntc

release: ## Cut a release via GoReleaser (requires GITHUB_TOKEN and a version tag)
	goreleaser release --clean

release-snapshot: ## Dry-run release build without publishing
	goreleaser release --snapshot --clean

## ── Development ─────────────────────────────────────────────────────────────

test: ## Run Go tests
	go test ./...

test-engine: ## Run Deno engine tests
	cd engine && deno test --allow-net --allow-read --allow-env

lint: ## Run Go linter
	golangci-lint run ./...

## ── Auth ────────────────────────────────────────────────────────────────────

login: ## Login to GHCR using gh CLI token
	gh auth token | docker login ghcr.io -u randybias --password-stdin

## ── Cleanup ─────────────────────────────────────────────────────────────────

clean: ## Remove local build artifacts
	rm -rf .tentacular/base-image.txt
	docker rmi $(IMAGE):local 2>/dev/null || true
