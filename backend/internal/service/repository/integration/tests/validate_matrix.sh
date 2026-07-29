#!/usr/bin/env bash
set -euo pipefail

if (($# != 6)); then
    echo 'usage: validate_matrix.sh ROOT CASE COMMIT OUTPUT_DIR WINDOWS_LIMIT LINUX_LIMIT' >&2
    exit 2
fi

repository_root="$1"
case_id="$2"
commit="$3"
output_dir="$4"
windows_limit="$5"
linux_limit="$6"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
runner="$script_dir/validate_disposable.sh"

bash "$runner" --platform windows --root "$repository_root" --case "$case_id" \
    --commit "$commit" --pass windows-a --workers 8 \
    --output "$output_dir/$case_id-windows-a.json" --timeout 4h \
    --memory-ceiling-bytes "$windows_limit"
bash "$runner" --platform windows --root "$repository_root" --case "$case_id" \
    --commit "$commit" --pass windows-b --workers 8 \
    --output "$output_dir/$case_id-windows-b.json" --timeout 4h \
    --memory-ceiling-bytes "$windows_limit"
bash "$runner" --platform windows --root "$repository_root" --case "$case_id" \
    --commit "$commit" --pass windows-worker1 --workers 1 \
    --output "$output_dir/$case_id-windows-worker1.json" --timeout 4h \
    --memory-ceiling-bytes "$windows_limit"
AEGIS_GO_BINARY=/opt/aegis-go-1.26.2/bin/go bash "$runner" --platform linux \
    --root "$repository_root" --case "$case_id" --commit "$commit" \
    --pass linux-a --workers 8 --output "$output_dir/$case_id-linux-a.json" \
    --timeout 4h --memory-ceiling-bytes "$linux_limit"

echo "Phase 4.0.7 matrix passed: $case_id"
