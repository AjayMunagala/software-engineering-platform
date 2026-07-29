#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat >&2 <<'EOF'
usage: validate_disposable.sh --platform windows|linux --root PATH --case ID \
  --commit SHA40 --pass LABEL --workers 1|8 --output PATH [--timeout DURATION] \
  [--memory-ceiling-bytes N] [--go-memory-limit VALUE] [--crash-recovery]
EOF
    exit 2
}

platform=''
repository_root=''
case_id=''
expected_commit=''
pass_id=''
workers=''
output=''
validation_timeout='30m'
memory_ceiling='0'
go_memory_limit=''
crash_recovery=false

while (($#)); do
    case "$1" in
        --platform) platform="${2:-}"; shift 2 ;;
        --root) repository_root="${2:-}"; shift 2 ;;
        --case) case_id="${2:-}"; shift 2 ;;
        --commit) expected_commit="${2:-}"; shift 2 ;;
        --pass) pass_id="${2:-}"; shift 2 ;;
        --workers) workers="${2:-}"; shift 2 ;;
        --output) output="${2:-}"; shift 2 ;;
        --timeout) validation_timeout="${2:-}"; shift 2 ;;
        --memory-ceiling-bytes) memory_ceiling="${2:-}"; shift 2 ;;
        --go-memory-limit) go_memory_limit="${2:-}"; shift 2 ;;
        --crash-recovery) crash_recovery=true; shift ;;
        *) usage ;;
    esac
done

