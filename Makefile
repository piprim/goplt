VERSION    := $(shell git describe --abbrev=0 2>/dev/null || echo "")
COMMIT     := $(shell git log --pretty=%h -1 2>/dev/null || echo "none")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
TARGET     := goplt
BIN        := ./bin/$(TARGET)
MODULE     := github.com/piprim/goplt
LDFLAGS    := -ldflags "\
  -X $(MODULE)/cmd/goplt/cmd.Version=$(VERSION) \
  -X $(MODULE)/cmd/goplt/cmd.Commit=$(COMMIT) \
  -X $(MODULE)/cmd/goplt/cmd.BuildDate=$(BUILD_DATE)"
GOARCH     := $(shell go env GOARCH)

ifeq ($(OS),Windows_NT)
    GOOS := windows
else
    UNAME_S := $(shell uname -s)
    ifeq ($(UNAME_S),Linux)
        GOOS := linux
    else ifeq ($(UNAME_S),Darwin)
        GOOS := darwin
    endif
endif

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@awk 'BEGIN {FS=":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	/^[a-zA-Z_\/-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the binary into ./bin/
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build $(LDFLAGS) -o $(BIN) ./cmd/goplt

GOBIN_DIR := $(shell go env GOBIN)
ifeq ($(GOBIN_DIR),)
    GOBIN_DIR := $(shell go env GOPATH)/bin
endif

.PHONY: install
install: build ## Install binary to GOBIN (or GOPATH/bin)
	cp $(BIN) $(GOBIN_DIR)/$(TARGET)

.PHONY: test
test: ## Run tests (skip network tests)
	go test -short ./...

.PHONY: test/all
test/all: ## Run all tests including network integration tests
	go test -timeout 120s ./...

.PHONY: test/cover
test/cover: ## Run tests with coverage report
	go test -short -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: clean
clean: ## Remove build artefacts
	rm -rf ./bin/ coverage.out
