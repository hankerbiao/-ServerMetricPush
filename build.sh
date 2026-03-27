#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
RELEASES_DIR="${ROOT_DIR}/releases"
BUILD_DIR="${RELEASES_DIR}/build"
APP_NAME="node-push-exporter"
CONFIG_FILE="${ROOT_DIR}/config.yml"
UPLOAD_URL="${BINARY_DOWNLOAD_UPLOAD_URL:-http://10.17.154.252:8888/api/upload}"

TARGETS=(
  "linux amd64"
  "linux arm64"
  "linux arm 7"
)

upload_artifact() {
  local artifact_path="$1"
  local artifact_name

  artifact_name="$(basename "${artifact_path}")"
  echo "uploading ${artifact_name} -> ${UPLOAD_URL}"
  curl --fail --silent --show-error \
    -F "file=@${artifact_path};filename=${artifact_name}" \
    "${UPLOAD_URL}" >/dev/null
}

rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}" "${RELEASES_DIR}"

if [[ ! -f "${CONFIG_FILE}" ]]; then
  echo "missing config file: ${CONFIG_FILE}" >&2
  exit 1
fi

for target in "${TARGETS[@]}"; do
  read -r goos goarch goarm <<<"${target}"

  suffix="${goos}-${goarch}"
  if [[ "${goarch}" == "arm" ]]; then
    suffix="${suffix}v${goarm}"
  fi

  package_name="${APP_NAME}-${suffix}"
  staging_dir="${BUILD_DIR}/${package_name}"
  archive_path="${RELEASES_DIR}/${package_name}.tar.gz"

  rm -rf "${staging_dir}" "${archive_path}"
  mkdir -p "${staging_dir}"

  echo "building ${package_name}"
  if [[ "${goarch}" == "arm" ]]; then
    GOOS="${goos}" GOARCH="${goarch}" GOARM="${goarm}" \
      go build -o "${staging_dir}/${APP_NAME}" ./src
  else
    GOOS="${goos}" GOARCH="${goarch}" \
      go build -o "${staging_dir}/${APP_NAME}" ./src
  fi

  cp "${CONFIG_FILE}" "${staging_dir}/config.yml"
  cp "${ROOT_DIR}/README.md" "${staging_dir}/README.md"

  tar -C "${BUILD_DIR}" -czf "${archive_path}" "${package_name}"
  upload_artifact "${archive_path}"
done

echo "artifacts written to ${RELEASES_DIR}"