[[ "$platform" == windows || "$platform" == linux ]] || usage
[[ "$repository_root" == /* && -d "$repository_root" ]] || usage
[[ "$case_id" =~ ^[a-z0-9-]{1,64}$ ]] || usage
[[ "$pass_id" =~ ^[a-z0-9-]{1,64}$ ]] || usage
[[ "$expected_commit" =~ ^[0-9a-f]{40}$ ]] || usage
[[ "$workers" == 1 || "$workers" == 8 ]] || usage
[[ "$output" == /* ]] || usage
[[ "$memory_ceiling" =~ ^[0-9]+$ ]] || usage
[[ -z "$go_memory_limit" || "$go_memory_limit" =~ ^[1-9][0-9]*(KiB|MiB|GiB|TiB)$ ]] || usage

if [[ "$(id -un)" != postgres ]]; then
    echo 'run this disposable suite as the local postgres OS user' >&2
    exit 2
fi

for command_name in atlas git pg_config mktemp openssl; do
    command -v "$command_name" >/dev/null || {
        echo "missing required command: $command_name" >&2
        exit 2
    }
done
if [[ "$platform" == windows ]]; then
    command -v powershell.exe >/dev/null || {
        echo 'powershell.exe is required for the Windows pass' >&2
        exit 2
    }
else
    go_binary="${AEGIS_GO_BINARY:-go}"
    "$go_binary" version | grep -F 'go1.26.2 linux/amd64' >/dev/null || {
        echo 'checksum-verified Go 1.26.2 is required for the Linux pass' >&2
        exit 2
    }
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
backend_dir="$(cd "$script_dir/../../../../.." && pwd)"
migration_dir="$backend_dir/persistence/postgres/migrations"
mkdir -p "$(dirname "$output")"
output="$(realpath -m "$output")"
repository_root="$(realpath "$repository_root")"
case "$output" in
    "$repository_root"|"$repository_root"/*)
        echo 'validation output must remain outside the repository' >&2
        exit 2
        ;;
esac

outcome_file="${output%.json}.outcome.json"
log_file="${output%.json}.log"
recovery_output="${output%.json}.recovery.json"
stage='preflight'
server_started=false
completed=false
temporary_root=''

write_outcome() {
    local classification="$1"
    printf '{"case":"%s","pass":"%s","platform":"%s","classification":"%s"}\n' \
        "$case_id" "$pass_id" "$platform" "$classification" >"$outcome_file"
    chmod 600 "$outcome_file" 2>/dev/null || true
}

cleanup() {
    local status=$?
    if [[ "$server_started" == true && -n "$temporary_root" ]]; then
        "$pg_bin/pg_ctl" -D "$cluster_dir" -m fast -w stop >/dev/null 2>&1 || true
    fi
    if [[ -n "$temporary_root" && -d "$temporary_root" && "$temporary_root" == /tmp/aegis-phase407-* ]]; then
        rm -rf -- "$temporary_root"
    fi
    if [[ "$completed" != true ]]; then
        if [[ -f "$log_file" ]] && grep -Fq 'memory ceiling exceeded' "$log_file"; then
            write_outcome memory_ceiling
        elif [[ -f "$log_file" ]] && grep -Fq 'validation timeout exceeded' "$log_file"; then
            write_outcome timeout
        elif [[ "$stage" == preflight || "$stage" == database || "$stage" == runtime ]]; then
            write_outcome environment_failure
        else
            write_outcome correctness_failure
        fi
    fi
    return "$status"
}
trap cleanup EXIT

# The corpus is stored on the Windows-mounted NTFS volume so both native
# Windows and WSL consume the exact same bytes. Use native Git for that volume:
# WSL Git's full untracked scan over the Kubernetes checkout is prohibitively
# slow and interprets Windows checkout normalization differently. Content,
# path, index, and revision changes remain fatal.
git_command=git
git_root="$repository_root"
if [[ "$repository_root" == /mnt/* && -n "$(command -v git.exe || true)" ]]; then
    git_command=git.exe
    git_root="$(wslpath -w "$repository_root")"
fi
git_fixture=("$git_command" -c "safe.directory=$git_root" -c core.fileMode=false -c core.autocrlf=true -C "$git_root")
actual_commit="$("${git_fixture[@]}" rev-parse HEAD)"
[[ "$actual_commit" == "$expected_commit" ]] || {
    echo 'repository revision does not match the approved fixture' >&2
    exit 1
}
[[ -z "$("${git_fixture[@]}" status --porcelain=v1 --untracked-files=all)" ]] || {
    echo 'repository fixture is not clean' >&2
    exit 1
}
tree_id="$("${git_fixture[@]}" rev-parse 'HEAD^{tree}')"
git_version="$("$git_command" --version | tr -d '\r')"

temporary_root="$(mktemp -d /tmp/aegis-phase407-XXXXXX)"
chmod 700 "$temporary_root"
tracked_list="${output%.json}.tracked.bin"
"${git_fixture[@]}" -c core.quotePath=false ls-files --stage -z >"$tracked_list"
chmod 600 "$tracked_list" 2>/dev/null || true

stage='database'
cluster_dir="$temporary_root/postgres-data"
cluster_log="$temporary_root/postgres.log"
run_id="$$"
cluster_port="$((57000 + run_id % 1000))"
database_name="aegis_phase407_${run_id}"
runtime_login="phase407_runtime_${run_id}"
pg_bin="$(pg_config --bindir)"
"$pg_bin/initdb" -D "$cluster_dir" --no-locale --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
"$pg_bin/pg_ctl" -D "$cluster_dir" -l "$cluster_log" -o "-p $cluster_port -c listen_addresses='127.0.0.1'" -w start >/dev/null
server_started=true
"$pg_bin/createdb" -h 127.0.0.1 -p "$cluster_port" "$database_name"
atlas migrate apply --dir "file://$migration_dir" --revisions-schema atlas_schema_revisions \
    --url "postgres://postgres@127.0.0.1:${cluster_port}/${database_name}?sslmode=disable" \
    --tx-mode file >/dev/null
"$pg_bin/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p "$cluster_port" -d "$database_name" -c \
    "CREATE ROLE $runtime_login LOGIN INHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
     GRANT platform_ingestor, platform_artifact_reader, platform_retention_worker TO $runtime_login;" >/dev/null

admin_url="postgres://postgres@127.0.0.1:${cluster_port}/${database_name}?sslmode=disable"
export POSTGRES_TEST_URL="$admin_url"
export AEGIS_RUNTIME_POSTGRES_PORT="$cluster_port"
export AEGIS_RUNTIME_POSTGRES_DATABASE="$database_name"
export AEGIS_RUNTIME_POSTGRES_USER="$runtime_login"
export AEGIS_REPOSITORY_SERVICE_REAL_VALIDATION=1
export AEGIS_REAL_CASE="$case_id"
export AEGIS_REAL_PASS="$pass_id"
export AEGIS_REAL_COMMIT="$actual_commit"
export AEGIS_REAL_TREE="$tree_id"
export AEGIS_REAL_GIT_VERSION="$git_version"
export AEGIS_REAL_TIMEOUT="$validation_timeout"
if [[ "$memory_ceiling" == 0 ]]; then
    unset AEGIS_REAL_MEMORY_CEILING_BYTES || true
else
    export AEGIS_REAL_MEMORY_CEILING_BYTES="$memory_ceiling"
fi
export GOMAXPROCS="$workers"
export GOPROXY=off
export GOSUMDB=off
if [[ -n "$go_memory_limit" ]]; then
    export GOMEMLIMIT="$go_memory_limit"
else
    unset GOMEMLIMIT || true
fi

stage='runtime'
if [[ "$platform" == windows ]]; then
    backend_target="$(wslpath -w "$backend_dir")"
    export AEGIS_REAL_ROOT="$(wslpath -w "$repository_root")"
    export AEGIS_REAL_TRACKED_LIST="$(wslpath -w "$tracked_list")"
    export AEGIS_REAL_OUTPUT="$(wslpath -w "$output")"
    windows_memory=''
    if [[ "$memory_ceiling" != 0 ]]; then
        windows_memory="\$env:AEGIS_REAL_MEMORY_CEILING_BYTES='$memory_ceiling';"
    fi
    windows_go_memory=''
    if [[ -n "$go_memory_limit" ]]; then
        windows_go_memory="\$env:GOMEMLIMIT='$go_memory_limit';"
    fi
    powershell.exe -NoProfile -NonInteractive -Command \
        "\$env:POSTGRES_TEST_URL='$admin_url'; \$env:AEGIS_RUNTIME_POSTGRES_PORT='$cluster_port'; \$env:AEGIS_RUNTIME_POSTGRES_DATABASE='$database_name'; \$env:AEGIS_RUNTIME_POSTGRES_USER='$runtime_login'; \$env:AEGIS_REPOSITORY_SERVICE_REAL_VALIDATION='1'; \$env:AEGIS_REAL_CASE='$case_id'; \$env:AEGIS_REAL_PASS='$pass_id'; \$env:AEGIS_REAL_COMMIT='$actual_commit'; \$env:AEGIS_REAL_TREE='$tree_id'; \$env:AEGIS_REAL_GIT_VERSION='$git_version'; \$env:AEGIS_REAL_TIMEOUT='$validation_timeout'; \$env:AEGIS_REAL_ROOT='$AEGIS_REAL_ROOT'; \$env:AEGIS_REAL_TRACKED_LIST='$AEGIS_REAL_TRACKED_LIST'; \$env:AEGIS_REAL_OUTPUT='$AEGIS_REAL_OUTPUT'; \$env:GOMAXPROCS='$workers'; \$env:GOPROXY='off'; \$env:GOSUMDB='off'; $windows_memory $windows_go_memory Set-Location -LiteralPath '$backend_target'; go test ./internal/service/repository/integration -run '^TestRealRepositoryValidation$' -count=1 -timeout '$validation_timeout' -v" \
        2>&1 | tee "$log_file"
else
    export AEGIS_REAL_ROOT="$repository_root"
    export AEGIS_REAL_TRACKED_LIST="$tracked_list"
    export AEGIS_REAL_OUTPUT="$output"
    (cd "$backend_dir" && "$go_binary" test ./internal/service/repository/integration -run '^TestRealRepositoryValidation$' -count=1 -timeout "$validation_timeout" -v) \
        2>&1 | tee "$log_file"
fi
[[ -s "$output" ]] || {
    echo 'real-repository test did not produce its required result' >&2
    exit 1
}

if [[ "$crash_recovery" == true ]]; then
    stage='recovery'
    "$pg_bin/pg_ctl" -D "$cluster_dir" -m immediate -w stop >/dev/null
    server_started=false
    "$pg_bin/pg_ctl" -D "$cluster_dir" -l "$cluster_log" -o "-p $cluster_port -c listen_addresses='127.0.0.1'" -w start >/dev/null
    server_started=true
    export AEGIS_REPOSITORY_SERVICE_REAL_VALIDATION=0
    export AEGIS_REPOSITORY_SERVICE_REAL_RECOVERY=1
    if [[ "$platform" == windows ]]; then
        export AEGIS_REAL_RECOVERY_INPUT="$(wslpath -w "$output")"
        export AEGIS_REAL_RECOVERY_OUTPUT="$(wslpath -w "$recovery_output")"
        powershell.exe -NoProfile -NonInteractive -Command \
            "\$env:POSTGRES_TEST_URL='$admin_url'; \$env:AEGIS_RUNTIME_POSTGRES_PORT='$cluster_port'; \$env:AEGIS_RUNTIME_POSTGRES_DATABASE='$database_name'; \$env:AEGIS_RUNTIME_POSTGRES_USER='$runtime_login'; \$env:AEGIS_REPOSITORY_SERVICE_REAL_RECOVERY='1'; \$env:AEGIS_REAL_ROOT='$AEGIS_REAL_ROOT'; \$env:AEGIS_REAL_RECOVERY_INPUT='$AEGIS_REAL_RECOVERY_INPUT'; \$env:AEGIS_REAL_RECOVERY_OUTPUT='$AEGIS_REAL_RECOVERY_OUTPUT'; \$env:GOPROXY='off'; \$env:GOSUMDB='off'; Set-Location -LiteralPath '$backend_target'; go test ./internal/service/repository/integration -run '^TestRealRepositoryCrashRecovery$' -count=1 -v" \
            2>&1 | tee -a "$log_file"
    else
        export AEGIS_REAL_RECOVERY_INPUT="$output"
        export AEGIS_REAL_RECOVERY_OUTPUT="$recovery_output"
        (cd "$backend_dir" && "$go_binary" test ./internal/service/repository/integration -run '^TestRealRepositoryCrashRecovery$' -count=1 -v) \
            2>&1 | tee -a "$log_file"
    fi
    [[ -s "$recovery_output" ]] || {
        echo 'crash-recovery test did not produce its required result' >&2
        exit 1
    }
fi

stage='complete'
completed=true
write_outcome success
echo "Phase 4.0.7 case passed: $case_id $pass_id $platform"
