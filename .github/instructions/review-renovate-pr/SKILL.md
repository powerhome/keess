---
name: review-renovate-pr
description: Use when reviewing a Renovate dependency update PR for the keess repository. Accepts a PR number or GitHub PR URL. Produces a risk-tiered dependency review, breaking change cross-reference against the Go source, build and test validation, and a SAFE TO MERGE / REVIEW NEEDED / BLOCKED verdict in chat.
---

# review-renovate-pr

Reviews a Renovate PR for the keess repository. Classifies every dependency change by risk tier, fetches release notes and cross-references changed APIs against the Go source, runs build and test commands, and prints a structured verdict report in chat.

## Invocation

```
/review-renovate-pr <PR number>
/review-renovate-pr <GitHub PR URL>
```

If given a URL, extract the PR number from the final path segment of `.../pull/<number>`.

## Step 1: Fetch PR data

```bash
gh pr view <number> --json title,number,headRefName,baseRefName
gh pr diff <number>
```

From the diff, group every changed file by type:

| File pattern              | Dependency type         |
|---------------------------|-------------------------|
| `go.mod`                  | Go dependencies         |
| `go.sum` only             | Lock file maintenance   |
| `Dockerfile*`             | Docker base images      |
| `.github/workflows/*.yml` | GitHub Actions          |
| `Jenkinsfile`             | ci-kubed library        |
| `chart/**`                | Helm chart              |

If only `go.sum` changed with no `go.mod` change, classify the entire PR as Tier 3 lock file maintenance and skip to Step 8.

## Step 2: Parse dependency changes from the diff

**go.mod changes.** For each module line present on both the removed (`-`) and added (`+`) sides:

```
-	k8s.io/client-go v0.33.0
+	k8s.io/client-go v0.34.2
```

Extract: module path, old version, new version. The line has no `// indirect` suffix for direct deps; it has `// indirect` for indirect deps.

**Dockerfile changes.** For each `FROM` line that changed, extract image name, old tag/digest, new tag/digest. A bump is digest-only if the image name and tag are identical on both sides and only the `@sha256:...` portion changed.

**GitHub Actions changes.** For each `uses:` line that changed, extract action name and old vs new SHA or version reference.

**Jenkinsfile changes.** Look for changes to the line `library 'github.com/powerhome/ci-kubed@...'`.

## Step 3: Assign a risk tier to each dependency

**Tier 1 — deep analysis:**
- Any `k8s.io/*` direct dependency at any semver jump
- Any direct Go dependency where the major version number increased (e.g. v2.x to v3.0.0)

**Tier 2 — standard analysis:**
- These direct Go runtime deps at minor or patch bump: `go.uber.org/zap`, `github.com/prometheus/client_golang`, `github.com/spf13/cobra`, `github.com/spf13/viper`, `github.com/fsnotify/fsnotify`
- ci-kubed Jenkinsfile library at any semver jump
- GitHub Actions changes (this repo pins actions by SHA so every bump is a real code change)
- Docker base image changes that are not digest-only

**Tier 3 — confirmation only:**
- Any Go indirect dependency
- Docker digest-only bumps
- Go toolchain directive changes (`toolchain goX.Y.Z` in go.mod)
- go.sum or lock file only commits
- These test framework deps at any semver jump: `github.com/onsi/ginkgo`, `github.com/onsi/gomega`, `github.com/stretchr/testify`

## Step 4: Constraint check (direct Go deps only)

Read `renovate.json`. The keess config extends `group:allNonMajor`, meaning Renovate groups all non-major updates into one PR and creates separate PRs for major bumps. If any direct Go dependency in this PR crossed a major version boundary (old major != new major), include this section in the report and end it with BLOCKED. Skip the rest of the report.

The major version is the integer between `v` and the first dot (e.g., `v0.34.2` → major is `0`, `v3.1.0` → major is `3`). For modules that encode their major version in the path (e.g., `github.com/onsi/ginkgo/v2`), a major bump would appear in the diff as the old path being removed and a new path (e.g., `.../v3`) being added rather than a version change on the same line — treat this pattern as a major version boundary as well.

If no direct Go deps crossed a major version boundary, write: "All direct Go dependency versions satisfy the group:allNonMajor constraint."

## Step 5: Tier 1 analysis

For each Tier 1 dependency, run these steps.

**Find what this repo imports from the changed module.** The keess codebase uses these k8s.io sub-packages (confirmed at time of writing — re-run the grep to get the current list):

For `k8s.io/client-go`:
```bash
grep -rh 'k8s.io/client-go' pkg/ cmd/ main.go --include='*.go' \
  | grep -oE '"k8s\.io/client-go[^"]*"' | sort -u
```

For `k8s.io/api`:
```bash
grep -rh 'k8s.io/api' pkg/ cmd/ main.go --include='*.go' \
  | grep -oE '"k8s\.io/api[^"]*"' | sort -u
```

For `k8s.io/apimachinery`:
```bash
grep -rh 'k8s.io/apimachinery' pkg/ cmd/ main.go --include='*.go' \
  | grep -oE '"k8s\.io/apimachinery[^"]*"' | sort -u
```

**Fetch release notes.** GitHub repositories for each `k8s.io` module:
- `k8s.io/api` → `https://github.com/kubernetes/api/releases`
- `k8s.io/apimachinery` → `https://github.com/kubernetes/apimachinery/releases`
- `k8s.io/client-go` → `https://github.com/kubernetes/client-go/releases`

