#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
    echo 'usage: check_compatibility.sh <database-url> <migration-directory>' >&2
    exit 2
fi

database_url="$1"
migration_dir="$2"

for command_name in psql find sort basename; do
    command -v "$command_name" >/dev/null || {
        echo "missing compatibility-check command: $command_name" >&2
        exit 2
    }
done

[[ -d "$migration_dir" && -f "$migration_dir/atlas.sum" ]] || {
    echo 'migration directory or checksum manifest is missing' >&2
    exit 2
}

declare -A supported_versions=()
local_latest=''
while IFS= read -r migration_path; do
    migration_file="$(basename "$migration_path")"
    if [[ ! "$migration_file" =~ ^([0-9]{12})_[a-z0-9_]+\.sql$ ]]; then
        echo "invalid migration filename: $migration_file" >&2
        exit 2
    fi
    migration_version="${BASH_REMATCH[1]}"
    if [[ -n "${supported_versions[$migration_version]:-}" ]]; then
        echo "duplicate migration version: $migration_version" >&2
        exit 2
    fi
    supported_versions[$migration_version]="$migration_file"
    local_latest="$migration_version"
done < <(find "$migration_dir" -maxdepth 1 -type f -name '*.sql' -print | sort)

[[ -n "$local_latest" ]] || {
    echo 'migration directory contains no SQL versions' >&2
    exit 2
}

revision_table_exists="$(psql "$database_url" -XAt -v ON_ERROR_STOP=1 \
    -c "SELECT to_regclass('atlas_schema_revisions.atlas_schema_revisions') IS NOT NULL")"
if [[ "$revision_table_exists" == 'f' ]]; then
    exit 0
fi

while IFS='|' read -r database_version applied total revision_error; do
    [[ -n "$database_version" ]] || continue
    if [[ -z "${supported_versions[$database_version]:-}" ]]; then
        echo "database revision $database_version is unsupported by this migration directory" >&2
        exit 1
    fi
    if [[ "$applied" != "$total" || -n "$revision_error" ]]; then
        echo "database revision $database_version is incomplete" >&2
        exit 1
    fi
done < <(psql "$database_url" -XAt -F '|' -v ON_ERROR_STOP=1 -c \
    "SELECT version, applied, total, error FROM atlas_schema_revisions.atlas_schema_revisions ORDER BY version")

database_latest="$(psql "$database_url" -XAt -v ON_ERROR_STOP=1 -c \
    "SELECT COALESCE(max(version), '') FROM atlas_schema_revisions.atlas_schema_revisions")"
if [[ -n "$database_latest" && "$database_latest" > "$local_latest" ]]; then
    echo "database schema $database_latest is newer than supported schema $local_latest" >&2
    exit 1
fi
