#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BINARY_DOWNLOAD_BASE_URL:-http://10.2.48.120:8888}"
DOWNLOAD_DIR="${DOWNLOAD_DIR:-$(pwd)}"
NODE_EXPORTER_BIN="/usr/local/bin/node_exporter"
NODE_EXPORTER_SERVICE_PATH="/etc/systemd/system/node_exporter.service"
NODE_PUSH_EXPORTER_BIN="/usr/local/bin/node-push-exporter"
NODE_PUSH_EXPORTER_CONFIG_DIR="/etc/node-push-exporter"
NODE_PUSH_EXPORTER_CONFIG_PATH="${NODE_PUSH_EXPORTER_CONFIG_DIR}/config.yaml"
NODE_PUSH_EXPORTER_SERVICE_PATH="/etc/systemd/system/node-push-exporter.service"
ERROR_TIP="联系管理员，光圈@libiao1"

log() {
  printf '[install] %s\n' "$*"
}

fail() {
  local message="$1"
  printf '[install] 异常: %s\n' "${message}" >&2
  printf '[install] %s\n' "${ERROR_TIP}" >&2
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

resolve_files() {
  case "$(uname -s):$(uname -m)" in
    Linux:x86_64|Linux:amd64)
      printf '%s\n' \
        "node_exporter-1.10.2.linux-amd64.tar.gz" \
        "node-push-exporter-linux-amd64.tar.gz"
      ;;
    Linux:aarch64|Linux:arm64)
      printf '%s\n' \
        "node_exporter-1.10.2.linux-arm64.tar.gz" \
        "node-push-exporter-linux-arm64.tar.gz"
      ;;
    Linux:armv7l|Linux:armv7)
      printf '%s\n' \
        "node-push-exporter-linux-armv7.tar.gz"
      ;;
    Darwin:arm64)
      printf '%s\n' \
        "node_exporter-1.10.2.darwin-arm64.tar.gz"
      ;;
    *)
      fail "不支持的平台: $(uname -s)/$(uname -m)"
      ;;
  esac
}

is_linux() {
  [[ "$(uname -s)" == "Linux" ]]
}

node_exporter_installed() {
  command -v node_exporter >/dev/null 2>&1 || [[ -x "${NODE_EXPORTER_BIN}" ]]
}

node_push_exporter_installed() {
  command -v node-push-exporter >/dev/null 2>&1 || [[ -x "${NODE_PUSH_EXPORTER_BIN}" ]]
}

download() {
  local filename="$1"
  run_step \
    "下载 ${filename}" \
    curl -fL "${BASE_URL}/download/${filename}" -o "${DOWNLOAD_DIR}/${filename}"
}

write_node_exporter_service() {
  local target_path="$1"
  cat > "${target_path}" <<'EOF'
[Unit]
Description=Node Exporter
Wants=network-online.target
After=network-online.target

[Service]
User=root
ExecStart=/usr/local/bin/node_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF
}

write_node_push_exporter_service() {
  local target_path="$1"
  cat > "${target_path}" <<'EOF'
[Unit]
Description=Node Metrics Push Exporter
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/node-push-exporter --config /etc/node-push-exporter/config.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
Environment=PATH=/usr/local/bin:/usr/bin:/bin
SyslogIdentifier=node-push-exporter

[Install]
WantedBy=multi-user.target
EOF
}

require_root() {
  [[ "${EUID}" -eq 0 ]] || {
    fail "安装需要 root 权限，请使用 sudo 执行"
  }
}

