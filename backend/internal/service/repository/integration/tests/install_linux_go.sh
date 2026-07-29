#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" != 0 ]]; then
    echo 'run this dedicated toolchain installer as root' >&2
    exit 2
fi

target='/opt/aegis-go-1.26.2'
if [[ -x "$target/bin/go" ]]; then
    "$target/bin/go" version
    exit 0
fi
[[ ! -e "$target" ]] || {
    echo "unexpected existing target: $target" >&2
    exit 2
}

for command_name in curl python3 sha256sum tar mktemp; do
    command -v "$command_name" >/dev/null || {
        echo "missing required command: $command_name" >&2
        exit 2
    }
done

temporary_root="$(mktemp -d /tmp/aegis-go-install-XXXXXX)"
cleanup() {
    if [[ -d "$temporary_root" && "$temporary_root" == /tmp/aegis-go-install-* ]]; then
        rm -rf -- "$temporary_root"
    fi
}
trap cleanup EXIT

curl -fsSL 'https://go.dev/dl/?mode=json&include=all' -o "$temporary_root/releases.json"
expected_sha="$(python3 -c 'import json,sys; data=json.load(open(sys.argv[1])); print(next(f["sha256"] for r in data if r["version"]=="go1.26.2" for f in r["files"] if f["filename"]=="go1.26.2.linux-amd64.tar.gz"))' "$temporary_root/releases.json")"
[[ "$expected_sha" =~ ^[0-9a-f]{64}$ ]] || {
    echo 'official Go checksum is unavailable' >&2
    exit 2
}
curl -fsSL 'https://go.dev/dl/go1.26.2.linux-amd64.tar.gz' -o "$temporary_root/go.tar.gz"
printf '%s  %s\n' "$expected_sha" "$temporary_root/go.tar.gz" | sha256sum -c -
mkdir -p "$target"
tar -C "$target" --strip-components=1 -xzf "$temporary_root/go.tar.gz"
"$target/bin/go" version
