FROM alpine:3.24.3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a

# Pre-built binary, not compiled here. GoReleaser is the single source of
# truth for how `keess` is built (see .goreleaser.yaml):
#   - release: GoReleaser's dockers_v2 stages it at $TARGETPLATFORM/keess.
#   - local: `make docker-build` runs `goreleaser build` and passes
#     KEESS_BIN via --build-arg.
ARG TARGETPLATFORM
ARG KEESS_BIN=${TARGETPLATFORM}/keess

COPY ${KEESS_BIN} /app/keess

ENTRYPOINT ["/app/keess", "run"]
