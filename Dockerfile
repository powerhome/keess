FROM alpine@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

# Pre-built binary, not compiled here. GoReleaser is the single source of
# truth for how `keess` is built (see .goreleaser.yaml):
#   - release: GoReleaser's dockers_v2 stages it at $TARGETPLATFORM/keess.
#   - local: `make docker-build` runs `goreleaser build` and passes
#     KEESS_BIN via --build-arg.
ARG TARGETPLATFORM
ARG KEESS_BIN=${TARGETPLATFORM}/keess

COPY ${KEESS_BIN} /app/keess

ENTRYPOINT ["/app/keess", "run"]
