# Runtime Configuration

## Status

- Phase: 3.5.1 — Accepted on 2026-07-26
- Contract: candidate `0.1.0`
- Scope: accepted configuration and secret-boundary baseline
- PostgreSQL pools, TLS construction, lifecycle, health, and observability: not
  implemented here

## Purpose

Load deterministic runtime settings from approved sources, validate the full
profile contract, freeze a secret-free immutable model, select exactly one
secret provider, and prove that required secret references resolve.

## Source Order

Later ordinary sources override earlier sources:

```text
profile defaults
  < strict JSON file
  < AEGIS_ environment variables
  < allowlisted non-secret command-line flags
```

Unknown `AEGIS_` variables, unknown JSON fields, duplicate keys, duplicate
flags, authenticated URLs, and command-line secret flags fail closed.

The loader receives environment and argument slices explicitly through
`LoadRequest`. It does not read process-global state, connect to PostgreSQL,
construct pools, execute migrations, start listeners, or mutate engines.

## Secret Boundary

`RuntimeConfig` never contains passwords. `LoadedConfiguration` retains only a
`SecretProvider` capability. `ResolveSecret` returns detached bytes that the
caller must overwrite immediately after use. `SafeView` and `String` emit only
the constant `[REDACTED]` marker for required references.

Environment injection is the default provider when no explicit provider is
supplied. Supplying both environment secrets and an explicit provider is
rejected as ambiguous.

Local and CI use one combined disposable database identity. Staging and
production require distinct ingest, read, and retention users and secrets.

## Package Layout

```text
config/
  interface.go
  implementation.go
  config.go
  model.go
  errors.go
  README.md
  implementation_test.go
  implementation_benchmark_test.go
```

## Guarantees

- strict, deterministic source precedence;
- private immutable model fields and detached collection getters;
- stable safe error codes and field names;
- no supplied values in error strings;
- explicit duration units and bounded numeric values;
- production `verify-full` policy;
- no secret values in configuration JSON, string output, or diagnostics;
- no database, persistence, engine, API, or UI dependency.

## Validation

Run from `backend`:

```powershell
go test ./internal/runtime/config
go test -shuffle=on -count=3 ./internal/runtime/config
go test -race ./internal/runtime/config
go test -cover ./internal/runtime/config
go test -run '^$' -bench . -benchmem ./internal/runtime/config
```

Full backend regression remains mandatory before milestone acceptance.
