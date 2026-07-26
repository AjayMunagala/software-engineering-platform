#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd "$script_dir/../../.." && pwd)"
go_binary="${AEGIS_GO_BINARY:-go}"

command -v "$go_binary" >/dev/null || {
    echo "Go binary not found: $go_binary" >&2
    exit 2
}

go_version="$($go_binary version)"
case "$go_version" in
    *' go1.26.2 '*) ;;
    *)
        echo "Go 1.26.2 is required; found: $go_version" >&2
        exit 2
        ;;
esac

cd "$backend_dir"

echo '[1/4] Linux runtime tests'
"$go_binary" test ./internal/runtime/... -count=1

echo '[2/4] Linux runtime shuffled regression'
"$go_binary" test ./internal/runtime/... -shuffle=on -count=3

echo '[3/4] Linux runtime vet'
"$go_binary" vet ./internal/runtime/...

echo '[4/4] Linux runtime race validation'
CGO_ENABLED=1 "$go_binary" test -race ./internal/runtime/... -count=1

echo 'Linux runtime validation passed'
