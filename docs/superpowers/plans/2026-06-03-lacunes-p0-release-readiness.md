# Lacunes P0 Release Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Audit the P0 section of `lacunes.md`, mark only proven completed items, and implement a reproducible external installation smoke check for the first still-open P0.

**Architecture:** Keep release-readiness checks outside framework runtime packages. Add one repository-level shell script under `scripts/` that creates a temporary external Go module, installs the local Helix module through a `replace`, verifies the CLI version command, builds a zero-config app, starts it, and checks `/actuator/health`. Update `lacunes.md` only after validation succeeds.

**Tech Stack:** Go 1.21+ module tooling, zsh-compatible POSIX shell script, `curl`, Helix CLI, existing Go packages.

---

## Execution Notes

During execution, the smoke script was hardened beyond the initial draft:

- It no longer accepts arbitrary `TMP_DIR` values for recursive cleanup; it creates its own `helix-external-smoke.*` directory under `TMP_PARENT`.
- It validates `HELIX_SMOKE_PORT` and reports missing `GO_BIN` explicitly.
- It writes `main.go` before `go mod tidy`, so Helix remains required in the external module.
- It runs `go get github.com/enokdev/helix/cmd/helix` before installing the CLI, so CLI-only dependencies are present in the external module sum.
- It explicitly enables the web and observability starters so `/actuator/health` is deterministic.

The final script at `scripts/smoke_external_install.sh` is the source of truth for the implemented procedure.

## File Structure

- Create: `scripts/smoke_external_install.sh`
  - Responsibility: Reproducible local smoke test from outside the checkout.
  - It must not write inside the Helix repo except reading it through `HELIX_ROOT`.
  - It must clean its temporary directory unless `KEEP_TMP=1` is set.
- Modify: `lacunes.md`
  - Responsibility: Mark completed P0 items with `[x]` only when there is proof.
  - For this cycle, mark `Verifier l'installation externe depuis un projet neuf` after the smoke script passes.
- Optional modify: `README.md`
  - Responsibility: Add a short maintainer command only if needed to make the smoke procedure discoverable from public docs.
  - Do not add internal planning terminology.

## Task 1: Confirm P0 Baseline

**Files:**
- Read: `lacunes.md`
- Read: `.github/workflows/ci.yml`
- Read: `.github/workflows/release.yml`
- Read: `README.md`
- Read: `docs/guide/installation.md`

- [x] **Step 1: Inspect current P0 checklist**

Run:

```bash
sed -n '1,95p' lacunes.md
```

Expected: the seven P0 items are unchecked before implementation begins.

- [x] **Step 2: Inspect current CI Go version**

Run:

```bash
sed -n '1,80p' .github/workflows/ci.yml
```

Expected: CI uses only `go-version: "1.21"`, so `Tester la compatibilite Go sur plusieurs versions supportees` remains open in this cycle.

- [x] **Step 3: Inspect release workflow**

Run:

```bash
sed -n '1,120p' .github/workflows/release.yml
```

Expected: release workflow exists, but no local dry-run checklist has been executed in this cycle, so `Valider le workflow de release de bout en bout` remains open.

- [x] **Step 4: Inspect public installation docs**

Run:

```bash
rg -n "go get github.com/enokdev/helix|go install github.com/enokdev/helix/cmd/helix|helix --version" README.md docs/guide/installation.md
```

Expected: docs mention installation commands, but no versioned smoke procedure proves them from an external module yet.

## Task 2: Add External Installation Smoke Script

**Files:**
- Create: `scripts/smoke_external_install.sh`

- [x] **Step 1: Create the smoke script**

Create `scripts/smoke_external_install.sh` with this complete content:

```bash
#!/usr/bin/env bash
set -euo pipefail

HELIX_ROOT="${HELIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
GO_BIN="${GO_BIN:-go}"
PORT="${HELIX_SMOKE_PORT:-18080}"
TMP_DIR="${TMP_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/helix-external-smoke.XXXXXX")}"
KEEP_TMP="${KEEP_TMP:-0}"

SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  if [[ "${KEEP_TMP}" != "1" ]]; then
    rm -rf "${TMP_DIR}"
  else
    printf 'keeping temp dir: %s\n' "${TMP_DIR}"
  fi
}
trap cleanup EXIT

printf 'helix root: %s\n' "${HELIX_ROOT}"
printf 'temp module: %s\n' "${TMP_DIR}"
printf 'go binary: %s\n' "$("${GO_BIN}" env GOVERSION)"

mkdir -p "${TMP_DIR}/config"
cd "${TMP_DIR}"

"${GO_BIN}" mod init example.com/helix-external-smoke
"${GO_BIN}" mod edit -replace github.com/enokdev/helix="${HELIX_ROOT}"
"${GO_BIN}" get github.com/enokdev/helix
"${GO_BIN}" get github.com/gofiber/fiber/v2
"${GO_BIN}" mod tidy

GOBIN="${TMP_DIR}/bin" "${GO_BIN}" install github.com/enokdev/helix/cmd/helix
"${TMP_DIR}/bin/helix" --version | grep -E '^helix '

cat > config/application.yaml <<YAML
server:
  port: ${PORT}
YAML

cat > main.go <<'GO'
package main

import (
	"log"

	"github.com/enokdev/helix"
)

func main() {
	if err := helix.Run(); err != nil {
		log.Fatal(err)
	}
}
GO

"${GO_BIN}" build ./...
"${GO_BIN}" run . > "${TMP_DIR}/server.log" 2>&1 &
SERVER_PID="$!"

for _ in {1..40}; do
  if curl -fsS "http://127.0.0.1:${PORT}/actuator/health" >/dev/null; then
    printf 'external smoke passed: /actuator/health responded on port %s\n' "${PORT}"
    exit 0
  fi
  if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
    printf 'server exited early\n' >&2
    cat "${TMP_DIR}/server.log" >&2
    exit 1
  fi
  sleep 0.25
done

printf 'timed out waiting for /actuator/health\n' >&2
cat "${TMP_DIR}/server.log" >&2
exit 1
```

