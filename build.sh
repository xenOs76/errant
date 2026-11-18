#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "${ROOT}/dist"

echo "==> Building errant into dist/errant..."
go build -v -o "${ROOT}/dist/errant" "${ROOT}/main.go"
echo "✔ Build complete: ${ROOT}/dist/errant"
