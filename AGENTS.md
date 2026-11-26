# keess

Keess syncs Secrets, ConfigMaps and Services across namespaces and clusters, driven by `keess.powerhrg.com/*` labels and annotations. See `README.md` for usage and `docs/` for design notes.

## Layout

- `main.go`, `cmd/`: CLI entrypoint (cobra). `cmd/version.go` holds a `"dev"` placeholder that GoReleaser overrides at build time.
- `pkg/keess/`: sync logic.
- `chart/`: Helm chart deployed to the PAC clusters.
- `tests/e2e/`: Ginkgo e2e tests against local kind clusters. `test.py` holds the Python e2e tests (`make tests-python-e2e`, which runs them in Docker via `Dockerfile.localTest`).
- `Dockerfile.kubeconfig-reloader`: the companion kubeconfig-reloader image.

## Build and test

- Go version: `.tool-versions`.
- `make build`, `make tests` (unit), `make tests-e2e` (needs `make setup-local-clusters` first), `make tests-all`.
- CI (`.github/workflows/ci.yml`) runs golangci-lint (`.golangci.yml`), `helm lint chart/` and the tests. Run the linter locally before pushing.

## Versioning and releases

Read `docs/versioning.md` before touching `chart/Chart.yaml` or cutting a release.

- `appVersion` decides the image that ships: `chart/values.yaml` leaves `image.tag` empty. Never pin an image tag or digest in `chart/values.yaml`.
- Any chart change bumps the chart `version`. A chart-only change bumps the chart patch only. The next app change moves both to the same new version.
- Release with `make tag-release VERSION=x.y.z`. If the chart isn't at that version on `main`, it opens a bump PR first.

## Pull requests

- PR titles follow Conventional Commits (`feat:`, `fix:`, `chore(deps):` ...).
- Fill in `.github/PULL_REQUEST_TEMPLATE.md`, including how you tested on alpha.

## Agent configuration

This repo follows Power's agent harness layout: `AGENTS.md` holds the instructions, `CLAUDE.md` imports it, and rules, commands and skills live in `.agents/`. `.claude/` and `.cursor/` hold only symlinks into `.agents/`. Skills: `.agents/skills/review-renovate-pr` reviews Renovate PRs.
