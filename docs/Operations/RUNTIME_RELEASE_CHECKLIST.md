# Runtime Infrastructure Release Checklist

Use this checklist before deploying Runtime Infrastructure `1.0.0`.

## Database and migrations

- [ ] Apply the accepted Atlas migrations using the migrator identity.
- [ ] Confirm the migration command completed without checksum drift.
- [ ] Confirm `platform.runtime_compatibility` exists and declares schema
      contract `1.0.0` compatible with PostgreSQL Adapter `1.0.0`.
- [ ] Confirm the runtime identities cannot modify Atlas revision history or
      the compatibility record.

## Secrets and TLS

- [ ] Configure the external secret provider for every required database
      password reference.
- [ ] Confirm no password, token, private key, or authenticated URL exists in
      source control or ordinary configuration.
- [ ] Confirm staging and production use `verify-full`.
- [ ] Confirm the root CA and any client certificate/key are readable by the
      process identity and stored outside the repository.
- [ ] Confirm the certificate validates the configured PostgreSQL server name.

## Pool and privilege proof

- [ ] Confirm ingest, read, and retention identities are distinct outside
      local/CI.
- [ ] Confirm connection budgets and pool maxima fit the database limit.
- [ ] Confirm every capability pool can acquire and ping.
- [ ] Confirm positive required privileges and negative forbidden privileges.

## Runtime readiness

- [ ] Start the runtime without bypassing schema compatibility checks.
- [ ] Confirm lifecycle reaches `ready`.
- [ ] Confirm callable liveness is healthy.
- [ ] Confirm callable readiness is healthy after a successful database proof.
- [ ] Confirm structured logs contain only bounded safe fields.
- [ ] Confirm the configured metric sink receives a bounded snapshot.

## Shutdown and rollback readiness

- [ ] Confirm the host stops admission before shutdown.
- [ ] Confirm the drain and forced-shutdown timeouts are appropriate.
- [ ] Confirm metric collection stops before PostgreSQL pools close.
- [ ] Confirm shutdown reaches `stopped` only after resources close.
- [ ] Keep the previous deployable application version available; database
      repair follows the accepted roll-forward migration policy.

The detailed procedure is in
[`RUNTIME_DEPLOYMENT_RUNBOOK.md`](RUNTIME_DEPLOYMENT_RUNBOOK.md).
