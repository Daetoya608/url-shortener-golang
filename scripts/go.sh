#!/usr/bin/env bash
# Обёртка над go: использует go из PATH, иначе локальный тулчейн в ./.tools/go
# (скачивается при первом запуске).
set -euo pipefail

GO_VERSION="1.27.2"
GO_URL="https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS_DIR="$ROOT/.tools"
LOCAL_GO="$TOOLS_DIR/go/bin/go"

if command -v go >/dev/null 2>&1; then
	exec go "$@"
fi

if [ ! -x "$LOCAL_GO" ]; then
	echo "go.sh: go не найден в PATH, скачиваю $GO_URL" >&2
	mkdir -p "$TOOLS_DIR"
	tmp="$(mktemp -d "$TOOLS_DIR/dl.XXXXXX")"
	trap 'rm -rf "$tmp"' EXIT
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$GO_URL" -o "$tmp/go.tar.gz"
	else
		wget -q "$GO_URL" -O "$tmp/go.tar.gz"
	fi
	tar -xzf "$tmp/go.tar.gz" -C "$tmp"
	rm -rf "$TOOLS_DIR/go"
	mv "$tmp/go" "$TOOLS_DIR/go"
	rm -rf "$tmp"
	trap - EXIT
fi

export PATH="$TOOLS_DIR/go/bin:$PATH"
exec go "$@"
