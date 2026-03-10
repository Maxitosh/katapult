SHELL := /bin/bash

ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
KATAPULT_DIR := $(ROOT_DIR)katapult
ENVTEST_K8S_VERSION ?= 1.35.x
E2E_CONTROLPLANE_IMAGE ?= katapult-controlplane:test
E2E_AGENT_IMAGE ?= katapult-agent:test

# Local dev environment
LOCAL_CLUSTER_NAME ?= katapult-dev
LOCAL_CONTROLPLANE_IMAGE ?= katapult-controlplane:local
LOCAL_AGENT_IMAGE ?= katapult-agent:local
LOCAL_WEB_IMAGE ?= katapult-web:local
WORKERS ?= 2
COMPONENT ?= controlplane

.PHONY: help test test-unit test-integration test-e2e test-all envtest-path check-integration-deps check-e2e-deps check-e2e-images build-e2e-images build-e2e-controlplane-image build-e2e-agent-image \
	local-up local-down local-rebuild local-check-prereqs local-images local-cluster local-deploy local-seed local-summary

help:
	@echo "Available targets:"
	@echo "  make test              - Run default/unit test tier"
	@echo "  make test-integration  - Run integration tests (auto-resolves envtest assets)"
	@echo "  make test-e2e          - Run e2e tests (requires prebuilt local images)"
	@echo "  make build-e2e-images  - Build local images required by e2e tests"
	@echo "  make test-all          - Run all test tiers sequentially"
	@echo "  make envtest-path      - Print resolved KUBEBUILDER_ASSETS path"
	@echo ""
	@echo "Local dev environment:"
	@echo "  make local-up          - Provision full local dev environment"
	@echo "  make local-down        - Tear down local dev environment"
	@echo "  make local-rebuild     - Rebuild a component (COMPONENT=controlplane|agent|webui)"

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

# =============================================================================
# Local Dev Environment
# @cpt:impl cpt-katapult-flow-local-dev-env-provision
# =============================================================================

local-up: local-check-prereqs local-images local-cluster local-deploy local-seed local-summary

# @cpt:impl cpt-katapult-flow-local-dev-env-teardown
local-down:
	@if ! kind get clusters 2>/dev/null | grep -q "^$(LOCAL_CLUSTER_NAME)$$"; then \
		echo "Cluster $(LOCAL_CLUSTER_NAME) does not exist, nothing to do."; \
		exit 0; \
	fi
	kind delete cluster --name "$(LOCAL_CLUSTER_NAME)"
	@echo ""
	@echo "=== Katapult Local Dev Environment Torn Down ==="
	@echo "Cluster $(LOCAL_CLUSTER_NAME) deleted."

# @cpt:impl cpt-katapult-flow-local-dev-env-rebuild
local-rebuild:
	@if ! kind get clusters 2>/dev/null | grep -q "^$(LOCAL_CLUSTER_NAME)$$"; then \
		echo "ERROR: Cluster $(LOCAL_CLUSTER_NAME) does not exist. Run 'make local-up' first."; \
		exit 1; \
	fi
	@case "$(COMPONENT)" in \
		controlplane) \
			docker build -t "$(LOCAL_CONTROLPLANE_IMAGE)" \
				-f "$(KATAPULT_DIR)/build/Dockerfile.controlplane" "$(KATAPULT_DIR)" && \
			kind load docker-image "$(LOCAL_CONTROLPLANE_IMAGE)" --name "$(LOCAL_CLUSTER_NAME)" && \
			kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout restart deployment/katapult-controlplane -n katapult-system && \
			kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status deployment/katapult-controlplane -n katapult-system --timeout=120s ;; \
		agent) \
			docker build -t "$(LOCAL_AGENT_IMAGE)" \
				-f "$(KATAPULT_DIR)/build/Dockerfile.agent" "$(KATAPULT_DIR)" && \
			kind load docker-image "$(LOCAL_AGENT_IMAGE)" --name "$(LOCAL_CLUSTER_NAME)" && \
			kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout restart daemonset/katapult-agent -n katapult-system && \
			kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status daemonset/katapult-agent -n katapult-system --timeout=120s ;; \
		webui) \
			docker build -t "$(LOCAL_WEB_IMAGE)" \
				--build-arg VITE_API_TOKEN=test-operator-token \
				-f "$(ROOT_DIR)web/Dockerfile" "$(ROOT_DIR)web" && \
			kind load docker-image "$(LOCAL_WEB_IMAGE)" --name "$(LOCAL_CLUSTER_NAME)" && \
			kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout restart deployment/katapult-web -n katapult-system && \
			kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status deployment/katapult-web -n katapult-system --timeout=120s ;; \
		*) \
			echo "ERROR: Unknown COMPONENT=$(COMPONENT). Use: controlplane, agent, or webui"; \
			exit 1 ;; \
	esac
	@echo "$(COMPONENT) rebuilt and restarted."

