 # Force bash instead of sh
 SHELL := /bin/bash

# Packages to be included in tests
# NOTE1: Make will replace '\n' with ' ' characters. See NOTE2
TEST_PKG := $(shell go list ./... | grep -v 'test\|internal\|rpc') 

# Build target (default)
.PHONY: build
build:
	@go build
	
# Clean target
.PHONY: clean
clean:
	@go clean

# Test everything
.PHONY: test
test: unit integration

# Unit testing only
.PHONY: unit
unit:
	@echo "Running unit tests"
	@go test -count=1 $(TEST_PKG) -coverprofile="unit.out" -covermode=count

# Integration testing only
# NOTE2: sed command replaces ' ' characters with ',' and then removes the last ',' to
# eliminate warning about packages not tested
.PHONY: integration
integration:
	@echo "Running integration tests"
	@go test -count=1 ./test/... -coverpkg="$(shell echo "$(TEST_PKG)" | sed 's/ /,/g;s/.$$//')" -coverprofile="integration.out" -covermode=count