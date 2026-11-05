#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"
cd "$ROOT_DIR"

declare -A SEEN_MODULES=()
MODULE_DIRS=()

while IFS= read -r -d '' file; do
    module_dir="$(dirname "$file")"

    if [[ -z "${SEEN_MODULES[$module_dir]+x}" ]]; then
        SEEN_MODULES["$module_dir"]=1
        MODULE_DIRS+=("$module_dir")
    fi

done < <(
    git diff --cached --name-only -z --diff-filter=ACMR -- \
        'go.mod' 'go.sum' '**/go.mod' '**/go.sum'
)

if [[ ${#MODULE_DIRS[@]} -eq 0 ]]; then
    echo "No staged Go modules to tidy."
    exit 0
fi

for module_dir in "${MODULE_DIRS[@]}"; do
    echo ""
    echo "Tidying module: $module_dir"

    (
        cd "$module_dir"
        go mod tidy
    )

    git add -- "$module_dir/go.mod"

    if [[ -f "$module_dir/go.sum" ]]; then
        git add -- "$module_dir/go.sum"
    fi
done

echo ""
echo "All affected Go modules tidied successfully."