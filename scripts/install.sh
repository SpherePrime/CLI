#!/bin/sh
set -eu

REPO="${PRIME_REPO:-SpherePrime/CLI}"
BIN_DIR="${PRIME_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s)
arch=$(uname -m)

case "$os" in
  Linux) os="Linux" ;;
  Darwin) os="Darwin" ;;
  MINGW*|MSYS*|CYGWIN*) os="Windows" ;;
  *)
    echo "prime: unsupported OS: $os" >&2
    exit 1
    ;;
esac

case "$arch" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  i386) arch="i386" ;;
  *)
    echo "prime: unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

if command -v curl >/dev/null 2>&1; then
  api=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")
elif command -v wget >/dev/null 2>&1; then
  api=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest")
else
  echo "prime: curl or wget is required" >&2
  exit 1
fi

ext="tar.gz"
if [ "$os" = "Windows" ]; then ext="zip"; fi

url=$(printf '%s' "$api" | tr -d ' \n' \
  | grep -o "\"browser_download_url\":\"[^\"]*_${os}_${arch}\.${ext}\"" \
  | cut -d'"' -f4 | head -n1)

if [ -z "$url" ]; then
  echo "prime: no release asset found for ${os}/${arch}" >&2
  echo "prime: see https://github.com/$REPO/releases" >&2
  exit 1
fi

name=$(basename "$url")
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "prime: downloading $name"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp/$name"
else
  wget -qO "$tmp/$name" "$url"
fi

sum_url=$(printf '%s' "$api" | tr -d ' \n' \
  | grep -o "\"browser_download_url\":\"[^\"]*checksums\.txt\"" \
  | cut -d'"' -f4 | head -n1)

if [ -n "$sum_url" ]; then
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$sum_url" -o "$tmp/checksums.txt"
  else
    wget -qO "$tmp/checksums.txt" "$sum_url"
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    expected=$(grep " ${name}$" "$tmp/checksums.txt" | cut -d' ' -f1)
    actual=$(sha256sum "$tmp/$name" | cut -d' ' -f1)
  elif command -v shasum >/dev/null 2>&1; then
    expected=$(grep " ${name}$" "$tmp/checksums.txt" | cut -d' ' -f1)
    actual=$(shasum -a 256 "$tmp/$name" | cut -d' ' -f1)
  else
    expected=""; actual="x"
  fi
  if [ -n "$expected" ] && [ "$expected" != "$actual" ]; then
    echo "prime: checksum verification failed" >&2
    exit 1
  fi
fi

if [ "$ext" = "zip" ]; then
  command -v unzip >/dev/null 2>&1 || { echo "prime: unzip is required" >&2; exit 1; }
  unzip -q "$tmp/$name" -d "$tmp"
else
  tar -xzf "$tmp/$name" -C "$tmp"
fi

binary=$(find "$tmp" -type f -name "prime*" ! -name "*.gz" ! -name "*.zip" ! -name "*.txt" | head -n1)
if [ -z "$binary" ]; then
  echo "prime: could not locate binary in archive" >&2
  exit 1
fi

if [ "$os" = "Windows" ]; then binary_name="prime.exe"; else binary_name="prime"; fi
mkdir -p "$BIN_DIR"
mv "$binary" "$BIN_DIR/$binary_name"
chmod 755 "$BIN_DIR/$binary_name"

echo "prime: installed to $BIN_DIR/$binary_name"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "prime: add $BIN_DIR to your PATH" ;;
esac
