.PHONY: fmt fmt-check lint test coverage coverage-html build help

GO_ENV := GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache
LINT_ENV := $(GO_ENV) GOLANGCI_LINT_CACHE=/tmp/golangci-lint

fmt: ## Format Go source files with gofmt
	$(GO_ENV) go fmt ./...

fmt-check: ## Fail if any files need gofmt
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint: ## Run golangci-lint
	$(LINT_ENV) golangci-lint run

test: ## Run unit tests
	$(GO_ENV) go test ./...

coverage: ## Run tests and print coverage summary
	$(GO_ENV) go test ./... -coverpkg=./... -coverprofile=coverage.out
	$(GO_ENV) go tool cover -func=coverage.out

coverage-html: ## Run tests and generate HTML coverage report
	$(GO_ENV) go test ./... -coverpkg=./... -coverprofile=coverage.out
	$(GO_ENV) go tool cover -html=coverage.out -o coverage.html

build: ## Build the fic binary into bin/
	# reminding: mkdir -p is safe if bin already exists
	mkdir -p bin
	$(GO_ENV) go build -o bin/fic .

help: ## Show this help
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
