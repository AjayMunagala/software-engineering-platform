#!/usr/bin/env bash
set -euo pipefail
[[ "$(id -un)" == postgres ]] || { echo 'run as postgres OS user' >&2; exit 2; }
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd "$script_dir/../../../.." && pwd)"
go_binary=/opt/aegis-go-1.26.2/bin/go
[[ -x "$go_binary" ]] || { echo 'pinned Go missing'; exit 2; }
atlas version | grep -F 'v1.2.3' >/dev/null
pg_bin="$(pg_config --bindir)"
temporary_root="$(mktemp -d /tmp/aegis-die505-XXXXXX)"
cluster="$temporary_root/data"
port="$((58000 + $$ % 1000))"
database="aegis_die505_$$"
started=false
cleanup() {
 if [[ "$started" == true ]]; then "$pg_bin/pg_ctl" -D "$cluster" -m fast -w stop >/dev/null || true; fi
 # Only this harness's validated disposable directory may be removed.
 case "$temporary_root" in /tmp/aegis-die505-*) rm -rf -- "$temporary_root";; esac
}
trap cleanup EXIT
"$pg_bin/initdb" -D "$cluster" --no-locale --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
"$pg_bin/pg_ctl" -D "$cluster" -l "$temporary_root/server.log" -o "-p $port -c listen_addresses=127.0.0.1 -c unix_socket_directories='$temporary_root'" -w start >/dev/null
started=true
"$pg_bin/createdb" -h 127.0.0.1 -p "$port" "$database"
atlas migrate apply --dir "file://$backend_dir/persistence/postgres/migrations" --revisions-schema atlas_schema_revisions --url "postgres://postgres@127.0.0.1:$port/$database?sslmode=disable" --tx-mode file >/dev/null
"$pg_bin/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p "$port" -d "$database" -c 'CREATE ROLE die505_runtime LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION; GRANT platform_ingestor, platform_artifact_reader, platform_retention_worker TO die505_runtime;' >/dev/null
export AEGIS_DEPENDENCY_DISPOSABLE=1 AEGIS_DEPENDENCY_RECOVERY=0
export AEGIS_RUNTIME_POSTGRES_PORT="$port" AEGIS_RUNTIME_POSTGRES_DATABASE="$database" AEGIS_RUNTIME_POSTGRES_USER=die505_runtime
export GOPROXY=off GOSUMDB=off GOMAXPROCS=4
cd "$backend_dir"
platform="${1:-linux}"
run_validation() {
 if [[ "$platform" == linux ]]; then
  "$go_binary" test ./internal/integration/dependency -run '^TestDisposableDependencyPublication$' -count=1 -timeout=120s -v
 elif [[ "$platform" == windows ]]; then
  powershell.exe -NoProfile -NonInteractive -Command "\$env:AEGIS_DEPENDENCY_DISPOSABLE='1'; \$env:AEGIS_DEPENDENCY_RECOVERY='$AEGIS_DEPENDENCY_RECOVERY'; \$env:AEGIS_DEPENDENCY_RETENTION='${AEGIS_DEPENDENCY_RETENTION:-0}'; \$env:AEGIS_RUNTIME_POSTGRES_PORT='$port'; \$env:AEGIS_RUNTIME_POSTGRES_DATABASE='$AEGIS_RUNTIME_POSTGRES_DATABASE'; \$env:AEGIS_RUNTIME_POSTGRES_USER='die505_runtime'; \$env:GOPROXY='off'; \$env:GOSUMDB='off'; Set-Location '$(wslpath -w "$backend_dir")'; go test ./internal/integration/dependency -run '^TestDisposableDependencyPublication$' -count=1 -timeout=120s -v; exit \$LASTEXITCODE"
 else echo 'invalid platform' >&2; return 2; fi
}
echo 'Environment:'
uname -sr; "$go_binary" version; "$pg_bin/postgres" --version; git --version
echo 'Cluster uses fsync=on, local trust, loopback only, fresh migrations; no existing database.'
run_validation
"$pg_bin/pg_ctl" -D "$cluster" -m immediate -w stop >/dev/null
started=false
"$pg_bin/pg_ctl" -D "$cluster" -l "$temporary_root/server.log" -o "-p $port -c listen_addresses=127.0.0.1 -c unix_socket_directories='$temporary_root'" -w start >/dev/null
started=true
export AEGIS_DEPENDENCY_RECOVERY=1
echo 'Fresh-process recovery after immediate database stop:'
run_validation
"$pg_bin/pg_dump" -h 127.0.0.1 -p "$port" -Fc -f "$temporary_root/backup.dump" "$database"
restored="${database}_restored"
"$pg_bin/createdb" -h 127.0.0.1 -p "$port" "$restored"
"$pg_bin/pg_restore" -h 127.0.0.1 -p "$port" -d "$restored" --exit-on-error "$temporary_root/backup.dump"
export AEGIS_RUNTIME_POSTGRES_DATABASE="$restored"
export AEGIS_DEPENDENCY_RETENTION=1
echo 'Fresh-process recovery after backup/restore:'
run_validation
echo 'PASS disposable publication/crash/restore checkpoint'