local-check-prereqs:
	@for bin in kind kubectl docker; do \
		command -v "$$bin" >/dev/null 2>&1 || { echo "ERROR: $$bin not found in PATH"; exit 1; }; \
	done
	@echo "Prerequisites OK"

local-images:
	docker build -t "$(LOCAL_CONTROLPLANE_IMAGE)" \
		-f "$(KATAPULT_DIR)/build/Dockerfile.controlplane" "$(KATAPULT_DIR)"
	docker build -t "$(LOCAL_AGENT_IMAGE)" \
		-f "$(KATAPULT_DIR)/build/Dockerfile.agent" "$(KATAPULT_DIR)"
	docker build -t "$(LOCAL_WEB_IMAGE)" \
		--build-arg VITE_API_TOKEN=test-operator-token \
		-f "$(ROOT_DIR)web/Dockerfile" "$(ROOT_DIR)web"

# @cpt:impl cpt-katapult-algo-local-dev-env-provision-kind
local-cluster:
	@if kind get clusters 2>/dev/null | grep -q "^$(LOCAL_CLUSTER_NAME)$$"; then \
		echo "Cluster $(LOCAL_CLUSTER_NAME) already exists, skipping creation."; \
		exit 0; \
	fi
	@TMPFILE=$$(mktemp) && \
	echo "kind: Cluster" > "$$TMPFILE" && \
	echo "apiVersion: kind.x-k8s.io/v1alpha4" >> "$$TMPFILE" && \
	echo "nodes:" >> "$$TMPFILE" && \
	echo "  - role: control-plane" >> "$$TMPFILE" && \
	echo "    extraPortMappings:" >> "$$TMPFILE" && \
	echo "      - containerPort: 30080" >> "$$TMPFILE" && \
	echo "        hostPort: 30080" >> "$$TMPFILE" && \
	echo "        protocol: TCP" >> "$$TMPFILE" && \
	echo "      - containerPort: 30081" >> "$$TMPFILE" && \
	echo "        hostPort: 30081" >> "$$TMPFILE" && \
	echo "        protocol: TCP" >> "$$TMPFILE" && \
	for i in $$(seq 1 $(WORKERS)); do \
		echo "  - role: worker" >> "$$TMPFILE"; \
	done && \
	kind create cluster --name "$(LOCAL_CLUSTER_NAME)" --config "$$TMPFILE" --wait 120s && \
	rm -f "$$TMPFILE"

# @cpt:impl cpt-katapult-algo-local-dev-env-deploy-services
# @cpt:impl cpt-katapult-algo-local-dev-env-deploy-stack
local-deploy:
	kind load docker-image "$(LOCAL_CONTROLPLANE_IMAGE)" --name "$(LOCAL_CLUSTER_NAME)"
	kind load docker-image "$(LOCAL_AGENT_IMAGE)" --name "$(LOCAL_CLUSTER_NAME)"
	kind load docker-image "$(LOCAL_WEB_IMAGE)" --name "$(LOCAL_CLUSTER_NAME)"
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" apply -k "$(ROOT_DIR)deploy/local"
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status deployment/postgres -n katapult-system --timeout=120s
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status deployment/minio -n katapult-system --timeout=120s
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status deployment/katapult-controlplane -n katapult-system --timeout=120s
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout restart daemonset/katapult-agent -n katapult-system
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status daemonset/katapult-agent -n katapult-system --timeout=120s
	kubectl --context "kind-$(LOCAL_CLUSTER_NAME)" rollout status deployment/katapult-web -n katapult-system --timeout=120s

# @cpt:impl cpt-katapult-flow-local-dev-env-seed
local-seed:
	kubectl config use-context "kind-$(LOCAL_CLUSTER_NAME)"
	API_BASE="http://localhost:30080" API_TOKEN="test-operator-token" \
		bash "$(ROOT_DIR)scripts/local-seed-data.sh"

local-summary:
	@echo ""
	@echo "=== Katapult Local Dev Environment ==="
	@echo "Cluster:  $(LOCAL_CLUSTER_NAME)"
	@echo "API:      http://localhost:30080"
	@echo "Web UI:   http://localhost:30081"
	@echo "Token:    test-operator-token (role: operator)"
	@echo ""
	@echo "Teardown: make local-down"
	@echo "Rebuild:  make local-rebuild COMPONENT=controlplane|agent|webui"
