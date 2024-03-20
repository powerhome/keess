# Project variables
PROJECT_NAME := "keess"
DOCKER_IMAGE_NAME := "keess"
DOCKER_TAG := "latest"

# Go related variables
GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin

.PHONY: build test docker-build coverage run docker-run create-local-clusters delete-local-clusters local-docker-run local-test help

# Build the project
build:
	@echo "Building $(PROJECT_NAME)..."
	@GOBIN=$(GOBIN) go build -o $(GOBIN)/$(PROJECT_NAME) $(GOBASE)

# Run tests
gotest:
	@echo "Running tests..."
	@go test ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) .

# Install requirements and run test.py
test:
	@echo "Installing requirements..."
	@pip3 install -r requirements.txt
	@echo "Running test.py..."
	@python3 test.py

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
	@./bin/keess run --localCluster=$(LOCAL_CLUSTER) --logLevel=debug

# Target to run the Docker image with the .kube directory mounted
docker-run:
	@echo "Running Docker image with .kube directory mounted..."
	@docker run --rm -it -v ${HOME}/.kube:/root/.kube $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) run --localCluster=$(LOCAL_CLUSTER) --logLevel=debug

LOCAL_TEST_KUBECONFIG_FILE := "localTestKubeconfig"
# Target to start local kube clusters for testing purposes
create-local-clusters:
	@kind create cluster --image=kindest/node:v1.22.17 -n source-cluster --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE)
	@kind create cluster --image=kindest/node:v1.22.17 -n destination-cluster --kubeconfig $(LOCAL_TEST_KUBECONFIG_FILE)

# Target to delete local kube clusters
delete-local-clusters:
	@kind delete clusters source-cluster destination-cluster

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
			--remoteCluster=kind-destination-cluster \
			--kubeConfigPath=/root/.kube/config \
			--pollingInterval=10 \
			--housekeepingInterval=10 \
			--logLevel=debug

# Test locally using kind
local-test:
	@echo "Building keess-test image"
	@docker build -f Dockerfile.localTest -t keess-test:1.0 .
	@echo "Running tests"
	@docker run \
		-it \
		--rm \
		--mount type=bind,source="./$(LOCAL_TEST_KUBECONFIG_FILE)",target=/root/.kube/config,readonly \
		--network host \
		--name keess-test \
		keess-test:1.0 \
		python test.py

# Help
help:
	@echo "Makefile commands:"
	@echo "build                 - Build the project"
	@echo "test                  - Run tests"
	@echo "docker-build          - Build Docker image"
	@echo "coverage              - Generate and view code coverage report"
	@echo "run                   - Run the application"
	@echo "docker-run            - Run the Docker image with .kube directory mounted"
	@echo "create-local-clusters - Create 2 clusters locally using Kind"
	@echo "delete-local-clusters - Delete the 2 local clusters created with Kind"
	@echo "local-docker-run      - Run the application locally using docker and pointing to the local cluster created with Kind"
	@echo "local-test            - Run the tests pointing to the local cluster created with Kind"
