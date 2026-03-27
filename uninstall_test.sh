#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
UNINSTALL_SH="${ROOT_DIR}/uninstall.sh"

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"

  if [[ "${actual}" != "${expected}" ]]; then
    echo "assertion failed: ${message}" >&2
    echo "expected: ${expected}" >&2
    echo "actual:   ${actual}" >&2
    exit 1
  fi
}

assert_fail() {
  local message="$1"
  shift

  if ( "$@" >/dev/null 2>&1 ); then
    echo "assertion failed: ${message}" >&2
    exit 1
  fi
}

# shellcheck disable=SC1090
source "${UNINSTALL_SH}"

assert_eq \
  "$(bash -lc 'source "'"${UNINSTALL_SH}"'"; printf "%s" "${NODE_EXPORTER_SERVICE_NAME},${NODE_PUSH_EXPORTER_SERVICE_NAME}"')" \
  "node_exporter,node-push-exporter" \
  "service names"

assert_eq \
  "$(bash -lc 'source "'"${UNINSTALL_SH}"'"; printf "%s" "${NODE_EXPORTER_BIN}|${NODE_PUSH_EXPORTER_BIN}|${NODE_PUSH_EXPORTER_CONFIG_DIR}"')" \
  "/usr/local/bin/node_exporter|/usr/local/bin/node-push-exporter|/etc/node-push-exporter" \
  "uninstall targets"

assert_fail \
  "non-linux should fail" \
  bash -lc 'source "'"${UNINSTALL_SH}"'"; uname() { printf "Darwin\n"; }; ensure_linux'

assert_eq \
  "$(bash -lc 'source "'"${UNINSTALL_SH}"'"; systemctl() { if [[ "$1" == "list-unit-files" ]]; then return 1; fi; printf "unexpected"; }; stop_and_disable_service "node-push-exporter" "/tmp/missing.service"')" \
  $'[uninstall] 结果: 跳过 node-push-exporter 停止和禁用，服务不存在' \
  "missing service should be skipped"

assert_eq \
  "$(bash -lc 'source "'"${UNINSTALL_SH}"'"; systemctl() { if [[ "$1" == "list-unit-files" ]]; then printf ""; return 0; fi; printf "unexpected"; }; stop_and_disable_service "node-push-exporter" "/tmp/missing.service"')" \
  $'[uninstall] 结果: 跳过 node-push-exporter 停止和禁用，服务不存在' \
  "empty unit-file lookup should be skipped"

assert_eq \
  "$(printf '%s\n' 'main() { printf "stdin-main-ran"; }' 'if [[ ${#BASH_SOURCE[@]} -eq 0 ]]; then' '  main "$@"' 'elif [[ "${BASH_SOURCE[0]}" == "$0" ]]; then' '  main "$@"' 'fi' | bash -u)" \
  "stdin-main-ran" \
  "stdin execution should not fail when BASH_SOURCE is unset"

echo "uninstall tests passed"
