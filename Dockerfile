# Stage 1: Build the Go application
FROM golang:1.26-alpine@sha256:9097beb5536220f7857bdcb65c1b4b340630dd7a70b85f03d5af29640b06693d AS builder

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN --mount=type=cache,id=keess-go-cache,target=/root/.cache/go-build \
    go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN --mount=type=cache,id=keess-go-cache,target=/root/.cache/go-build \
    go build -o keess .

# Stage 2: Build a small image
FROM alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

# Copy the binary from the builder stage
COPY --from=builder /build/keess /app/keess

# Command to run
ENTRYPOINT ["/app/keess", "run"]
