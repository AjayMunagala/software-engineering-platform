#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
migration_dir="$script_dir/migrations"
compatibility_check="$script_dir/check_compatibility.sh"
atlas_args=(--dir "file://$migration_dir" --revisions-schema atlas_schema_revisions)

usage() {
    cat >&2 <<'TEXT'
usage:
  migrate.sh validate
  migrate.sh status <database-url>
  migrate.sh apply <database-url> [migration-count]

The URL must identify an explicitly authorized database. The command never
loads an environment file and never prints the URL.
TEXT
    exit 2
}

command -v atlas >/dev/null || {
    echo 'Atlas Community CLI v1.2.3 is required' >&2
    exit 2
}
atlas version | grep -F 'v1.2.3' >/dev/null || {
    echo 'Atlas Community CLI v1.2.3 is required' >&2
    exit 2
}

command_name="${1:-}"
case "$command_name" in
validate)
    [[ $# -eq 1 ]] || usage
    atlas migrate validate --dir "file://$migration_dir"
    ;;
status)
    [[ $# -eq 2 ]] || usage
    database_url="$2"
    if [[ "$database_url" =~ ^postgres(ql)?://[^/@:]+:[^/@]+@ ]]; then
        echo 'password-bearing URLs are not accepted in Phase 3.3' >&2
        exit 2
    fi
    atlas migrate validate --dir "file://$migration_dir"
    bash "$compatibility_check" "$database_url" "$migration_dir"
    atlas migrate status "${atlas_args[@]}" --url "$database_url"
    ;;
apply)
    [[ $# -eq 2 || $# -eq 3 ]] || usage
    database_url="$2"
    if [[ "$database_url" =~ ^postgres(ql)?://[^/@:]+:[^/@]+@ ]]; then
        echo 'password-bearing URLs are not accepted in Phase 3.3' >&2
        exit 2
    fi
    migration_count="${3:-}"
    if [[ -n "$migration_count" && ! "$migration_count" =~ ^[1-9][0-9]*$ ]]; then
        echo 'migration count must be a positive integer' >&2
        exit 2
    fi
    atlas migrate validate --dir "file://$migration_dir"
    bash "$compatibility_check" "$database_url" "$migration_dir"
    if [[ -n "$migration_count" ]]; then
        atlas migrate apply "$migration_count" "${atlas_args[@]}" \
            --url "$database_url" --tx-mode file
    else
        atlas migrate apply "${atlas_args[@]}" \
            --url "$database_url" --tx-mode file
    fi
    ;;
*)
    usage
    ;;
esac
