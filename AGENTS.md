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

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

These rules apply only when the `graphify` CLI is installed. The graph is built locally at session start and is not committed. Skip the rules if `graphify` is missing.

Rules:
- When graphify-out/graph.json exists, run `graphify query "<question>"` before your first grep, search or source-file read for a codebase question, and put this rule in every subagent prompt that explores code. Read raw files only for lines the graph or an earlier read already located. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. When the answer is under a Markdown heading, `graphify section "<heading>"` prints that section's text, so the .md need not be opened. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
