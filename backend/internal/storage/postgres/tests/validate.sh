#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -un)" != "postgres" ]]; then
    echo 'run the disposable adapter suite as the local postgres OS user' >&2
    exit 2
fi

for command_name in atlas pg_config mktemp go; do
    command -v "$command_name" >/dev/null || {
        echo "missing required command: $command_name" >&2
        exit 2
    }
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd "$script_dir/../../../.." && pwd)"
migration_runner="$backend_dir/persistence/postgres/migrate.sh"
temporary_root="$(mktemp -d)"
cluster_dir="$temporary_root/postgres-data"
socket_dir="$temporary_root/postgres-socket"
cluster_log="$temporary_root/postgres.log"
cluster_port="$((56000 + $$ % 1000))"
database_name="aegis_adapter_$$"
pg_bin="$(pg_config --bindir)"
server_started=false

cleanup() {
    if [[ "$server_started" == true ]]; then
        "$pg_bin/pg_ctl" -D "$cluster_dir" -m fast -w stop >/dev/null 2>&1 || true
    fi
    if [[ -d "$temporary_root" && "$temporary_root" == /tmp/* ]]; then
		chmod -R u+w "$temporary_root" >/dev/null 2>&1 || true
        rm -rf -- "$temporary_root"
    fi
}
trap cleanup EXIT

mkdir -p "$socket_dir"
"$pg_bin/initdb" -D "$cluster_dir" --no-locale --encoding=UTF8 \
    --auth-local=trust --auth-host=trust >/dev/null
"$pg_bin/pg_ctl" -D "$cluster_dir" -l "$cluster_log" \
    -o "-F -k $socket_dir -p $cluster_port -c listen_addresses='127.0.0.1'" \
    -w start >/dev/null
server_started=true
"$pg_bin/createdb" -h "$socket_dir" -p "$cluster_port" "$database_name"

database_url="postgres://postgres@127.0.0.1:$cluster_port/$database_name?sslmode=disable"
bash "$migration_runner" apply "$database_url" >/dev/null

cd "$backend_dir"
export POSTGRES_TEST_URL="$database_url"
export GOTOOLCHAIN=auto
export GOCACHE="/var/tmp/aegis-go-cache"
export GOMODCACHE="/var/tmp/aegis-go-mod-cache"
mkdir -p "$GOCACHE" "$GOMODCACHE"

echo '[1/5] neutral PostgreSQL adapter conformance'
go test ./internal/storage/postgres -run '^TestPostgresNeutralConformance$' -count=1 -v
echo '[2/5] PostgreSQL rollback, atomicity, integrity, concurrency, and retention'
go test ./internal/storage/postgres -run '^TestPostgres(Rollback|Idempotency|Concurrent|Complete)' -count=1 -v
echo '[3/5] adapter race validation'
go test -race ./internal/storage/postgres -run '^TestPostgres' -count=1
echo '[4/5] adapter statement coverage'
go test ./internal/storage/postgres -coverprofile="$temporary_root/adapter.cover" -count=1 >/dev/null
go tool cover -func="$temporary_root/adapter.cover" | tail -n 1
echo '[5/5] released-scale 1,556,379,091-byte streaming gate'
/usr/bin/time -f 'large_test_elapsed_s=%e large_test_max_rss_kib=%M' \
    env POSTGRES_LARGE_TEST_BYTES=1556379091 \
    go test ./internal/storage/postgres -run '^TestPostgresReleasedScalePayload$' -count=1 -v -timeout=12m
