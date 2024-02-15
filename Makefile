# Project variables
PROJECT_NAME := "keess"
DOCKER_IMAGE_NAME := "keess"
DOCKER_TAG := "latest"

# Go related variables
GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin

.PHONY: build test docker-build coverage run docker-run help

# Build the project
build:
	@echo "Building $(PROJECT_NAME)..."
	@GOBIN=$(GOBIN) go build -o $(GOBIN)/$(PROJECT_NAME) $(GOBASE)

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."	
	@docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_TAG) .


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

# Help
help:
	@echo "Makefile commands:"
	@echo "build        - Build the project"
	@echo "test         - Run tests"
	@echo "docker-build - Build Docker image"
	@echo "coverage     - Generate and view code coverage report"
	@echo "run          - Run the application"
	@echo "docker-run   - Run the Docker image with .kube directory mounted"
