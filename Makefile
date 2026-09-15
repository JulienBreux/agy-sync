BIN_DIR=./bin
BIN=./bin/agy-sync
CONFIG_FILE_LOCAL=config.yaml.dist
CONFIG_FILE=/root/.config/agy-sync/config.yaml

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -trimpath -ldflags "-s -w -X github.com/julienbreux/agy-sync/cmd.Version=$(VERSION) -X github.com/julienbreux/agy-sync/cmd.Commit=$(COMMIT) -X github.com/julienbreux/agy-sync/cmd.Date=$(DATE)"

.DEFAULT_GOAL := help

generate: ## Run go generate
	go generate ./...

lint: ## Lint code
	golangci-lint run

test: ## Test packages
	go test -count=1 -failfast -cover -coverprofile=coverage.txt -v ./...

test-race: ## Test packages with data race detector
	go test -count=1 -race -failfast ./...

vulncheck: ## Scan dependencies for known vulnerabilities
	go tool govulncheck ./...

coverage: test ## Test coverage with default output
	go tool cover -func=coverage.txt

coverage-total: coverage ## Get the total percentage of lines covered by tests
	go tool cover -func=coverage.txt | fgrep total | awk '{print substr($$3, 1, length($$3)-1)}'

coverage-html: coverage ## Test coverage with html output
	go tool cover -html=coverage.txt -o coverage.html

clean: ## Clean project
	rm -Rf ${BIN_DIR}
	rm -Rf coverage.txt coverage.html

build: clean ## Build local binary
	mkdir -p ${BIN_DIR}
	go build $(LDFLAGS) -o ${BIN} .

build-image: ## Build local Docker image
	docker build -t ghcr.io/julienbreux/agy-sync:latest .

run: build ## Run local binary
	${BIN}

run-container: ## Run prepared local container
	docker run --rm -v $(PWD)/${CONFIG_FILE_LOCAL}:${CONFIG_FILE} ghcr.io/julienbreux/agy-sync:latest

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: generate lint test test-race vulncheck coverage coverage-total coverage-html clean build build-image run run-container help
