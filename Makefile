.PHONY: fmt fmt-check lint test coverage coverage-html build help

fmt: ## Format Go source files with gofmt
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go fmt ./...

fmt-check: ## Fail if any files need gofmt
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

lint: ## Run golangci-lint
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache GOLANGCI_LINT_CACHE=/tmp/golangci-lint golangci-lint run

test: ## Run unit tests
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go test ./...

coverage: ## Run tests and print coverage summary
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go test ./... -coverpkg=./... -coverprofile=coverage.out
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go tool cover -func=coverage.out

coverage-html: ## Run tests and generate HTML coverage report
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go test ./... -coverpkg=./... -coverprofile=coverage.out
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go tool cover -html=coverage.out -o coverage.html

build: ## Build the fic binary into bin/
	# reminding: mkdir -p is safe if bin already exists
	mkdir -p bin
	GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build -o bin/fic .

help: ## Show this help
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
