# lazytempo — common developer tasks.
# Run `make help` for a list of targets.

BINARY      := lazytempo
PKG         := ./cmd/lazytempo
BIN_DIR     := bin
BIN         := $(BIN_DIR)/$(BINARY)
GO          ?= go

# Inject a version string from git when available.
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)

.DEFAULT_GOAL := help

.PHONY: build
build: ## Build the binary into ./bin
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)
	@echo "built $(BIN) ($(VERSION))"

.PHONY: run
run: ## Build and run the TUI
	$(GO) run $(PKG)

.PHONY: install
install: ## Install the binary into $GOBIN (or $GOPATH/bin)
	$(GO) install -ldflags '$(LDFLAGS)' $(PKG)
	@dest="$$($(GO) env GOBIN)"; [ -n "$$dest" ] || dest="$$($(GO) env GOPATH)/bin"; \
		echo "installed $(BINARY) to $$dest"

.PHONY: test
test: ## Run all tests
	$(GO) test ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format all Go source
	$(GO) fmt ./...

.PHONY: tidy
tidy: ## Tidy go.mod/go.sum
	$(GO) mod tidy

.PHONY: demo
demo: ## Record the demo GIF into assets/ (needs vhs)
	./demo/record.sh

.PHONY: check
check: fmt vet test ## Format, vet and test

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)
	$(GO) clean

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
