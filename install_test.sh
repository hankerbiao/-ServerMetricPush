#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SH="${ROOT_DIR}/install.sh"

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
source "${INSTALL_SH}"

assert_eq \
  "$(bash -lc 'source "'"${INSTALL_SH}"'"; uname() { if [[ "$1" == "-s" ]]; then printf "Linux\n"; else printf "x86_64\n"; fi; }; resolve_files')" \
  $'node_exporter-1.10.2.linux-amd64.tar.gz\nnode-push-exporter-linux-amd64.tar.gz' \
  "linux amd64 mapping"

assert_eq \
  "$(bash -lc 'source "'"${INSTALL_SH}"'"; uname() { if [[ "$1" == "-s" ]]; then printf "Linux\n"; else printf "aarch64\n"; fi; }; resolve_files')" \
  $'node_exporter-1.10.2.linux-arm64.tar.gz\nnode-push-exporter-linux-arm64.tar.gz' \
  "linux arm64 mapping"

assert_eq \
  "$(bash -lc 'source "'"${INSTALL_SH}"'"; uname() { if [[ "$1" == "-s" ]]; then printf "Darwin\n"; else printf "arm64\n"; fi; }; resolve_files')" \
  "node_exporter-1.10.2.darwin-arm64.tar.gz" \
  "darwin arm64 mapping"

service_file="$(mktemp)"
write_node_exporter_service "${service_file}"
assert_eq \
  "$(cat "${service_file}")" \
  $'[Unit]\nDescription=Node Exporter\nWants=network-online.target\nAfter=network-online.target\n\n[Service]\nUser=root\nExecStart=/usr/local/bin/node_exporter\nRestart=always\n\n[Install]\nWantedBy=multi-user.target' \
  "service file content"
rm -f "${service_file}"

push_service_file="$(mktemp)"
write_node_push_exporter_service "${push_service_file}"
assert_eq \
  "$(cat "${push_service_file}")" \
  $'[Unit]\nDescription=Node Metrics Push Exporter\nAfter=network-online.target\nWants=network-online.target\n\n[Service]\nType=simple\nUser=root\nGroup=root\nExecStart=/usr/local/bin/node-push-exporter --config /etc/node-push-exporter/config.yaml\nRestart=always\nRestartSec=10\nStandardOutput=journal\nStandardError=journal\nEnvironment=PATH=/usr/local/bin:/usr/bin:/bin\nSyslogIdentifier=node-push-exporter\n\n[Install]\nWantedBy=multi-user.target' \
  "push service file content"
rm -f "${push_service_file}"

assert_fail \
  "darwin amd64 should fail" \
  bash -lc 'uname() { if [[ "$1" == "-s" ]]; then printf "Darwin\n"; else printf "x86_64\n"; fi; }; source "'"${INSTALL_SH}"'"; resolve_files'

assert_eq \
  "$(bash -uc 'source "'"${INSTALL_SH}"'"; printf "%s" "${BASE_URL}"')" \
  "http://10.17.154.252:8888" \
  "base url default under nounset"

assert_eq \
  "$(printf '%s\n' 'main() { printf "stdin-main-ran"; }' 'if [[ ${#BASH_SOURCE[@]} -eq 0 ]]; then' '  main "$@"' 'elif [[ "${BASH_SOURCE[0]}" == "$0" ]]; then' '  main "$@"' 'fi' | bash -u)" \
  "stdin-main-ran" \
  "stdin execution should not fail when BASH_SOURCE is unset"

echo "install tests passed"
