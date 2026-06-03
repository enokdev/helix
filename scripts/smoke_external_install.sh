#!/usr/bin/env bash
set -euo pipefail

HELIX_ROOT="${HELIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
GO_BIN="${GO_BIN:-go}"
PORT="${HELIX_SMOKE_PORT:-18080}"
TMP_PARENT="${TMP_PARENT:-${TMPDIR:-/tmp}}"
TMP_DIR="$(mktemp -d "${TMP_PARENT%/}/helix-external-smoke.XXXXXX")"
KEEP_TMP="${KEEP_TMP:-0}"

SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
  if [[ "${KEEP_TMP}" != "1" ]]; then
    if [[ "${TMP_DIR}" == "${TMP_PARENT%/}/helix-external-smoke."* ]]; then
      rm -rf -- "${TMP_DIR}"
    fi
  else
    printf 'keeping temp dir: %s\n' "${TMP_DIR}"
  fi
}
trap cleanup EXIT

if ! [[ "${PORT}" =~ ^[0-9]+$ ]]; then
  printf 'invalid HELIX_SMOKE_PORT: %s\n' "${PORT}" >&2
  exit 2
fi
PORT_NUM=$((10#${PORT}))
if ((PORT_NUM < 1 || PORT_NUM > 65535)); then
  printf 'invalid HELIX_SMOKE_PORT: %s\n' "${PORT}" >&2
  exit 2
fi

if ! GO_BIN_PATH="$(command -v "${GO_BIN}")"; then
  printf 'go binary not found: %s\n' "${GO_BIN}" >&2
  exit 127
fi
GO_VERSION="$("${GO_BIN}" env GOVERSION)"

printf 'helix root: %s\n' "${HELIX_ROOT}"
printf 'temp module: %s\n' "${TMP_DIR}"
printf 'go binary: %s (%s)\n' "${GO_BIN_PATH}" "${GO_VERSION}"

mkdir -p "${TMP_DIR}/config"
cd "${TMP_DIR}"

"${GO_BIN}" mod init example.com/helix-external-smoke
"${GO_BIN}" mod edit -replace github.com/enokdev/helix="${HELIX_ROOT}"

cat > config/application.yaml <<YAML
server:
  port: ${PORT}
helix:
  starters:
    web:
      enabled: true
    observability:
      enabled: true
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

"${GO_BIN}" get github.com/enokdev/helix
"${GO_BIN}" get github.com/gofiber/fiber/v2
"${GO_BIN}" mod tidy

"${GO_BIN}" get github.com/enokdev/helix/cmd/helix
GOBIN="${TMP_DIR}/bin" "${GO_BIN}" install github.com/enokdev/helix/cmd/helix
"${TMP_DIR}/bin/helix" --version | grep -E '^helix '

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
