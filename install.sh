#!/usr/bin/env bash

set -euo pipefail

# 默认指向本机运行的 binary-download-service，可通过参数或环境变量覆盖。
DEFAULT_BASE_URL="http://127.0.0.1:8888"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

log() {
  printf '[install] %s\n' "$*"
}

die() {
  printf '[install] error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  local cmd="$1"
  command -v "${cmd}" >/dev/null 2>&1 || die "missing required command: ${cmd}"
}

normalize_os() {
  local raw="${1:-}"
  # 将 uname 输出统一映射为脚本内部使用的 os 字段。
  case "${raw}" in
    Linux|linux)
      printf 'linux\n'
      ;;
    Darwin|darwin)
      printf 'darwin\n'
      ;;
    *)
      die "unsupported operating system: ${raw}"
      ;;
  esac
}

normalize_arch() {
  local raw="${1:-}"
  # 将常见架构名称统一映射为脚本内部使用的 arch 字段。
  case "${raw}" in
    x86_64|amd64)
      printf 'amd64\n'
      ;;
    aarch64|arm64)
      printf 'arm64\n'
      ;;
    armv7l|armv7)
      printf 'armv7\n'
      ;;
    *)
      die "unsupported architecture: ${raw}"
      ;;
  esac
}

infer_binary_name() {
  # 程序名和归档包里实际二进制名并不总是完全等价，集中在这里维护映射。
  case "$1" in
    node_exporter)
      printf 'node_exporter\n'
      ;;
    node-push-exporter)
      printf 'node-push-exporter\n'
      ;;
    *)
      die "unknown program: $1"
      ;;
  esac
}

resolve_filename() {
  local program="$1"
  local desired_os="$2"
  local desired_arch="$3"

  # 固定机型使用固定下载地址，不再查询 /api/files。
  case "${program}:${desired_os}:${desired_arch}" in
    node_exporter:linux:arm64)
      printf 'node_exporter-1.10.2.linux-arm64.tar.gz\n'
      ;;
    node_exporter:linux:amd64)
      printf 'node_exporter-1.10.2.linux-amd64.tar.gz\n'
      ;;
    node_exporter:darwin:arm64)
      printf 'node_exporter-1.10.2.darwin-arm64.tar.gz\n'
      ;;
    node-push-exporter:linux:armv7)
      printf 'node-push-exporter-linux-armv7.tar.gz\n'
      ;;
    node-push-exporter:linux:arm64)
      printf 'node-push-exporter-linux-arm64.tar.gz\n'
      ;;
    node-push-exporter:linux:amd64)
      printf 'node-push-exporter-linux-amd64.tar.gz\n'
      ;;
    *)
      die "no fixed artifact for ${program} on ${desired_os}/${desired_arch}"
      ;;
  esac
}

download_artifact() {
  local url="$1"
  local output="$2"
  curl --fail --silent --show-error --location "$url" --output "$output"
}

extract_archive_member() {
  local archive_path="$1"
  local member_name="$2"
  local temp_dir="$3"
  local target_path="$4"
  local archive_member

  # 发布包通常会带一级目录，这里先列出归档内容，再精确找到目标二进制的实际路径。
  archive_member="$(tar -tzf "${archive_path}" | awk -v target="${member_name}" '$0 ~ ("(^|/)" target "$") { print; exit }')"
  [[ -n "${archive_member}" ]] || die "archive ${archive_path} does not contain ${member_name}"

  tar -xzf "${archive_path}" -C "${temp_dir}" "${archive_member}"
  cp "${temp_dir}/${archive_member}" "${target_path}"
  chmod 0755 "${target_path}"
}

install_artifact() {
  local binary_name="$1"
  local artifact_path="$2"
  local filename="$3"
  local install_dir="$4"
  local temp_dir="$5"
  local target_path="${install_dir}/${binary_name}"

  mkdir -p "${install_dir}"
  # 当前所有固定下载地址都是 tar.gz，保留裸二进制复制逻辑，方便后续扩展。
  if [[ "${filename}" == *.tar.gz ]]; then
    extract_archive_member "${artifact_path}" "${binary_name}" "${temp_dir}" "${target_path}"
  else
    cp "${artifact_path}" "${target_path}"
    chmod 0755 "${target_path}"
  fi

  log "installed ${binary_name} -> ${target_path}"
}

install_program() {
  local program="$1"
  local desired_os="$2"
  local desired_arch="$3"
  local base_url="$4"
  local install_dir="$5"
  local temp_root="$6"
  local binary_name
  local filename
  local artifact_path
  local download_url
  local program_temp

  binary_name="$(infer_binary_name "${program}")"
  filename="$(resolve_filename "${program}" "${desired_os}" "${desired_arch}")"
  artifact_path="${temp_root}/${filename}"
  download_url="${base_url%/}/download/${filename}"
  program_temp="${temp_root}/${program}"
  mkdir -p "${program_temp}"

  # 平台一旦识别完成，下载地址就是固定的。
  log "downloading ${download_url}"
  download_artifact "${download_url}" "${artifact_path}"
  install_artifact "${binary_name}" "${artifact_path}" "${filename}" "${install_dir}" "${program_temp}"
}

usage() {
  cat <<EOF
Usage: $0 [-b base_url] [-d install_dir]

Options:
  -b, --base-url      Binary download service base URL
  -d, --install-dir   Directory to place binaries into
  -h, --help          Show this help text

Environment variables:
  BINARY_DOWNLOAD_BASE_URL  Same as --base-url
  INSTALL_DIR               Same as --install-dir
EOF
}

main() {
  require_command curl
  require_command tar
  require_command uname

  # 默认把二进制安装到脚本所在目录，便于本地直接执行；
  # 正式部署时通常会显式传 `-d /usr/local/bin`。
  local base_url="${BINARY_DOWNLOAD_BASE_URL:-${DEFAULT_BASE_URL}}"
  local install_dir="${INSTALL_DIR:-${SCRIPT_DIR}}"
  local current_os
  local current_arch
  local temp_root

  while [[ $# -gt 0 ]]; do
    case "$1" in
      -b|--base-url)
        [[ $# -ge 2 ]] || die "missing value for $1"
        base_url="$2"
        shift 2
        ;;
      -d|--install-dir)
        [[ $# -ge 2 ]] || die "missing value for $1"
        install_dir="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done

  current_os="$(normalize_os "$(uname -s)")"
  current_arch="$(normalize_arch "$(uname -m)")"

  temp_root="$(mktemp -d)"
  trap 'rm -rf "${temp_root}"' EXIT

  log "detected platform: ${current_os}/${current_arch}"
  log "install dir: ${install_dir}"
  install_program "node_exporter" "${current_os}" "${current_arch}" "${base_url}" "${install_dir}" "${temp_root}"
  install_program "node-push-exporter" "${current_os}" "${current_arch}" "${base_url}" "${install_dir}" "${temp_root}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
