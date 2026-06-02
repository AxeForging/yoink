#!/bin/sh
# yoink installer — downloads the right binary for your OS/arch and installs it.
#
#   curl -fsSL https://raw.githubusercontent.com/AxeForging/yoink/main/install.sh | sh
#
# Env overrides:
#   VERSION=v0.1.0     install a specific release (default: latest)
#   YOINK_INSTALL_DIR  install location (default: /usr/local/bin)
set -eu

REPO="AxeForging/yoink"
INSTALL_DIR="${YOINK_INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

err() { echo "yoink-install: $*" >&2; exit 1; }

# Detect OS
os="$(uname -s)"
case "$os" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) err "unsupported OS: $os (Linux and macOS only)" ;;
esac

# Detect arch
arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  i386|i686) ARCH="386" ;;
  *) err "unsupported architecture: $arch" ;;
esac

ASSET="yoink-${OS}-${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

echo "Installing yoink (${VERSION}) for ${OS}-${ARCH}..."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMP/$ASSET" || err "download failed: $URL"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$TMP/$ASSET" "$URL" || err "download failed: $URL"
else
  err "need curl or wget to download"
fi

tar -xzf "$TMP/$ASSET" -C "$TMP" || err "failed to extract $ASSET"
chmod +x "$TMP/yoink"

# Install, using sudo only if the target dir isn't writable.
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP/yoink" "$INSTALL_DIR/yoink"
elif command -v sudo >/dev/null 2>&1; then
  echo "Elevating with sudo to write to $INSTALL_DIR"
  sudo mv "$TMP/yoink" "$INSTALL_DIR/yoink"
else
  err "$INSTALL_DIR is not writable and sudo is unavailable — set YOINK_INSTALL_DIR to a writable path"
fi

echo "yoink installed to $INSTALL_DIR/yoink"
"$INSTALL_DIR/yoink" version 2>/dev/null || true
