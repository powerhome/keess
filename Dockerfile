FROM alpine@sha256:5b02b42e375f7426f8d65c3af331ca05d9878f9989230354504e0b9dfd431f60

# Pre-built binary, not compiled here. GoReleaser is the single source of
# truth for how `keess` is built (see .goreleaser.yaml):
#   - release: GoReleaser's dockers_v2 stages it at $TARGETPLATFORM/keess.
#   - local: `make docker-build` runs `goreleaser build` and passes
#     KEESS_BIN via --build-arg.
ARG TARGETPLATFORM
ARG KEESS_BIN=${TARGETPLATFORM}/keess

COPY ${KEESS_BIN} /app/keess

ENTRYPOINT ["/app/keess", "run"]
