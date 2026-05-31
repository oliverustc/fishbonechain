# Include environment variables from .env file
-include .env

# PHONY targets declaration
.PHONY: all test clean build deploy help install format \
        zk-test zk-build zk-clean zk-all

# Default target
all: clean build test

# zkSync specific targets
zk-all: zk-clean zk-build zk-test

# Help command
help:
	@echo "Available targets:"
	@echo "  all      - Clean, build and test (EVM)"
	@echo "  build    - Build the project (EVM)"
	@echo "  test     - Run tests (EVM)"
	@echo "  clean    - Remove build artifacts (EVM)"
	@echo "  zk-all   - Clean, build and test (zkSync)"
	@echo "  zk-build - Build the project (zkSync)"
	@echo "  zk-test  - Run tests (zkSync)"
	@echo "  zk-clean - Remove build artifacts (zkSync)"
	@echo "  format   - Format code"

# EVM targets
clean:
	forge clean

build:
	forge clean && forge build

test:
	forge clean && forge test

# zkSync targets
zk-clean:
	zforge clean

zk-build:
	zforge clean && zforge build --zksync

zk-test:
	zforge clean && zforge test --zksync


chg:
	forge test --match-test "(^testGroth16Range|^testGroth16Substr|^testGroth16Subset)" | grep PASS

zk-chg:
	zforge test --zksync --match-test "(^testGroth16Range|^testGroth16Substr|^testGroth16Subset)" | grep PASS

rog:
	forge test --match-test "(^testGroth16Root)" | grep PASS

zk-rog:
	zforge test --zksync --match-test "(^testGroth16Root)" | grep PASS

# Development tools
format:
	forge fmt

# Default target when no arguments provided
.DEFAULT_GOAL := help
