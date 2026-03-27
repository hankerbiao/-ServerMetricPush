#!/usr/bin/env bash

set -euo pipefail

NODE_EXPORTER_BIN="/usr/local/bin/node_exporter"
NODE_EXPORTER_SERVICE_NAME="node_exporter"
NODE_EXPORTER_SERVICE_PATH="/etc/systemd/system/node_exporter.service"
NODE_PUSH_EXPORTER_BIN="/usr/local/bin/node-push-exporter"
NODE_PUSH_EXPORTER_SERVICE_NAME="node-push-exporter"
NODE_PUSH_EXPORTER_CONFIG_DIR="/etc/node-push-exporter"
NODE_PUSH_EXPORTER_SERVICE_PATH="/etc/systemd/system/node-push-exporter.service"
ERROR_TIP="联系管理员，光圈@libiao1"

log() {
  printf '[uninstall] %s\n' "$*"
}

fail() {
  local message="$1"
  printf '[uninstall] 异常: %s\n' "${message}" >&2
  printf '[uninstall] %s\n' "${ERROR_TIP}" >&2
  exit 1
}

run_step() {
  local description="$1"
  shift

  log "执行: ${description}"
  if "$@"; then
    log "结果: ${description} 成功"
  else
    fail "${description} 失败"
  fi
}

require_root() {
  [[ "${EUID}" -eq 0 ]] || {
    fail "卸载需要 root 权限，请使用 sudo 执行"
  }
}

ensure_linux() {
  [[ "$(uname -s)" == "Linux" ]] || {
    fail "uninstall.sh 仅支持 Linux/systemd 环境"
  }
}

ensure_systemctl() {
  command -v systemctl >/dev/null 2>&1 || {
    fail "当前系统缺少 systemctl，无法完成卸载"
  }
}

service_is_known() {
  local service_name="$1"
  local service_path="$2"
  local unit_file_name="${service_name}.service"

  if [[ -f "${service_path}" ]]; then
    return 0
  fi

  systemctl list-unit-files "${unit_file_name}" --no-legend 2>/dev/null | grep -Fq "${unit_file_name}"
}

stop_and_disable_service() {
  local service_name="$1"
  local service_path="$2"

  if service_is_known "${service_name}" "${service_path}"; then
    run_step "停止并禁用 ${service_name}" systemctl disable --now "${service_name}"
  else
    log "结果: 跳过 ${service_name} 停止和禁用，服务不存在"
  fi
}

remove_file_if_exists() {
  local target_path="$1"

  if [[ -e "${target_path}" ]]; then
    run_step "删除 ${target_path}" rm -f "${target_path}"
  else
    log "结果: 跳过删除 ${target_path}，文件不存在"
  fi
}

remove_dir_if_exists() {
  local target_path="$1"

  if [[ -e "${target_path}" ]]; then
    run_step "删除目录 ${target_path}" rm -rf "${target_path}"
  else
    log "结果: 跳过删除目录 ${target_path}，目录不存在"
  fi
}

main() {
  ensure_linux
  ensure_systemctl
  require_root

  stop_and_disable_service "${NODE_PUSH_EXPORTER_SERVICE_NAME}" "${NODE_PUSH_EXPORTER_SERVICE_PATH}"
  stop_and_disable_service "${NODE_EXPORTER_SERVICE_NAME}" "${NODE_EXPORTER_SERVICE_PATH}"

  remove_file_if_exists "${NODE_PUSH_EXPORTER_SERVICE_PATH}"
  remove_file_if_exists "${NODE_EXPORTER_SERVICE_PATH}"
  remove_file_if_exists "${NODE_PUSH_EXPORTER_BIN}"
  remove_file_if_exists "${NODE_EXPORTER_BIN}"
  remove_dir_if_exists "${NODE_PUSH_EXPORTER_CONFIG_DIR}"

  run_step "刷新 systemd 配置" systemctl daemon-reload
  log "结果: uninstall.sh 执行完成"
}

if [[ ${#BASH_SOURCE[@]} -eq 0 ]]; then
  main "$@"
elif [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
