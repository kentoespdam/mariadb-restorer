# MariaDB Restorer — Development Makefile
# =========================================
# Common commands for building, testing, and running the TUI.
#
# Quick reference:
#   make build       — build binary to ./mariadb-restorer
#   make run         — build + run TUI in demo mode
#   make test        — run all tests
#   make lint        — run golangci-lint
#   make clean       — remove build artifacts

BINARY   := mariadb-restorer
MAIN_PKG := ./cmd/mariadb-restorer/
GO       ?= go

# Detect operating system for cross-platform support.
UNAME_S := $(shell uname -s)

.PHONY: all build run test lint vet fmt tidy clean demo help install-tools

all: clean fmt vet build test lint  ## Full quality gate (default target)

# --- Build ---

build:  ## Build the binary
	$(GO) build -o $(BINARY) $(MAIN_PKG)

build-race:  ## Build with race detector enabled
	$(GO) build -race -o $(BINARY) $(MAIN_PKG)

build-all:  ## Build all packages (no binary output)
	$(GO) build ./...

# --- Run ---

run: build  ## Build and launch TUI
	./$(BINARY) tui

demo: build  ## Build and launch TUI in demo mode
	./$(BINARY) tui --demo

# --- Test ---

test:  ## Run all tests with race detector
	$(GO) test -v -race -count=1 ./...

test-short:  ## Run tests without race detector (faster)
	$(GO) test -count=1 ./...

test-tui:  ## Run only TUI package tests
	$(GO) test -v -count=1 ./internal/tui/...

test-engine:  ## Run only restore-engine tests
	$(GO) test -v -count=1 ./internal/restore-engine/...

test-vault:  ## Run only credential-vault tests
	$(GO) test -v -count=1 ./internal/credential-vault/...

test-cover:  ## Run tests with coverage report
	$(GO) test -v -race -count=1 -coverprofile=coverage.out ./... && \
	$(GO) tool cover -func=coverage.out | tail -1 && \
	$(GO) tool cover -html=coverage.out -o coverage.html

# --- Lint & Vet ---

lint:  ## Run golangci-lint
	golangci-lint run --timeout=5m ./...

vet:  ## Run go vet
	$(GO) vet ./...

# --- Code Quality ---

fmt:  ## Format Go source code
	$(GO) fmt ./...

tidy:  ## Tidy and verify go.mod
	$(GO) mod tidy
	$(GO) mod verify

check: fmt vet lint test  ## Full quality gate (fmt → vet → lint → test)

# --- Clean ---

clean:  ## Remove build artifacts and coverage data
	rm -f $(BINARY)
	rm -f coverage.out coverage.html
	rm -rf /tmp/mariadb-restorer-*

# --- Utilities ---

version:  ## Show Go version and module info
	$(GO) version
	$(GO) env GOMOD

help:  ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*##' Makefile | sort | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

info:  ## Show project metadata
	@echo "Binary:  $(BINARY)"
	@echo "Module:  $(shell head -1 go.mod)"
	@echo "Go:      $(shell $(GO) version)"
	@echo "OS:      $(UNAME_S)"

install-tools:  ## Install dev dependencies (golangci-lint, entr)
ifeq ($(UNAME_S),Linux)
	@command -v golangci-lint >/dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin
	@command -v entr >/dev/null && echo "entr: found" || echo "Tip: sudo apt install entr   (for 'make watch')"
else
	@command -v golangci-lint >/dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin
	@command -v entr >/dev/null && echo "entr: found" || echo "Tip: brew install entr   (for 'make watch')"
endif

# --- Development shortcuts ---

watch:  ## Rebuild on source changes (requires entr or fswatch)
	@echo "Watching for changes... (Ctrl-C to stop)"
	-find . -name '*.go' -not -path '*/vendor/*' | entr -r sh -c '$(GO) build -o $(BINARY) $(MAIN_PKG) && echo "--- rebuilt $$(date +%H:%M:%S) ---"'

# --- Release ---

release: clean fmt vet test lint build  ## Build a production-ready binary
	@echo "Release build complete: ./$(BINARY)"
	@ls -lh $(BINARY)
