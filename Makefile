SHELL := pwsh.exe
.SHELLFLAGS := -NoProfile -Command
.ONESHELL:

GO ?= go
CMD_PATH ?= ./cmd/ai-sync
OUTPUT_DIR ?= bin
CLI_NAME ?= ai-sync
GOEXE ?= .exe

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
	@New-Item -ItemType Directory -Force -Path "$(OUTPUT_DIR)" | Out-Null
	@echo "Building CLI: $(BINARY_PATH)"
	@$(GO) build -buildvcs=false -o "$(BINARY_PATH)" "$(CMD_PATH)"

run: build
	@& "$(BINARY_PATH)" $(ARGS)

test:
	@$(GO) test ./...

fmt:
	@$(GO) fmt ./...

clean:
	@if (Test-Path "$(OUTPUT_DIR)") { Remove-Item -Recurse -Force "$(OUTPUT_DIR)" }
