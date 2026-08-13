#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION="${VERSION:-}"
OUTPUT_DIR="${OUTPUT_DIR:-${BACKEND_ROOT}/dist/linux-arm64}"
STAGING_DIR="${BACKEND_ROOT}/build/linux-arm64-staging"
TARGET="runtime-linux-arm64"
GO_REQUIRED_VERSION="1.26.1"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e"${GREEN}[INFO]${NC} $*"; }
log_warn() { echo -e"${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e"${RED}[ERROR]${NC} $*" >&2; }

fail() {
    log_error "$*"
    exit 1
}

check_tool() {
    if ! command -v "$1" >/dev/null 2>&1; then
        fail "required tool not found: $1"
    fi
}

validate_inputs() {
    if [[ -z "${VERSION}" ]]; then
        fail "VERSION is required (set VERSION environment variable or pass --version)"
    fi
    if [[ "${VERSION}" == "dev" ]]; then
        fail "VERSION must not be 'dev' for frozen artifact"
    fi
}

get_commit() {
    if ! commit=$(git -C "${BACKEND_ROOT}" rev-parse HEAD 2>/dev/null); then
        fail "failed to get git commit"
    fi
    if [[ -z "${commit}" ]]; then
        fail "git commit is empty"
    fi
    echo "${commit}"
}

check_git_state() {
    local dirty_files
    dirty_files=$(git -C "${BACKEND_ROOT}" status --porcelain 2>/dev/null | wc -l)
    if [[ "${dirty_files}" -gt 0 ]]; then
        log_warn "working tree has ${dirty_files} dirty file(s)"
        log_warn "frozen artifact should be built from clean source"
        git -C "${BACKEND_ROOT}" status --short
        if [[ "${ALLOW_DIRTY:-0}" != "1" ]]; then
            fail "dirty working tree (set ALLOW_DIRTY=1 to override)"
        fi
    fi
}

check_go_version() {
    local actual_version
    actual_version=$(go version | awk '{print $3}' | sed 's/go//')
    local required_major minor actual_major actual_minor
    required_major=$(echo "${GO_REQUIRED_VERSION}" | cut -d. -f1)
    required_minor=$(echo "${GO_REQUIRED_VERSION}" | cut -d. -f2)
    actual_major=$(echo "${actual_version}" | cut -d. -f1)
    actual_minor=$(echo "${actual_version}" | cut -d. -f2)
    if [[ "${actual_major}" -lt "${required_major}" ]] || \
       [[ "${actual_major}" -eq "${required_major}" && "${actual_minor}" -lt "${required_minor}" ]]; then
        fail "Go version ${actual_version} is below required ${GO_REQUIRED_VERSION}"
    fi
    echo "${actual_version}"
}

clean_staging() {
    if [[ -d "${STAGING_DIR}" ]]; then
        rm -rf "${STAGING_DIR}"
    fi
    mkdir -p "${STAGING_DIR}"
}

build_binary() {
    local go_version="$1"
    local commit="$2"

    log_info "building amitia-server for linux/arm64"
    log_info "  version: ${VERSION}"
    log_info "  commit: ${commit}"
    log_info "  target: ${TARGET}"
    log_info "  go version: ${go_version}"

    cd "${BACKEND_ROOT}"

    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=arm64 \
    go build \
        -mod=readonly \
        -trimpath \
        -buildvcs=false \
        -ldflags "\
-s -w \
-buildid= \
-X github.com/u-ai/backend/internal/buildinfo.Version=${VERSION} \
-X github.com/u-ai/backend/internal/buildinfo.Commit=${commit} \
-X github.com/u-ai/backend/internal/buildinfo.Target=${TARGET}" \
        -o "${STAGING_DIR}/amitia-server" \
        ./cmd/server

    chmod 0755 "${STAGING_DIR}/amitia-server"
}

verify_elf() {
    log_info "verifying ELF binary"

    local file_output
    file_output=$(file "${STAGING_DIR}/amitia-server")

    if ! echo "${file_output}" | grep -q "ELF 64-bit"; then
        fail "ELF verification failed: not ELF 64-bit. Output: ${file_output}"
    fi
    if ! echo "${file_output}" | grep -q "ARM aarch64\|AArch64"; then
        fail "ELF verification failed: not ARM aarch64. Output: ${file_output}"
    fi
    if ! echo "${file_output}" | grep -q "statically linked"; then
        fail "ELF verification failed: not statically linked. Output: ${file_output}"
    fi

    if command -v readelf >/dev/null 2>&1; then
        local readelf_header
        readelf_header=$(readelf -h "${STAGING_DIR}/amitia-server" 2>&1)
        if ! echo "${readelf_header}" | grep -q "Machine:.*AArch64\|Machine:.*ARM"; then
            fail "readelf verification failed: not AArch64"
        fi
    fi
}

compute_sha() {
    local sha
    if command -v sha256sum >/dev/null 2>&1; then
        sha=$(sha256sum "${STAGING_DIR}/amitia-server" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        sha=$(shasum -a 256 "${STAGING_DIR}/amitia-server" | awk '{print $1}')
    else
        fail "no sha256 tool found (sha256sum or shasum required)"
    fi
    echo "${sha}"
}

generate_sha256sums() {
    local sha="$1"
    echo "${sha}  amitia-server" > "${STAGING_DIR}/SHA256SUMS"
}

generate_artifact_json() {
    local sha="$1"
    local go_version="$2"
    local commit="$3"
    local size
    size=$(stat -c%s "${STAGING_DIR}/amitia-server" 2>/dev/null || stat -f%z "${STAGING_DIR}/amitia-server" 2>/dev/null)

    cat > "${STAGING_DIR}/backend-artifact.json" <<EOF
{
  "schemaVersion": 1,
  "component": "backend",
  "binary": "amitia-server",
  "version": "${VERSION}",
  "commit": "${commit}",
  "target": "${TARGET}",
  "goVersion": "${go_version}",
  "goos": "linux",
  "goarch": "arm64",
  "cgoEnabled": false,
  "linkMode": "static",
  "sha256": "${sha}",
  "size": ${size},
  "mode": "0755"
}
EOF
}

atomic_publish() {
    log_info "publishing artifact to ${OUTPUT_DIR}"
    if [[ -d "${OUTPUT_DIR}" ]]; then
        rm -rf "${OUTPUT_DIR}"
    fi
    mkdir -p "$(dirname "${OUTPUT_DIR}")"
    mv "${STAGING_DIR}" "${OUTPUT_DIR}"
}

print_report() {
    local sha="$1"
    local go_version="$2"
    local commit="$3"

    echo ""
    echo "============================================"
    echo "Amitia Step 1 - Linux ARM64 Go Backend Build"
    echo "============================================"
    echo ""
    echo "Canonical Build Entry = ${SCRIPT_DIR}/$(basename "${BASH_SOURCE[0]}")"
    echo "Go Required Version = ${GO_REQUIRED_VERSION}"
    echo "Go Actual Version = ${go_version}"
    echo ""
    echo "Version = ${VERSION}"
    echo "Commit = ${commit}"
    echo "Target = ${TARGET}"
    echo ""
    echo "GOOS = linux"
    echo "GOARCH = arm64"
    echo "CGO_ENABLED = 0"
    echo ""
    echo "Artifact = ${OUTPUT_DIR}/amitia-server"
    echo "Artifact SHA256 = ${sha}"
    echo ""
    echo "Artifact Metadata = ${OUTPUT_DIR}/backend-artifact.json"
    echo "SHA256SUMS = ${OUTPUT_DIR}/SHA256SUMS"
    echo ""
    echo "Build Status = PASS"
    echo "============================================"
}

main() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --version)
                VERSION="$2"
                shift 2
                ;;
            --output-dir)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            --allow-dirty)
                ALLOW_DIRTY=1
                shift
                ;;
            -h|--help)
                echo "Usage: $0 --version <version> [--output-dir <dir>] [--allow-dirty]"
                exit 0
                ;;
            *)
                fail "unknown argument: $1"
                ;;
        esac
    done

    check_tool go
    check_tool git
    check_tool file

    validate_inputs

    local commit
    commit=$(get_commit)
    check_git_state

    local go_version
    go_version=$(check_go_version)

    clean_staging
    build_binary "${go_version}" "${commit}"
    verify_elf

    local sha
    sha=$(compute_sha)
    generate_sha256sums "${sha}"
    generate_artifact_json "${sha}" "${go_version}" "${commit}"
    atomic_publish

    print_report "${sha}" "${go_version}" "${commit}"
}

main "$@"
