# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Change these variables as necessary.
MAIN_PACKAGE_PATH := ./
BINARY_NAME := db-archiving
GATEWAY_BINARY_NAME=db-archiving
GATEWAY_PKG_PATH=./
OUTPUT_DIR=./tmp/bin

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

.PHONY: no-dirty
no-dirty:
	git diff --exit-code

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	$(GOFMT) ./...
	$(GOMOD) tidy -v

## audit: run quality control checks (vet, staticcheck, vulncheck)
.PHONY: audit
audit: 
	$(GOVET) ./...
	@echo "Audit checks passed."

# ==================================================================================== #
# PRODUCTION BUILDING
# ==================================================================================== #

## production/build: build production Linux AMD64 binaries
.PHONY: production/build
production/build: tidy
	@echo "Building Linux AMD64 binary for Gateway..."
	mkdir -p $(OUTPUT_DIR)/linux_amd64
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags='-s -w' -o=$(OUTPUT_DIR)/linux_amd64/$(GATEWAY_BINARY_NAME) $(GATEWAY_PKG_PATH)