install_node_exporter() {
  local archive_path="$1"
  local extract_dir="${DOWNLOAD_DIR}/node_exporter_extract"
  local extracted_binary

  if ! is_linux; then
    log "结果: 跳过 node_exporter 安装，只有 Linux 支持 systemd 自动安装"
    return
  fi

  if node_exporter_installed; then
    log "结果: 跳过 node_exporter 安装，系统已存在 node_exporter"
    return
  fi

  require_root

  run_step "清理 node_exporter 临时目录" rm -rf "${extract_dir}"
  run_step "创建 node_exporter 临时目录" mkdir -p "${extract_dir}"
  run_step "解压 $(basename "${archive_path}")" tar -xvf "${archive_path}" -C "${extract_dir}"
  extracted_binary="$(find "${extract_dir}" -type f -name node_exporter | head -n 1)"
  [[ -n "${extracted_binary}" ]] || {
    fail "${archive_path} 中未找到 node_exporter"
  }

  run_step "安装 node_exporter 到 ${NODE_EXPORTER_BIN}" mv "${extracted_binary}" "${NODE_EXPORTER_BIN}"
  run_step "设置 node_exporter 可执行权限" chmod +x "${NODE_EXPORTER_BIN}"
  write_node_exporter_service "${NODE_EXPORTER_SERVICE_PATH}"
  log "结果: 已写入服务文件 ${NODE_EXPORTER_SERVICE_PATH}"
  run_step "刷新 systemd 配置" systemctl daemon-reload
  run_step "启用并启动 node_exporter" systemctl enable --now node_exporter
}

install_node_push_exporter() {
  local archive_path="$1"
  local extract_dir="${DOWNLOAD_DIR}/node_push_exporter_extract"
  local extracted_binary
  local extracted_config

  if ! is_linux; then
    log "结果: 跳过 node-push-exporter 安装，只有 Linux 支持 systemd 自动安装"
    return
  fi

  if node_push_exporter_installed; then
    log "结果: 跳过 node-push-exporter 安装，系统已存在 node-push-exporter"
    return
  fi

  require_root

  run_step "清理 node-push-exporter 临时目录" rm -rf "${extract_dir}"
  run_step "创建 node-push-exporter 临时目录" mkdir -p "${extract_dir}"
  run_step "解压 $(basename "${archive_path}")" tar -xvf "${archive_path}" -C "${extract_dir}"
  extracted_binary="$(find "${extract_dir}" -type f -name node-push-exporter | head -n 1)"
  [[ -n "${extracted_binary}" ]] || {
    fail "${archive_path} 中未找到 node-push-exporter"
  }

  extracted_config="$(find "${extract_dir}" -type f -name config.yml | head -n 1)"

  run_step "安装 node-push-exporter 到 ${NODE_PUSH_EXPORTER_BIN}" mv "${extracted_binary}" "${NODE_PUSH_EXPORTER_BIN}"
  run_step "设置 node-push-exporter 可执行权限" chmod +x "${NODE_PUSH_EXPORTER_BIN}"

  run_step "创建配置目录 ${NODE_PUSH_EXPORTER_CONFIG_DIR}" mkdir -p "${NODE_PUSH_EXPORTER_CONFIG_DIR}"
  if [[ ! -f "${NODE_PUSH_EXPORTER_CONFIG_PATH}" ]]; then
    [[ -n "${extracted_config}" ]] || {
      fail "${archive_path} 中未找到 config.yml"
    }
    run_step "复制默认配置到 ${NODE_PUSH_EXPORTER_CONFIG_PATH}" cp "${extracted_config}" "${NODE_PUSH_EXPORTER_CONFIG_PATH}"
  else
    log "结果: 保留现有配置 ${NODE_PUSH_EXPORTER_CONFIG_PATH}"
  fi

  write_node_push_exporter_service "${NODE_PUSH_EXPORTER_SERVICE_PATH}"
  log "结果: 已写入服务文件 ${NODE_PUSH_EXPORTER_SERVICE_PATH}"
  run_step "刷新 systemd 配置" systemctl daemon-reload
  run_step "启用并启动 node-push-exporter" systemctl enable --now node-push-exporter
}

main() {
  run_step "创建下载目录 ${DOWNLOAD_DIR}" mkdir -p "${DOWNLOAD_DIR}"
  while IFS= read -r filename; do
    download "${filename}"
    if [[ "${filename}" == node_exporter-*.tar.gz ]]; then
      install_node_exporter "${DOWNLOAD_DIR}/${filename}"
    elif [[ "${filename}" == node-push-exporter-*.tar.gz ]]; then
      install_node_push_exporter "${DOWNLOAD_DIR}/${filename}"
    fi
  done < <(resolve_files)
  log "结果: install.sh 执行完成"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
