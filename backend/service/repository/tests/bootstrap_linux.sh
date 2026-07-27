#!/usr/bin/env bash
set -euo pipefail

for command_name in curl python3 sha256sum tar mktemp; do
    command -v "$command_name" >/dev/null || {
        echo "missing required command: $command_name" >&2
        exit 2
    }
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
temporary_root="$(mktemp -d /tmp/aegis-repository-service-contract-go-XXXXXX)"
archive="$temporary_root/go.tar.gz"

cleanup() {
    if [[ -d "$temporary_root" && "$temporary_root" == /tmp/aegis-repository-service-contract-go-* ]]; then
        rm -rf -- "$temporary_root"
    fi
}
trap cleanup EXIT

echo '[bootstrap] downloading official Go 1.26.2 Linux toolchain'
download_metadata="$(curl -fsSL 'https://go.dev/dl/?mode=json&include=all')"
expected_sha="$(python3 -c 'import json,sys; data=json.load(sys.stdin); print(next(f["sha256"] for r in data if r["version"]=="go1.26.2" for f in r["files"] if f["filename"]=="go1.26.2.linux-amd64.tar.gz"))' <<<"$download_metadata")"
[[ "$expected_sha" =~ ^[0-9a-f]{64}$ ]] || {
    echo 'official Go checksum was unavailable' >&2
    exit 2
}

curl -fsSL 'https://go.dev/dl/go1.26.2.linux-amd64.tar.gz' -o "$archive"
printf '%s  %s\n' "$expected_sha" "$archive" | sha256sum -c -
tar -C "$temporary_root" -xzf "$archive"

AEGIS_GO_BINARY="$temporary_root/go/bin/go" bash "$script_dir/validate_linux.sh"
