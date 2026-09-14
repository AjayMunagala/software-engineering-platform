# Disposable dependency integration validation

Run `validate_disposable.sh linux` or `validate_disposable.sh windows` from WSL
as the `postgres` OS user. Requires PostgreSQL 18.4, Atlas Community 1.2.3,
the existing pinned `/opt/aegis-go-1.26.2/bin/go`, and native PowerShell/Go for
Windows mode. Do not point this test at an existing database.

The harness creates a random private directory under `/tmp/aegis-die505-*`, a
loopback-only cluster on a high ephemeral port, and fresh checksum-verified
migrations. It grants a disposable combined-capability CI login, runs a real
runtime/publication test, immediately stops **only its own cluster**, restarts,
verifies in a fresh process, backs up/restores into a second disposable database,
then checks archive/purge/GC. Its exit trap stops the cluster and removes the
validated harness-owned directory. Trust authentication is disposable-local only;
this is not production TLS qualification. Existing personal databases are untouched.

The Go test additionally checks persisted publication certificates through a
test-only observer. That connection is never passed to runtime or production
integration code. Native Windows and Linux compare exact bytes against accepted
View serialization plus LF for both empty and nonempty Unicode inventories.

This is a checkpoint harness, not the entire Phase 5.0.5 failure matrix. See the
real-capability report for unexecuted cases and governance status.
