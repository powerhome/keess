# Project variables
PROJECT_NAME := "keess"
DOCKER_IMAGE_NAME := "keess"
RELOADER_IMAGE_NAME := "keess-kubeconfig-reloader"
DOCKER_TAG := "latest"
# this file will host kubeconfig for local clusters created with kind
LOCAL_TEST_KUBECONFIG_FILE := "localTestKubeconfig"
# this file will host the same kubeconfig, but to be used within the clusters themselves
LOCAL_INTERNAL_TEST_KUBECONFIG_FILE := "localInternalTestKubeconfig"
LOCAL_CLUSTER := "kind-source-cluster"
KIND_K8S_VERSION := v1.34.11
CILIUM_CLI_VERSION := v0.19.7
CILIUM_VERSION := v1.19.1
OS := $(shell uname | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/;s/aarch64/arm64/')

# Go related variables
GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin

.PHONY: build docker-build coverage run docker-run create-local-clusters delete-local-clusters install-cilium-cli install-cilium-to-clusters install-keess setup-local-clusters setup-local-clusters-with-keess local-docker-run tests tests-e2e tests-python-e2e tag-release bump-chart-version help

# Build the project
build:
	@echo "Building $(GOBIN)/$(PROJECT_NAME)..."
	mkdir -p $(GOBIN)
	GOBIN=$(GOBIN) go build -o $(GOBIN)/$(PROJECT_NAME) $(GOBASE)

# Build Docker image. Requires the goreleaser CLI (`brew install goreleaser`):
# it's the source of truth for how the keess binary is built (see .goreleaser.yaml).
docker-build:
	@echo "Building keess binary via goreleaser..."
	mkdir -p bin
	GOOS=linux GOARCH=$(ARCH) goreleaser build --single-target --snapshot --clean --id keess -o bin/keess-linux-$(ARCH)
	@echo "Building Docker image..."
	@docker build --build-arg KEESS_BIN=bin/keess-linux-$(ARCH) -t $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) .
	@echo "Building kubeconfig-reloader Docker image..."
	@docker build -f Dockerfile.kubeconfig-reloader -t $(RELOADER_IMAGE_NAME):$(DOCKER_TAG) .

## Tag and push a release tag, triggering the GoReleaser release workflow which
## builds and publishes both the keess and kubeconfig-reloader images to Docker Hub.
## VERSION sets both app and chart version; use APP_VERSION/CHART_VERSION to set them
## independently (e.g. a chart-only bump ahead of the next app version, see docs/versioning.md).
## If chart/Chart.yaml on main isn't already at those versions, this opens a bump PR
## (via bump-chart-version) and stops -- merge it, then re-run the same command to tag.
## Run it with:
##   make tag-release VERSION=1.4.2 [IGNORE_BRANCH_CHECK=true]
##   make tag-release APP_VERSION=1.4.2 CHART_VERSION=1.4.3
tag-release:
	@_APP_VERSION="$(if $(APP_VERSION),$(APP_VERSION),$(VERSION))"; \
	_CHART_VERSION="$(if $(CHART_VERSION),$(CHART_VERSION),$(VERSION))"; \
	if [ -z "$$_APP_VERSION" ] || [ -z "$$_CHART_VERSION" ]; then \
		echo "Set VERSION (both), or APP_VERSION/CHART_VERSION individually (e.g. make tag-release VERSION=1.4.2)"; exit 1; \
	fi; \
	for v in "$$_APP_VERSION" "$$_CHART_VERSION"; do \
		if ! echo "$$v" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
			echo "Versions must be bare semver, e.g. 1.4.2 (this repo's tags have no 'v' prefix): $$v"; exit 1; \
		fi; \
	done; \
	if [ "$(IGNORE_BRANCH_CHECK)" != "true" ]; then \
		CURRENT_BRANCH=$$(git rev-parse --abbrev-ref HEAD); \
		if [ "$$CURRENT_BRANCH" != "main" ]; then \
			echo "Error: You must be on the 'main' branch to create releases. Current branch: $$CURRENT_BRANCH"; \
			echo "Use IGNORE_BRANCH_CHECK=true to skip this check."; \
			exit 1; \
		fi; \
	fi; \
	CURRENT_APP_VERSION=$$(awk -F'"' '/^appVersion:/ {print $$2}' chart/Chart.yaml); \
	CURRENT_CHART_VERSION=$$(awk -F'[: ]+' '/^version:/ {print $$2}' chart/Chart.yaml); \
	if [ "$$CURRENT_APP_VERSION" != "$$_APP_VERSION" ] || [ "$$CURRENT_CHART_VERSION" != "$$_CHART_VERSION" ]; then \
		echo "chart/Chart.yaml on main is at appVersion=$$CURRENT_APP_VERSION version=$$CURRENT_CHART_VERSION, not $$_APP_VERSION/$$_CHART_VERSION."; \
		echo "Opening a PR to bump it..."; \
		$(MAKE) bump-chart-version APP_VERSION=$$_APP_VERSION CHART_VERSION=$$_CHART_VERSION; \
		echo "Merge that PR, then re-run: make tag-release APP_VERSION=$$_APP_VERSION CHART_VERSION=$$_CHART_VERSION"; \
		exit 1; \
	fi; \
	git tag $$_APP_VERSION -m "Release $$_APP_VERSION"; \
	git push origin $$_APP_VERSION

