# Runtime Deployment Runbook

## Status

- Release: Runtime Infrastructure `1.0.0`
- Release date: 2026-07-27
- Scope: process-local configuration, PostgreSQL runtime, lifecycle, health,
  structured logging, and metrics
- Network listeners and API endpoints: not included

## Deployment boundary

The deployment host owns configuration sources, secret injection, process
signals, log destinations, and the metric sink. Runtime packages own strict
configuration, TLS construction, PostgreSQL pools, schema compatibility,
health, admission, drain, and cleanup. The PostgreSQL adapter does not create
pools or load credentials.

## Preflight

1. Apply the accepted Atlas migrations with the deployment migrator identity.
2. Confirm the `platform.runtime_compatibility` record exists.
3. Provision distinct ingest, read, and retention identities for staging and
   production.
4. Inject database passwords through the configured secret provider.
5. Provide CA and client-certificate files outside the repository when TLS
   client authentication is used.
6. Validate that staging and production use `verify-full` TLS.
7. Validate the complete configuration before starting the application.

The runtime never runs migrations. A missing or incompatible schema fails
startup with a bounded error code.

## Startup order

```text
Load strict configuration
  -> construct structured observability
  -> construct and prove PostgreSQL capability pools
  -> construct health monitor
  -> start metric collection
  -> publish ready state
```

No partially initialized runtime is returned. Any failure closes already-owned
resources in reverse order.

## Runtime operation

- Admit work only while state is `ready`.
- Consume callable liveness/readiness; do not reimplement their rules.
- Export only the bounded structured events and metric snapshots.
- Never add credentials, DSNs, SQL, payloads, source, paths, or raw driver
  errors to logs or metric labels.
- A metric-export failure is isolated from persistence readiness.

## Shutdown order

1. Transition to `draining` and reject new work.
2. Wait up to the configured drain timeout.
3. Cancel remaining work when the timeout or force signal fires.
4. Stop metric collection.
5. Close PostgreSQL pools in reverse construction order.
6. Record the final bounded lifecycle event.
7. Close observability and finish in `stopped` only if resources closed.

Repeated and concurrent shutdown calls receive the same immutable result.

## Health interpretation

- Database failure removes readiness after the configured threshold.
- Database failure does not by itself remove liveness.
- `draining`, `stopping`, and `stopped` are not ready.
- A stale successful database proof removes readiness.
- No HTTP or Kubernetes health endpoint is part of this release.

## Validation commands

From `D:\Project_Ai\backend`:

```powershell
go test ./...
go vet ./...
go test ./... -shuffle=on -count=3
```

Native Linux runtime/race validation:

```powershell
wsl.exe -d Ubuntu-24.04 -- bash /mnt/d/Project_Ai/backend/internal/runtime/tests/bootstrap_linux.sh
```

Disposable PostgreSQL/TLS/lifecycle validation:

```powershell
wsl.exe -d Ubuntu-24.04 -u postgres -- bash -lc `
  'cd /mnt/d/Project_Ai/backend/internal/runtime/postgres/tests && bash validate.sh'
```

Both harnesses remove their temporary resources.

## Recovery

If startup fails, correct configuration, secret, TLS, privilege, or schema
compatibility evidence and start a new process. Do not bypass the compatibility
gate. Migration failures use the accepted roll-forward repair policy. Runtime
packages never modify migration history.
