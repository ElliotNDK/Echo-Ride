#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "$ROOT_DIR"

declare -A SEEN_MODULES=()
MODULE_DIRS=()

while IFS= read -r -d '' file; do
    dir="$(dirname "$file")"
    module_dir=""

    while [[ "$dir" != "." ]]; do
        if [[ -f "$dir/go.mod" ]]; then
            module_dir="$dir"
            break
        fi

        parent="$(dirname "$dir")"

        if [[ "$parent" == "$dir" ]]; then
            break
        fi

        dir="$parent"
    done

    if [[ -z "$module_dir" && -f "go.mod" ]]; then
        module_dir="."
    fi

    if [[ -z "$module_dir" ]]; then
        echo "Error: No go.mod found for $file" >&2
        exit 1
    fi

    if [[ -z "${SEEN_MODULES[$module_dir]+x}" ]]; then
        SEEN_MODULES["$module_dir"]=1
        MODULE_DIRS+=("$module_dir")
    fi

done < <(
    git diff --cached --name-only -z --diff-filter=ACMR -- '*.go'
)

if [[ ${#MODULE_DIRS[@]} -eq 0 ]]; then
    echo "No staged Go modules to lint."
    exit 0
fi

for module_dir in "${MODULE_DIRS[@]}"; do
    echo ""
    echo "Linting module: $module_dir"

    (
        cd "$module_dir"
        golangci-lint run --new-from-rev HEAD ./...
    )
done

echo ""
echo "All affected Go modules passed lint."