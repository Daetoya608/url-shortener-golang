#!/usr/bin/env bash
# Команда тестов проекта: go vet + go test через scripts/go.sh.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

./scripts/go.sh vet ./...
./scripts/go.sh test ./... -count=1
