GO ?= go
CMD_PATH ?= ./cmd/ai-sync
OUTPUT_DIR ?= bin
CLI_NAME ?= ai-sync
GOEXE ?= .exe
ARGS ?= --help

BINARY_NAME := $(CLI_NAME)$(GOEXE)
BINARY_PATH := $(OUTPUT_DIR)/$(BINARY_NAME)

.PHONY: help build run test fmt clean

help:
	@echo "AI Sync Manager - available targets:"
	@echo ""
	@echo "  make build                  - build the default CLI binary"
	@echo "  make build CLI_NAME=my-cli - build with a custom CLI name"
	@echo "  make run ARGS=scan         - build and run the CLI"
	@echo "  make test                  - run Go tests"
	@echo "  make fmt                   - run go fmt"
	@echo "  make clean                 - remove the bin directory"
	@echo ""
	@echo "Current output: $(BINARY_PATH)"

build:
	@echo "Building CLI: $(BINARY_PATH)"
	@$(GO) build -buildvcs=false -o "$(BINARY_PATH)" "$(CMD_PATH)"

run: build
	@"$(BINARY_PATH)" $(ARGS)

test:
	@$(GO) test ./...

fmt:
	@$(GO) fmt ./...

clean:
	@echo "Cleaning $(OUTPUT_DIR)..."
	@if exist "$(OUTPUT_DIR)" rmdir /s /q "$(OUTPUT_DIR)"
