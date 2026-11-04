#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROTO_DIR="$PROJECT_ROOT/proto-service"

if ! command -v buf >/dev/null 2>&1; then
  echo "Error: buf is not installed or not in PATH." >&2
  exit 1
fi

cd "$PROTO_DIR"

echo "Generating protobuf and gRPC code..."
buf generate

echo "Generation completed successfully."