- [x] **Step 2: Make the script executable**

Run:

```bash
chmod +x scripts/smoke_external_install.sh
```

Expected: command exits 0.

- [x] **Step 3: Verify shell syntax**

Run:

```bash
bash -n scripts/smoke_external_install.sh
```

Expected: command exits 0 with no output.

## Task 3: Run the External Smoke Validation

**Files:**
- Execute: `scripts/smoke_external_install.sh`

- [x] **Step 1: Run the smoke script with the project Go binary**

Run:

```bash
GO_BIN=/Users/yacoubakone/.govm/go/bin/go scripts/smoke_external_install.sh
```

Expected output includes:

```text
helix root: /Users/yacoubakone/Documents/dev/helix
helix
external smoke passed: /actuator/health responded on port 18080
```

If port `18080` is already in use, rerun:

```bash
HELIX_SMOKE_PORT=18081 GO_BIN=/Users/yacoubakone/.govm/go/bin/go scripts/smoke_external_install.sh
```

Expected: same pass message with port `18081`.

- [x] **Step 2: If network dependency resolution is blocked**

If `go get` fails with a DNS, proxy, or module download error, rerun the same command with escalated permissions per environment policy. If it is still blocked, keep the script and do not mark the P0 item complete.

## Task 4: Update `lacunes.md`

**Files:**
- Modify: `lacunes.md`

- [x] **Step 1: Mark only the completed external install P0**

Change this line:

```markdown
- [ ] Verifier l'installation externe depuis un projet neuf
```

to:

```markdown
- [x] Verifier l'installation externe depuis un projet neuf
```

Do not mark the other P0 items complete in this cycle unless a prior task produced direct proof for them.

- [x] **Step 2: Add validation evidence under that item**

Add this bullet under the existing `Validation:` bullet:

```markdown
  - Preuve: `scripts/smoke_external_install.sh` cree un module temporaire externe, ajoute Helix via `go get` avec `replace`, installe `cmd/helix`, verifie `helix --version`, build l'app zero-config et teste `/actuator/health`.
```

Expected: `lacunes.md` remains French and still contains no planning IDs in public package docs.

## Task 5: Final Verification

**Files:**
- Verify: `scripts/smoke_external_install.sh`
- Verify: `lacunes.md`

- [x] **Step 1: Run syntax validation again**

Run:

```bash
bash -n scripts/smoke_external_install.sh
```

Expected: exits 0.

- [x] **Step 2: Run the focused smoke test**

Run:

```bash
GO_BIN=/Users/yacoubakone/.govm/go/bin/go scripts/smoke_external_install.sh
```

Expected: exits 0 and prints the external smoke pass message.

- [x] **Step 3: Inspect changed files**

Run:

```bash
git diff -- scripts/smoke_external_install.sh lacunes.md
```

Expected: only the smoke script and the completed P0 checkbox/evidence changed.

- [x] **Step 4: Check worktree state**

Run:

```bash
git status --short
```

Expected: changed files include `lacunes.md`, `scripts/smoke_external_install.sh`, and this plan file if it has not been committed yet.

- [x] **Step 5: Commit implementation**

Run:

```bash
git add lacunes.md scripts/smoke_external_install.sh docs/superpowers/plans/2026-06-03-lacunes-p0-release-readiness.md
git commit -m "test: add external install smoke check"
```

Expected: commit succeeds.

## Self-Review

- Spec coverage: the plan audits all P0 items, implements the first open P0, updates `lacunes.md` only after proof, and validates locally.
- Placeholder scan: no `TBD`, `TODO`, or vague "handle edge cases" steps remain.
- Type and command consistency: all paths match the Helix repository layout, and Go commands use the govm binary required by project instructions.
