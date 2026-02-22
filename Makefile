REGISTRY   := ghcr.io/randybias
IMAGE      := $(REGISTRY)/tentacular-engine
TAG        ?= latest
PLATFORMS  := linux/amd64,linux/arm64
DOCKER     ?= docker

.PHONY: help build build-local push test lint clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## ── Engine Image ────────────────────────────────────────────────────────────
## Primary build path: .github/workflows/build-engine.yml (GHA, triggers on
## push to main when engine/** changes). The targets below are for local
## one-off builds only — not required for normal development.

build: ## [local only] Multi-arch build and push to GHCR (linux/amd64 + linux/arm64)
	$(DOCKER) buildx build \
		--platform $(PLATFORMS) \
		--file engine/Dockerfile \
		--tag $(IMAGE):$(TAG) \
		--tag $(IMAGE):$(shell git rev-parse --short HEAD) \
		--push \
		.

build-local: ## [local only] Single-arch build into local daemon (no push, for testing)
	$(DOCKER) build \
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

release: ## Tag and push to trigger GHA release workflow (usage: make release TAG=v0.1.0)
	@test -n "$(TAG)" || (echo "usage: make release TAG=v0.1.0" && exit 1)
	echo "$(TAG)" > stable.txt
	git add stable.txt
	git commit -m "release: $(TAG)"
	git tag $(TAG)
	git push origin main $(TAG)

release-snapshot: ## Local dry-run build without publishing (requires goreleaser)
	goreleaser release --snapshot --clean

## ── Development ─────────────────────────────────────────────────────────────

test: ## Run Go tests
	go test ./...

test-engine: ## Run Deno engine tests
	cd engine && deno test --allow-net --allow-read --allow-env

lint: ## Run Go linter
	golangci-lint run ./...

## ── Auth ────────────────────────────────────────────────────────────────────

login: ## [local only] Login to GHCR using gh CLI token
	gh auth token | $(DOCKER) login ghcr.io -u randybias --password-stdin

## ── Cleanup ─────────────────────────────────────────────────────────────────

clean: ## Remove local build artifacts
	rm -rf .tentacular/base-image.txt dist/
	$(DOCKER) rmi $(IMAGE):local 2>/dev/null || true
