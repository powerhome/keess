FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf

# Pre-built binary, not compiled here. GoReleaser is the single source of
# truth for how `keess` is built (see .goreleaser.yaml):
#   - release: GoReleaser's dockers_v2 stages it at $TARGETPLATFORM/keess.
#   - local: `make docker-build` runs `goreleaser build` and passes
#     KEESS_BIN via --build-arg.
ARG TARGETPLATFORM
ARG KEESS_BIN=${TARGETPLATFORM}/keess

COPY ${KEESS_BIN} /app/keess

ENTRYPOINT ["/app/keess", "run"]
