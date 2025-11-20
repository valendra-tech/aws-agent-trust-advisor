#!/usr/bin/env bash
set -euo pipefail

# Simple installer for aws-agent-trust-advisor.
# Usage: sudo ./install.sh [VERSION]
# If VERSION is omitted, the latest GitHub release is installed.

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Please run as root (sudo) to install into /usr/local/bin"
  exit 1
fi

VERSION="${1:-}"
REPO="valendra-tech/aws-agent-trust-advisor"

if [[ -z "$VERSION" ]]; then
  RELEASE=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d '"' -f4)
  VERSION="${RELEASE#v}"
else
  RELEASE="v${VERSION}"
fi

# Detect OS
case "$OSTYPE" in
  darwin*) OS="macos" ;;  # release asset uses macos_*
  linux*)  OS="linux" ;;
  *) echo "Unsupported OS: $OSTYPE"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  arm64)  ARCH="arm64" ;;
  aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY_NAME="aws-agent-trust-advisor_${OS}_${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${RELEASE}/${BINARY_NAME}"

TMP_FILE=$(mktemp)
echo "Downloading ${BINARY_NAME} (${RELEASE}) ..."
curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_FILE}"

install -m 0755 "${TMP_FILE}" /usr/local/bin/aws-agent-trust-advisor
rm -f "${TMP_FILE}"

echo "Installed to /usr/local/bin/aws-agent-trust-advisor"
echo "Version:"
/usr/local/bin/aws-agent-trust-advisor --help || true
