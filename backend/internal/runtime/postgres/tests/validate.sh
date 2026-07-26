#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -un)" != "postgres" ]]; then
    echo "run this disposable runtime suite as the local postgres OS user" >&2
    exit 2
fi

for command_name in atlas pg_config mktemp openssl powershell.exe; do
    command -v "$command_name" >/dev/null || {
        echo "missing required command: $command_name" >&2
        exit 2
    }
done

atlas version | grep -F 'v1.2.3' >/dev/null || {
    echo 'Atlas Community CLI v1.2.3 is required' >&2
    exit 2
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd "$script_dir/../../../.." && pwd)"
migration_dir="$backend_dir/persistence/postgres/migrations"
run_id="$$"
database_name="aegis_phase352_${run_id}"
runtime_login="phase352_runtime_${run_id}"
ingest_login="phase352_ingest_${run_id}"
read_login="phase352_read_${run_id}"
retention_login="phase352_retention_${run_id}"
temporary_root="$(mktemp -d)"
chmod 755 "$temporary_root"
cluster_dir="$temporary_root/postgres-data"
cluster_log="$temporary_root/postgres.log"
ca_key="$temporary_root/ca.key"
ca_cert="$temporary_root/ca.crt"
server_key="$temporary_root/server.key"
server_request="$temporary_root/server.csr"
server_cert="$temporary_root/server.crt"
server_extensions="$temporary_root/server.ext"
cluster_port="$((56000 + run_id % 1000))"
pg_bin="$(pg_config --bindir)"
server_started=false

cleanup() {
    if [[ "$server_started" == true ]]; then
        "$pg_bin/dropdb" -h 127.0.0.1 -p "$cluster_port" \
            --if-exists --force "$database_name" >/dev/null 2>&1 || true
        "$pg_bin/pg_ctl" -D "$cluster_dir" -m fast -w stop >/dev/null 2>&1 || true
    fi
    if [[ -d "$temporary_root" && "$temporary_root" == /tmp/* ]]; then
        rm -rf -- "$temporary_root"
    fi
}
trap cleanup EXIT

echo '[1/4] creating disposable PostgreSQL 18 cluster'
openssl req -x509 -newkey rsa:2048 -nodes -sha256 -days 1 \
    -subj '/CN=Aegis Runtime Test CA' -keyout "$ca_key" -out "$ca_cert" \
    >/dev/null 2>&1
openssl req -newkey rsa:2048 -nodes -sha256 -subj '/CN=runtime.test' \
    -keyout "$server_key" -out "$server_request" >/dev/null 2>&1
printf 'subjectAltName=DNS:runtime.test\nextendedKeyUsage=serverAuth\n' >"$server_extensions"
openssl x509 -req -sha256 -days 1 -in "$server_request" \
    -CA "$ca_cert" -CAkey "$ca_key" -CAcreateserial \
    -extfile "$server_extensions" -out "$server_cert" >/dev/null 2>&1
chmod 600 "$server_key"
chmod 644 "$ca_cert"
"$pg_bin/initdb" -D "$cluster_dir" --no-locale --encoding=UTF8 \
    --auth-local=trust --auth-host=trust >/dev/null
"$pg_bin/pg_ctl" -D "$cluster_dir" -l "$cluster_log" \
    -o "-F -p $cluster_port -c listen_addresses='127.0.0.1' -c ssl=on -c ssl_cert_file='$server_cert' -c ssl_key_file='$server_key' -c ssl_ca_file='$ca_cert'" \
    -w start >/dev/null
server_started=true
"$pg_bin/createdb" -h 127.0.0.1 -p "$cluster_port" "$database_name"

echo '[2/4] applying checksum-verified accepted migrations'
atlas migrate apply --dir "file://$migration_dir" \
    --revisions-schema atlas_schema_revisions \
    --url "postgres://postgres@127.0.0.1:${cluster_port}/${database_name}?sslmode=disable" \
    --tx-mode file >/dev/null

echo '[3/4] creating a disposable combined-capability login'
"$pg_bin/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p "$cluster_port" \
    -d "$database_name" -c \
    "CREATE ROLE $runtime_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
     CREATE ROLE $ingest_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
     CREATE ROLE $read_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
     CREATE ROLE $retention_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
     GRANT platform_ingestor, platform_artifact_reader, platform_retention_worker TO $runtime_login;
     GRANT platform_ingestor TO $ingest_login;
     GRANT platform_artifact_reader TO $read_login;
     GRANT platform_retention_worker TO $retention_login;" \
    >/dev/null
tls_probe="$(PGSSLMODE=verify-full PGSSLROOTCERT="$ca_cert" PGHOST=runtime.test \
    PGHOSTADDR=127.0.0.1 PGPORT="$cluster_port" PGDATABASE="$database_name" \
    PGUSER="$ingest_login" "$pg_bin/psql" -X -At -c \
    "SELECT EXISTS (SELECT 1 FROM pg_stat_ssl WHERE pid=pg_backend_pid() AND ssl)")"
[[ "$tls_probe" == 't' ]] || {
    echo 'disposable TLS session proof failed before Go validation' >&2
    exit 1
}

echo '[4/4] running the Windows runtime integration test against the disposable cluster'
backend_windows="$(wslpath -w "$backend_dir")"
ca_windows="$(wslpath -w "$ca_cert")"
powershell.exe -NoProfile -NonInteractive -Command \
    "\$env:AEGIS_RUNTIME_POSTGRES_INTEGRATION='1'; \$env:AEGIS_RUNTIME_POSTGRES_PORT='$cluster_port'; \$env:AEGIS_RUNTIME_POSTGRES_DATABASE='$database_name'; \$env:AEGIS_RUNTIME_POSTGRES_USER='$runtime_login'; \$env:AEGIS_RUNTIME_POSTGRES_INGEST_USER='$ingest_login'; \$env:AEGIS_RUNTIME_POSTGRES_READ_USER='$read_login'; \$env:AEGIS_RUNTIME_POSTGRES_RETENTION_USER='$retention_login'; \$env:AEGIS_RUNTIME_POSTGRES_ROOT_CA='$ca_windows'; Set-Location -LiteralPath '$backend_windows'; go test ./internal/runtime/postgres -run '^TestDisposablePostgreSQL' -count=1 -v"

echo 'disposable PostgreSQL runtime validation passed'
