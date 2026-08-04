# Used only by GoReleaser (.goreleaser.yaml): copies the pre-built binary that
# GoReleaser stages in the build context at <os>/<arch>/keess.
# The main Dockerfile remains the from-source build for local flows.
FROM alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

ARG TARGETPLATFORM

COPY $TARGETPLATFORM/keess /app/keess

ENTRYPOINT ["/app/keess", "run"]