## Open a PR bumping chart/Chart.yaml's version and appVersion. Called automatically by
## tag-release when they're out of date, or run directly for a chart-only bump ahead of
## the next app version (see docs/versioning.md). Requires the `gh` CLI.
## Run it with:
##   make bump-chart-version APP_VERSION=1.4.2 CHART_VERSION=1.4.2
bump-chart-version:
	@if [ -z "$(APP_VERSION)" ] || [ -z "$(CHART_VERSION)" ]; then \
		echo "APP_VERSION and CHART_VERSION must both be set (e.g. make bump-chart-version APP_VERSION=1.4.2 CHART_VERSION=1.4.2)"; exit 1; \
	fi
	@for v in "$(APP_VERSION)" "$(CHART_VERSION)"; do \
		if ! echo "$$v" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$'; then \
			echo "Versions must be bare semver, e.g. 1.4.2: $$v"; exit 1; \
		fi; \
	done
	@BRANCH="chore/$(APP_VERSION)"; \
	git fetch origin main && \
	git checkout -B "$$BRANCH" origin/main && \
	sed -i.bak -E 's/^version: .*/version: $(CHART_VERSION)/' chart/Chart.yaml && \
	sed -i.bak -E 's/^appVersion: .*/appVersion: "$(APP_VERSION)"/' chart/Chart.yaml && \
	rm -f chart/Chart.yaml.bak && \
	git add chart/Chart.yaml && \
	git commit -m "chore(chart): bump to app $(APP_VERSION), chart $(CHART_VERSION)" && \
	git push origin "$$BRANCH" && \
	gh pr create --base main --head "$$BRANCH" \
		--title "chore(chart): bump to app $(APP_VERSION), chart $(CHART_VERSION)" \
		--body "Bumps chart/Chart.yaml ahead of tagging release $(APP_VERSION)."

# New target for code coverage
coverage:
	@echo "Generating code coverage..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Opening code coverage report in browser..."
	@open coverage.html

# Target to execute the application
run: build
	@echo "Running the application..."
	@$(GOBIN)/$(PROJECT_NAME) run --localCluster=$(LOCAL_CLUSTER) --logLevel=debug --kubeConfigPath=$(LOCAL_TEST_KUBECONFIG_FILE) --pollingInterval=10 --housekeepingInterval=10 --namespacePollingInterval=10

# Target to run the Docker image with the .kube directory mounted
docker-run:
	@echo "Running Docker image with .kube directory mounted..."
	@docker run --rm -it -v ${HOME}/.kube:/root/.kube $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) run --localCluster=$(LOCAL_CLUSTER) --logLevel=debug

# Target to start local kube clusters for testing purposes
create-local-clusters:
	kind create cluster --image=kindest/node:$(KIND_K8S_VERSION) -n source-cluster --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --config tests/kind-config-1.yaml
	kind create cluster --image=kindest/node:$(KIND_K8S_VERSION) -n destination-cluster --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --config tests/kind-config-2.yaml
	KUBECONFIG=$(LOCAL_INTERNAL_TEST_KUBECONFIG_FILE) kind export kubeconfig -n source-cluster --internal
	KUBECONFIG=$(LOCAL_INTERNAL_TEST_KUBECONFIG_FILE) kind export kubeconfig -n destination-cluster --internal


# Target to delete local kube clusters
delete-local-clusters:
	@kind delete clusters source-cluster destination-cluster

$(GOBIN)/cilium:
	mkdir -p $(GOBIN)
	curl -sL https://github.com/cilium/cilium-cli/releases/download/$(CILIUM_CLI_VERSION)/cilium-$(OS)-$(ARCH).tar.gz | tar -xzf - -C $(GOBIN);
	chmod +x $(GOBIN)/cilium

install-cilium-cli: $(GOBIN)/cilium

# Install Cilium on local clusters
install-cilium-to-clusters: install-cilium-cli
	$(GOBIN)/cilium install --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster --version $(CILIUM_VERSION) --set cluster.id=1 --set cluster.name=source-cluster || true
	$(GOBIN)/cilium install --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-destination-cluster --version $(CILIUM_VERSION) --set cluster.id=2 --set cluster.name=destination-cluster || true
	$(GOBIN)/cilium status --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster --wait
	$(GOBIN)/cilium status --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-destination-cluster --wait
	$(GOBIN)/cilium clustermesh enable --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster --service-type NodePort
	$(GOBIN)/cilium clustermesh enable --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-destination-cluster --service-type NodePort
	$(GOBIN)/cilium clustermesh status --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster --wait
	$(GOBIN)/cilium clustermesh status --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-destination-cluster --wait
	KUBECONFIG=$(LOCAL_TEST_KUBECONFIG_FILE) $(GOBIN)/cilium clustermesh connect --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster  --destination-context kind-destination-cluster
	$(GOBIN)/cilium clustermesh status --wait --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster
	$(GOBIN)/cilium clustermesh status --wait --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-destination-cluster

