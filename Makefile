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