For modules with a `github.com/` prefix, the GitHub URL is the module path directly (e.g. `github.com/foo/bar` → `https://github.com/foo/bar/releases`).

Fetch the release page for the new version. Look for sections labelled: BREAKING CHANGES, API Changes, Incompatibilities, Removed, Renamed.

**Cross-reference removed or renamed symbols.** For each symbol found in those sections:
```bash
grep -rn '<OldSymbolName>' pkg/ cmd/ main.go --include='*.go'
```

Report every `file:line` hit alongside what the symbol changed to upstream.

If no breaking changes are found in the release notes sections for the imported sub-packages: write "no breaking changes found in used API surface".

## Step 6: Tier 2 analysis

For each Tier 2 dependency, run these steps.

**Fetch release notes** for the version range between old and new version. GitHub URLs:
- `go.uber.org/zap` → `https://github.com/uber-go/zap/releases`
- `github.com/prometheus/client_golang` → `https://github.com/prometheus/client_golang/releases`
- `github.com/spf13/cobra` → `https://github.com/spf13/cobra/releases`
- `github.com/spf13/viper` → `https://github.com/spf13/viper/releases`
- `github.com/fsnotify/fsnotify` → `https://github.com/fsnotify/fsnotify/releases`
- `ci-kubed` → `https://github.com/powerhome/ci-kubed/releases` (may be private; if inaccessible, note that release notes could not be fetched and flag for manual review)

Scan for deprecation notices and behavior changes in the releases between old and new version.

**If deprecations are found**, grep the codebase:
```bash
grep -rn '<DeprecatedSymbol>' pkg/ cmd/ main.go --include='*.go'
```

Report: a summary of what changed in the version range and whether any deprecated symbols appear in the codebase.

For GitHub Actions bumps: fetch the action's release page and summarize what changed.

For non-digest Docker bumps: state the new tag and whether it represents a significant image version jump.

## Step 7: Tier 3 analysis

For each Tier 3 dependency, write one confirmation line:

- Indirect Go dep: `<module> <old> to <new> — indirect dependency, not imported directly by this codebase`
- Digest-only Docker: `<image>:<tag> — digest updated, tag unchanged, security patch only`
- Toolchain: `toolchain <old> to <new> — Go toolchain version bump, no library API surface change`
- Lock file only: `go.sum updated — lock file maintenance, no dependency version changed`
- Test framework dep: `<module> <old> to <new> — test-only dependency, no production API surface`

## Step 8: Build and test validation

Record the current branch, check out the PR branch, run validation, then return.

```bash
CURRENT_BRANCH=$(git branch --show-current)
if [ -z "$CURRENT_BRANCH" ]; then CURRENT_BRANCH=$(git rev-parse HEAD); fi
gh pr checkout <number>
go build ./...
go test ./... -timeout 120s
```

If any of `Dockerfile`, `Dockerfile.kubeconfig-reloader`, or `Dockerfile.localTest` changed in the PR diff, build the changed ones:

```bash
docker build -f Dockerfile .
docker build -f Dockerfile.kubeconfig-reloader .
docker build -f Dockerfile.localTest .
```

Only run the `docker build` command for Dockerfiles that actually appear in the PR diff.

If any file under `chart/` changed:

```bash
helm lint chart/
```

Return to the original branch regardless of whether the above commands succeeded or failed:

```bash
git checkout "$CURRENT_BRANCH"
```

Capture exit codes and output for every command. If all pass, a single PASS line per command is sufficient. If any command fails, include the full error output and end the report with BLOCKED.

## Step 9: Print the report in chat

No file is written. Print this structure directly in chat:

```
PR #<number> — <title>
Types: <comma-separated detected types>
Changes: <module or image> <old version> to <new version>[, ...]

CONSTRAINT CHECK
<compliance statement, or BLOCKED with reason>

DEPENDENCY ANALYSIS
Tier 1 findings:
  Imported symbols from changed packages:
    <module>: <list of sub-package import paths used in this repo>
  Release note findings:
    <module>: <findings, or "no breaking changes found in used API surface">

Tier 2 findings:
  <module>: <release note summary and grep results, or "no deprecations found in used API surface">

Tier 3:
  <one confirmation line per Tier 3 dependency>

BUILD AND TEST RESULT
  go build ./...                               PASS
  go test ./...                                PASS
  docker build Dockerfile                      PASS   (only if applicable)
  docker build Dockerfile.kubeconfig-reloader  PASS   (only if applicable)
  helm lint chart/                             PASS   (only if applicable)

VERDICT
SAFE TO MERGE / REVIEW NEEDED / BLOCKED
<one or two sentences explaining the verdict>
```

Sections present conditionally:
- CONSTRAINT CHECK: only when direct Go deps are in the PR
- DEPENDENCY ANALYSIS: always present (omit Tier 1 sub-section if no Tier 1 deps; omit Tier 2 sub-section if no Tier 2 deps)

Verdict rules:
- BLOCKED: any constraint check failure, or any build/test command returned a non-zero exit code
- REVIEW NEEDED: all commands passed but breaking change concerns were found, or release notes mention behavior changes that cannot be ruled out by a grep (e.g., changed timeout defaults, altered error message formats, modified retry logic, changed default configuration values)
- SAFE TO MERGE: constraint check passed, no breaking changes found in the used API surface, all commands green
