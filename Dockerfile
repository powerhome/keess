# Stage 1: Build the Go application
FROM golang:1.25-alpine@sha256:77dd832edf2752dafd030693bef196abb24dcba3a2bc3d7a6227a7a1dae73169 AS builder

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
FROM alpine@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1

# Copy the binary from the builder stage
COPY --from=builder /build/keess /app/keess

# Command to run
ENTRYPOINT ["/app/keess", "run"]