install-keess: build docker-build
	@echo "Installing or updating Keess on local clusters..."
	kind load docker-image $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) --name source-cluster
	helm upgrade --install keess chart \
		--kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --kube-context kind-source-cluster \
		--namespace keess --create-namespace \
		--set localCluster=kind-source-cluster \
		--set-file config.kubeconfigContent=$(LOCAL_INTERNAL_TEST_KUBECONFIG_FILE) \
		--values tests/helm-values.yaml
	kind load docker-image $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) --name destination-cluster
	helm upgrade --install keess chart \
		--kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --kube-context kind-destination-cluster \
		--namespace keess --create-namespace \
		--set localCluster=kind-destination-cluster \
		--set-file config.kubeconfigContent=$(LOCAL_INTERNAL_TEST_KUBECONFIG_FILE) \
		--values tests/helm-values.yaml
	@echo "Make sure we restart Keess pods to apply changes..."
	kubectl --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-source-cluster -n keess delete pod --all
	kubectl --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE) --context kind-destination-cluster -n keess delete pod --all

# Fully prepare local clusters for testing with Keess running outside of the clusters (with make run)
setup-local-clusters: create-local-clusters install-cilium-to-clusters

# Fully prepare local clusters for testing with Keess running inside the clusters
setup-local-clusters-with-keess: setup-local-clusters install-keess

# Target to run the Docker image with local test kubeconfig mounted
local-docker-run:
	@echo "Running Docker image with $(LOCAL_TEST_KUBECONFIG_FILE) mounted..."
	@docker run \
	  --rm \
		-it \
		-v ./$(LOCAL_TEST_KUBECONFIG_FILE):/root/.kube/config \
		--network host \
		$(DOCKER_IMAGE_NAME):$(DOCKER_TAG) \
		run \
		  --localCluster=kind-source-cluster \
			--kubeConfigPath=/root/.kube/config \
			--pollingInterval=10 \
			--housekeepingInterval=10 \
			--namespacePollingInterval=10 \
			--logLevel=debug

# Run unit tests only (excludes e2e tests in /tests directory)
tests:
	@echo "Running Unit tests..."
	go test $(shell go list ./... | grep -v /tests)

# Run e2e tests (requires local clusters to be running)
tests-e2e:
	@echo "Running e2e tests..."
	@echo "Make sure local clusters are running (use 'make setup-local-clusters' if needed)"
	@cd tests/e2e && ginkgo -v

# Original e2e tests on python
tests-python-e2e:
	@echo "Running python e2e tests on docker ..."
	@echo "Building keess-test image"
	docker build -f Dockerfile.localTest -t keess-test:1.0 .
	@echo "Run container ..."
	docker run \
		-it \
		--rm \
		--mount type=bind,source="./$(LOCAL_TEST_KUBECONFIG_FILE)",target=/root/.kube/config,readonly \
		--network host \
		--name keess-test \
		keess-test:1.0 \
		python test.py

# Run all tests (unit + e2e)
tests-all: tests tests-e2e tests-python-e2e


# Help
help:
	@echo "--------------------------------"
	@echo "## Most used Makefile commands:"
	@echo "--------------------------------"
	@echo "build                           - Build the project"
	@echo "docker-build                    - Build Docker image"
	@echo "coverage                        - Generate and view code coverage report"
	@echo "run                             - Run the application from local machine build"
	@echo "setup-local-clusters            - Create 2 clusters locally using Kind ready for testing for PAC-V2 (includes Cilium)"
	@echo "setup-local-clusters-with-keess - Create 2 clusters locally with Keess running inside the clusters"
	@echo "tests                           - Run Unit tests only"
	@echo "tests-e2e                       - Run e2e tests (cluster sync focused, requires local clusters)"
	@echo "tests-python-e2e                - Run python e2e tests using docker (namespace sync focused)"
	@echo "tests-all                       - Run all tests (unit + e2e)"
	@echo "delete-local-clusters           - Delete the 2 local clusters created with Kind"
	@echo "tag-release                     - Tag and push a release (VERSION=x.y.z, or APP_VERSION=/CHART_VERSION=), triggering the GoReleaser image release"
	@echo "bump-chart-version              - Open a PR bumping chart/Chart.yaml (APP_VERSION=/CHART_VERSION=)"
	@echo "--------------------------------"
	@echo "## Other Makefile commands:"
	@echo "--------------------------------"
	@echo "create-local-clusters           - Create 2 clusters locally using Kind"
	@echo "install-cilium-cli              - Install Cilium CLI on the local machine"
	@echo "install-cilium-to-clusters      - Install Cilium on the local clusters"
	@echo "install-keess                   - Install Keess on local clusters using Helm"
	@echo "docker-run                      - Run the Docker image with .kube directory mounted"
	@echo "local-docker-run                - Run the application locally using docker and pointing to the local cluster created with Kind"
