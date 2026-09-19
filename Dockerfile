FROM alpine@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# Pre-built binary, not compiled here. GoReleaser is the single source of
# truth for how `keess` is built (see .goreleaser.yaml):
#   - release: GoReleaser's dockers_v2 stages it at $TARGETPLATFORM/keess.
#   - local: `make docker-build` runs `goreleaser build` and passes
#     KEESS_BIN via --build-arg.
ARG TARGETPLATFORM
ARG KEESS_BIN=${TARGETPLATFORM}/keess

COPY ${KEESS_BIN} /app/keess

ENTRYPOINT ["/app/keess", "run"]
