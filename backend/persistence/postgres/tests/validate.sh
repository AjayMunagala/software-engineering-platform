#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -un)" != "postgres" ]]; then
    echo "run this disposable migration suite as the local postgres OS user" >&2
    exit 2
fi

for command_name in atlas pg_config mktemp cp sha256sum; do
    command -v "$command_name" >/dev/null || {
        echo "missing required command: $command_name" >&2
        exit 2
    }
done

atlas version | grep -F 'v1.2.3' >/dev/null || {
    echo "Atlas Community CLI v1.2.3 is required" >&2
    exit 2
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
migration_dir="$(cd "$script_dir/../migrations" && pwd)"
migration_runner="$(cd "$script_dir/.." && pwd)/migrate.sh"
compatibility_check="$(cd "$script_dir/.." && pwd)/check_compatibility.sh"
run_id="$$"
empty_db="aegis_phase33_${run_id}_empty"
upgrade_db="aegis_phase33_${run_id}_upgrade"
failure_db="aegis_phase33_${run_id}_failure"
concurrent_db="aegis_phase33_${run_id}_concurrent"
migration_login="phase33_migration_${run_id}"
temporary_root="$(mktemp -d)"
cluster_dir="$temporary_root/postgres-data"
socket_dir="$temporary_root/postgres-socket"
cluster_log="$temporary_root/postgres.log"
cluster_port="$((55000 + run_id % 1000))"
pg_bin="$(pg_config --bindir)"
initdb_command="$pg_bin/initdb"
pg_ctl_command="$pg_bin/pg_ctl"
psql_command=("$pg_bin/psql" -X -h "$socket_dir" -p "$cluster_port")
createdb_command=("$pg_bin/createdb" -h "$socket_dir" -p "$cluster_port")
dropdb_command=("$pg_bin/dropdb" -h "$socket_dir" -p "$cluster_port")
server_started=false
database_names=("$empty_db" "$upgrade_db" "$failure_db" "$concurrent_db")

cleanup() {
    local database_name
    if [[ "$server_started" == true ]]; then
        for database_name in "${database_names[@]}"; do
            "${dropdb_command[@]}" --if-exists --force "$database_name" \
                >/dev/null 2>&1 || true
        done
        "$pg_ctl_command" -D "$cluster_dir" -m fast -w stop >/dev/null 2>&1 || true
    fi
    if [[ -d "$temporary_root" && "$temporary_root" == /tmp/* ]]; then
        rm -rf -- "$temporary_root"
    fi
}
trap cleanup EXIT

mkdir -p "$socket_dir"
"$initdb_command" -D "$cluster_dir" --no-locale --encoding=UTF8 \
    --auth-local=trust --auth-host=reject >/dev/null
"$pg_ctl_command" -D "$cluster_dir" -l "$cluster_log" \
    -o "-F -k $socket_dir -p $cluster_port -c listen_addresses=''" \
    -w start >/dev/null
server_started=true

database_url() {
    printf 'postgres:///%s?host=%s&port=%s' "$1" "$socket_dir" "$cluster_port"
}

migration_url() {
    printf 'postgres://%s@/%s?host=%s&port=%s' \
        "$migration_login" "$1" "$socket_dir" "$cluster_port"
}

bootstrap_database() {
    local database_name="$1"
    "${createdb_command[@]}" "$database_name"
    atlas migrate apply 1 "${atlas_args[@]}" \
        --url "$(database_url "$database_name")" --tx-mode file >/dev/null
}

atlas_args=(--dir "file://$migration_dir" --revisions-schema atlas_schema_revisions)

echo '[1/10] validating committed checksum manifest'
bash "$migration_runner" validate

echo '[2/10] proving changed migration bytes are rejected'
tamper_dir="$temporary_root/tamper"
cp -R "$migration_dir" "$tamper_dir"
printf '\n-- intentional validation-only tamper\n' \
    >>"$tamper_dir/202607240001_bootstrap_roles_and_schema.sql"
if atlas migrate validate --dir "file://$tamper_dir" >/dev/null 2>&1; then
    echo 'tampered migration unexpectedly passed checksum validation' >&2
    exit 1
fi

echo '[3/10] installing an empty PostgreSQL 18 database'
empty_started="$(date +%s%N)"
bootstrap_database "$empty_db"
"${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$empty_db" -c \
    "CREATE ROLE $migration_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION; GRANT platform_migrator TO $migration_login;" \
    >/dev/null
migration_is_superuser="$("${psql_command[@]}" -At -U "$migration_login" \
    -d "$empty_db" -c "SELECT rolsuper FROM pg_roles WHERE rolname = current_user")"
[[ "$migration_is_superuser" == 'f' ]] || {
    echo 'ephemeral migration login unexpectedly has superuser rights' >&2
    exit 1
}
bash "$migration_runner" apply "$(migration_url "$empty_db")" >/dev/null
empty_finished="$(date +%s%N)"
empty_install_ms="$(( (empty_finished - empty_started) / 1000000 ))"
"${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$empty_db" \
    -f "$script_dir/schema_assertions.sql" >/dev/null
"${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$empty_db" \
    -f "$script_dir/privilege_assertions.sql" >/dev/null

echo '[4/10] proving runtime DDL and restricted reads are denied'
if "${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$empty_db" \
    -c 'SET ROLE platform_ingestor; CREATE TABLE platform.forbidden_runtime_ddl(id integer);' \
    >/dev/null 2>&1; then
    echo 'runtime DDL unexpectedly succeeded' >&2
    exit 1
fi
if "${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$empty_db" \
    -c 'SET ROLE platform_query_reader; SELECT * FROM platform.artifact_payload_chunks LIMIT 0;' \
    >/dev/null 2>&1; then
    echo 'query reader unexpectedly read exact payload chunks' >&2
    exit 1
fi
"${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$empty_db" \
    -c 'SET ROLE platform_artifact_reader; SELECT * FROM platform.artifact_payload_chunks LIMIT 0;' \
    >/dev/null

echo '[5/10] validating partial install followed by supported upgrade'
bootstrap_database "$upgrade_db"
bash "$migration_runner" apply "$(migration_url "$upgrade_db")" 2 >/dev/null
partial_count="$("${psql_command[@]}" -At -d "$upgrade_db" -c \
    'SELECT count(*) FROM atlas_schema_revisions.atlas_schema_revisions')"
[[ "$partial_count" == '3' ]] || {
    echo "expected three partial-install revisions, got $partial_count" >&2
    exit 1
}
bash "$migration_runner" apply "$(migration_url "$upgrade_db")" >/dev/null
"${psql_command[@]}" -v ON_ERROR_STOP=1 -d "$upgrade_db" \
    -f "$script_dir/schema_assertions.sql" >/dev/null

echo '[6/10] validating repeat apply as a deterministic no-op'
noop_started="$(date +%s%N)"
bash "$migration_runner" apply "$(migration_url "$upgrade_db")" >/dev/null
noop_finished="$(date +%s%N)"
noop_apply_ms="$(( (noop_finished - noop_started) / 1000000 ))"

echo '[7/10] rejecting a database newer than the available migration directory'
older_dir="$temporary_root/older"
cp -R "$migration_dir" "$older_dir"
rm -- "$older_dir/202607240007_apply_runtime_privileges.sql"
atlas migrate hash --dir "file://$older_dir"
if bash "$compatibility_check" "$(migration_url "$upgrade_db")" "$older_dir" \
    >/dev/null 2>&1; then
    echo 'newer database unexpectedly matched an older migration directory' >&2
    exit 1
fi

echo '[8/10] validating transactional failure rollback'
failure_dir="$temporary_root/failure"
cp -R "$migration_dir" "$failure_dir"
cat >"$failure_dir/202607240008_intentional_failure.sql" <<'SQL'
SET lock_timeout = '5s';
SET statement_timeout = '5min';
CREATE TABLE platform.must_rollback (id integer PRIMARY KEY);
SELECT 1 / 0;
SQL
atlas migrate hash --dir "file://$failure_dir"
"${createdb_command[@]}" "$failure_db"
if atlas migrate apply --dir "file://$failure_dir" \
    --revisions-schema atlas_schema_revisions \
    --url "$(database_url "$failure_db")" --tx-mode file >/dev/null 2>&1; then
    echo 'intentionally failing migration unexpectedly succeeded' >&2
    exit 1
fi
rollback_residue="$("${psql_command[@]}" -At -d "$failure_db" -c \
    "SELECT to_regclass('platform.must_rollback') IS NOT NULL")"
[[ "$rollback_residue" == 'f' ]] || {
    echo 'failed migration left schema residue' >&2
    exit 1
}
failed_revision="$("${psql_command[@]}" -At -d "$failure_db" -c \
    "SELECT count(*) FROM atlas_schema_revisions.atlas_schema_revisions WHERE version = '202607240008'")"
[[ "$failed_revision" == '0' ]] || {
    echo 'failed transactional migration was recorded as a revision' >&2
    exit 1
}

echo '[9/10] validating advisory-lock concurrency'
concurrent_dir="$temporary_root/concurrent"
cp -R "$migration_dir" "$concurrent_dir"
cat >"$concurrent_dir/202607240008_lock_probe.sql" <<'SQL'
SET lock_timeout = '5s';
SET statement_timeout = '5min';
SELECT pg_sleep(1);
CREATE TABLE platform.migration_lock_probe (id integer PRIMARY KEY);
ALTER TABLE platform.migration_lock_probe OWNER TO platform_owner;
SQL
atlas migrate hash --dir "file://$concurrent_dir"
"${createdb_command[@]}" "$concurrent_db"
atlas migrate apply --dir "file://$concurrent_dir" \
    --revisions-schema atlas_schema_revisions \
    --url "$(database_url "$concurrent_db")" --tx-mode file \
    >"$temporary_root/concurrent-one.log" 2>&1 &
first_pid=$!
atlas migrate apply --dir "file://$concurrent_dir" \
    --revisions-schema atlas_schema_revisions \
    --url "$(database_url "$concurrent_db")" --tx-mode file \
    >"$temporary_root/concurrent-two.log" 2>&1 &
second_pid=$!
wait "$first_pid"
wait "$second_pid"
concurrent_revisions="$("${psql_command[@]}" -At -d "$concurrent_db" -c \
    'SELECT count(*) FROM atlas_schema_revisions.atlas_schema_revisions')"
[[ "$concurrent_revisions" == '8' ]] || {
    echo "expected eight serialized revisions, got $concurrent_revisions" >&2
    exit 1
}

echo '[10/10] collecting deterministic migration metrics'
table_count="$("${psql_command[@]}" -At -d "$empty_db" -c \
    "SELECT count(*) FROM information_schema.tables WHERE table_schema='platform' AND table_type='BASE TABLE'")"
constraint_count="$("${psql_command[@]}" -At -d "$empty_db" -c \
    "SELECT count(*) FROM pg_constraint c JOIN pg_namespace n ON n.oid=c.connamespace WHERE n.nspname='platform'")"
index_count="$("${psql_command[@]}" -At -d "$empty_db" -c \
    "SELECT count(*) FROM pg_indexes WHERE schemaname='platform'")"
role_count="$("${psql_command[@]}" -At -d "$empty_db" -c \
    "SELECT count(*) FROM pg_roles WHERE rolname LIKE 'platform_%'")"

printf 'PASS Phase 3.3 migration validation\n'
printf 'atlas_version=v1.2.3\n'
printf 'postgres_version=%s\n' \
    "$("${psql_command[@]}" -At -d "$empty_db" -c 'SHOW server_version')"
printf 'migration_files=7\n'
printf 'schema_tables=%s\n' "$table_count"
printf 'schema_constraints=%s\n' "$constraint_count"
printf 'schema_indexes=%s\n' "$index_count"
printf 'capability_roles=%s\n' "$role_count"
printf 'empty_install_ms=%s\n' "$empty_install_ms"
printf 'noop_apply_ms=%s\n' "$noop_apply_ms"
