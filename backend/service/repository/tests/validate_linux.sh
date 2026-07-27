#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd "$script_dir/../../.." && pwd)"
go_binary="${AEGIS_GO_BINARY:-go}"
packages='./service/repository/...'

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

echo '[1/6] Linux contract tests and shuffle'
"$go_binary" test "$packages" -count=1
"$go_binary" test "$packages" -shuffle=on -count=5

echo '[2/6] Linux full backend regression'
"$go_binary" test ./... -count=1

echo '[3/6] Linux vet'
"$go_binary" vet ./...

echo '[4/6] Linux targeted race validation'
CGO_ENABLED=1 "$go_binary" test -race "$packages" -count=1

echo '[5/6] Linux full backend race validation'
CGO_ENABLED=1 "$go_binary" test -race ./... -count=1

echo '[6/6] Linux contract benchmarks'
"$go_binary" test -run '^$' -bench . -benchmem -count=3 "$packages"

echo 'Linux Repository Service contract validation passed'
