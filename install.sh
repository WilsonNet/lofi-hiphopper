#!/bin/sh
# lofi-hiphopper installer (macOS only).
set -eu

REPO="WilsonNet/lofi-hiphopper"
BIN="lofi-hiphopper"
VERSION="${VERSION:-}"
PREFIX="${PREFIX:-$HOME/.local}"
INSTALL_DIR="${INSTALL_DIR:-}"

usage() {
	cat <<EOF
usage: install.sh [options]

Installs the latest lofi-hiphopper release on macOS.

options:
  --version <tag>       install a specific release, for example v0.2.0
  --prefix <dir>        prefix directory (default: \$HOME/.local)
  --install-dir <dir>   exact bin directory (default: <prefix>/bin)
  -h, --help            show this help

environment:
  VERSION, PREFIX, INSTALL_DIR

examples:
  curl -fsSL https://raw.githubusercontent.com/$REPO/main/install.sh | bash
  curl -fsSL .../install.sh | bash -s -- --version v0.2.0
  PREFIX=/opt/homebrew ./install.sh

Linux is not supported yet. This installer only runs on macOS 14.4+.
EOF
}

while [ "$#" -gt 0 ]; do
	case "$1" in
	--version)
		[ "$#" -ge 2 ] || { printf 'install: --version needs a tag\n' >&2; exit 2; }
		VERSION="$2"
		shift 2
		;;
	--prefix)
		[ "$#" -ge 2 ] || { printf 'install: --prefix needs a directory\n' >&2; exit 2; }
		PREFIX="$2"
		shift 2
		;;
	--install-dir)
		[ "$#" -ge 2 ] || { printf 'install: --install-dir needs a directory\n' >&2; exit 2; }
		INSTALL_DIR="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		printf 'install: unknown argument: %s\n' "$1" >&2
		usage >&2
		exit 2
		;;
	esac
done

if [ "$(uname -s)" != "Darwin" ]; then
	printf 'install: lofi-hiphopper is macOS-only for now. Linux support is planned.\n' >&2
	exit 1
fi

os_version="$(sw_vers -productVersion)"
major="${os_version%%.*}"
rest="${os_version#*.}"
minor="${rest%%.*}"
if [ "$major" -lt 14 ] || { [ "$major" -eq 14 ] && [ "$minor" -lt 4 ]; }; then
	printf 'install: macOS 14.4+ is required (found %s).\n' "$os_version" >&2
	exit 1
fi

case "$(uname -m)" in
arm64) arch="arm64" ;;
x86_64) arch="amd64" ;;
*)
	printf 'install: unsupported architecture: %s\n' "$(uname -m)" >&2
	exit 1
	;;
esac

for tool in curl tar shasum; do
	command -v "$tool" >/dev/null 2>&1 || {
		printf 'install: missing required tool: %s\n' "$tool" >&2
		exit 1
	}
done

if [ -z "${INSTALL_DIR:-}" ]; then
	INSTALL_DIR="$PREFIX/bin"
fi

if [ -z "${VERSION:-}" ]; then
	latest_url="$(curl -fsSIL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" 2>/dev/null || true)"
	case "$latest_url" in
	*/tag/*) VERSION="${latest_url##*/}" ;;
	*)
		printf 'install: could not find a published release.\n' >&2
		printf 'Install from source instead: go install github.com/%s@latest\n' "$REPO" >&2
		exit 1
		;;
	esac
fi

case "$VERSION" in
v*) ;;
*) VERSION="v$VERSION" ;;
esac

asset="${BIN}_darwin_${arch}.tar.gz"
base="${BASE_URL:-https://github.com/$REPO/releases/download/$VERSION}"
tmp=""

cleanup() {
	if [ -n "${tmp:-}" ]; then
		rm -rf "$tmp"
	fi
}
trap cleanup 0 1 2 15

tmp="$(mktemp -d "${TMPDIR:-/tmp}/lofi-hiphopper.XXXXXX")"
printf 'Downloading %s (%s)...\n' "$VERSION" "$arch"
curl -fsSL --retry 3 --retry-delay 1 "$base/$asset" -o "$tmp/$asset"
curl -fsSL --retry 3 --retry-delay 1 "$base/checksums.txt" -o "$tmp/checksums.txt"

expected="$(awk -v file="$asset" '$2 == file { print $1; exit }' "$tmp/checksums.txt")"
if [ -z "$expected" ]; then
	printf 'install: checksum for %s is missing.\n' "$asset" >&2
	exit 1
fi
actual="$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')"
if [ "$expected" != "$actual" ]; then
	printf 'install: checksum mismatch for %s\n' "$asset" >&2
	exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp"
if [ ! -f "$tmp/$BIN" ]; then
	printf 'install: archive did not contain %s\n' "$BIN" >&2
	exit 1
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BIN" "$INSTALL_DIR/$BIN"

printf 'Installed %s %s to %s\n' "$BIN" "${VERSION#v}" "$INSTALL_DIR/$BIN"
case ":${PATH:-}:" in
*":$INSTALL_DIR:"*) ;;
*)
	printf '\nAdd it to PATH:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
	;;
esac

missing=""
for tool in mpv yt-dlp ffmpeg; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		missing="$missing $tool"
	fi
done
if [ -n "$missing" ]; then
	printf '\nMissing runtime dependencies:%s\n  brew install%s\n' "$missing" "$missing"
fi

printf '\nRun: %s\n' "$BIN"
