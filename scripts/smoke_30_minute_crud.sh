#!/usr/bin/env bash
set -euo pipefail

HELIX_ROOT="${HELIX_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
GO_BIN="${GO_BIN:-go}"
PORT="${HELIX_SMOKE_PORT:-8080}"
TMP_PARENT="${TMP_PARENT:-${TMPDIR:-/tmp}}"
KEEP_TMP="${KEEP_TMP:-0}"

if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
  printf 'go binary not found: %s\n' "${GO_BIN}" >&2
  exit 1
fi

case "${PORT}" in
  ''|*[!0-9]*)
    printf 'HELIX_SMOKE_PORT must be numeric, got: %s\n' "${PORT}" >&2
    exit 1
    ;;
esac
if [[ "${PORT}" != "8080" ]]; then
  printf 'HELIX_SMOKE_PORT=%s is not supported by the current public CRUD example; use 8080\n' "${PORT}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d "${TMP_PARENT%/}/helix-30-minute-crud.XXXXXX")"
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
printf 'go version: %s\n' "$("${GO_BIN}" env GOVERSION)"

cp -R "${HELIX_ROOT}/examples/crud-api/." "${TMP_DIR}/"
cd "${TMP_DIR}"

"${GO_BIN}" mod init example.com/helix-crud-smoke
"${GO_BIN}" mod edit -replace github.com/enokdev/helix="${HELIX_ROOT}"
"${GO_BIN}" get github.com/enokdev/helix
"${GO_BIN}" mod tidy
"${GO_BIN}" test ./...
"${GO_BIN}" build ./...

if curl -fsS "http://127.0.0.1:${PORT}/users" >/dev/null 2>&1; then
  printf 'port %s already serves /users; stop the existing server or set HELIX_SMOKE_PORT after updating the example to load custom ports before starter configuration\n' "${PORT}" >&2
  exit 1
fi

"${GO_BIN}" run . > "${TMP_DIR}/server.log" 2>&1 &
SERVER_PID="$!"

READY=0
for _ in {1..80}; do
  if curl -fsS "http://127.0.0.1:${PORT}/users" >/dev/null; then
    READY=1
    break
  fi
  if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
    printf 'server exited early\n' >&2
    cat "${TMP_DIR}/server.log" >&2
    exit 1
  fi
  sleep 0.25
done

if [[ "${READY}" != "1" ]]; then
  printf 'timed out waiting for /users on port %s\n' "${PORT}" >&2
  cat "${TMP_DIR}/server.log" >&2
  exit 1
fi

created="$(curl -fsS -X POST "http://127.0.0.1:${PORT}/users" \
  -H 'content-type: application/json' \
  -d '{"name":"Ada Lovelace","email":"ada@example.com"}')"
printf '%s\n' "${created}" | grep -q '"id":1'
printf '%s\n' "${created}" | grep -q '"email":"ada@example.com"'

curl -fsS "http://127.0.0.1:${PORT}/users/1" | grep -q '"name":"Ada Lovelace"'
curl -fsS "http://127.0.0.1:${PORT}/users" | grep -q '"email":"ada@example.com"'

printf '30-minute CRUD smoke passed on port %s\n' "${PORT}"
