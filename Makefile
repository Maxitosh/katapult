SHELL := /bin/bash

ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
KATAPULT_DIR := $(ROOT_DIR)katapult
ENVTEST_K8S_VERSION ?= 1.35.x
E2E_CONTROLPLANE_IMAGE ?= katapult-controlplane:test
E2E_AGENT_IMAGE ?= katapult-agent:test

.PHONY: help test test-unit test-integration test-e2e test-all envtest-path check-integration-deps check-e2e-deps check-e2e-images build-e2e-images build-e2e-controlplane-image build-e2e-agent-image

help:
	@echo "Available targets:"
	@echo "  make test              - Run default/unit test tier"
	@echo "  make test-integration  - Run integration tests (auto-resolves envtest assets)"
	@echo "  make test-e2e          - Run e2e tests (requires prebuilt local images)"
	@echo "  make build-e2e-images  - Build local images required by e2e tests"
	@echo "  make test-all          - Run all test tiers sequentially"
	@echo "  make envtest-path      - Print resolved KUBEBUILDER_ASSETS path"

test: test-unit

test-unit:
	cd "$(KATAPULT_DIR)" && go test ./...

envtest-path:
	@command -v setup-envtest >/dev/null 2>&1 || { \
		echo "setup-envtest not found. Install controller-runtime setup-envtest first."; \
		exit 1; \
	}
	@setup-envtest use "$(ENVTEST_K8S_VERSION)" -p path

check-integration-deps:
	@command -v docker >/dev/null 2>&1 || { echo "docker not found in PATH"; exit 1; }
	@command -v setup-envtest >/dev/null 2>&1 || { \
		echo "setup-envtest not found. Install controller-runtime setup-envtest first."; \
		exit 1; \
	}

test-integration: check-integration-deps
	cd "$(KATAPULT_DIR)" && \
	KUBEBUILDER_ASSETS="$$(setup-envtest use "$(ENVTEST_K8S_VERSION)" -p path)" \
	go test -tags integration ./...

check-e2e-deps:
	@for bin in kind kubectl docker; do \
		command -v "$$bin" >/dev/null 2>&1 || { echo "$$bin not found in PATH"; exit 1; }; \
	done

check-e2e-images:
	@docker image inspect "$(E2E_CONTROLPLANE_IMAGE)" >/dev/null 2>&1 || { \
		echo "missing local image: $(E2E_CONTROLPLANE_IMAGE)"; \
		echo "run 'make build-e2e-images' before running e2e tests"; \
		exit 1; \
	}
	@docker image inspect "$(E2E_AGENT_IMAGE)" >/dev/null 2>&1 || { \
		echo "missing local image: $(E2E_AGENT_IMAGE)"; \
		echo "run 'make build-e2e-images' before running e2e tests"; \
		exit 1; \
	}

build-e2e-images: check-e2e-deps build-e2e-controlplane-image build-e2e-agent-image

build-e2e-controlplane-image:
	docker build \
		-t "$(E2E_CONTROLPLANE_IMAGE)" \
		-f "$(KATAPULT_DIR)/build/Dockerfile.controlplane" \
		"$(KATAPULT_DIR)"

build-e2e-agent-image:
	docker build \
		-t "$(E2E_AGENT_IMAGE)" \
		-f "$(KATAPULT_DIR)/build/Dockerfile.agent" \
		"$(KATAPULT_DIR)"

test-e2e: check-e2e-deps check-e2e-images
	cd "$(KATAPULT_DIR)" && go test -tags e2e ./...

test-all: test-unit test-integration test-e2e
