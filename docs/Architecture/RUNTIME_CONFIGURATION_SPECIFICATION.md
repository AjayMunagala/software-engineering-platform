# Runtime Configuration Specification

## Status

- Phase: 3.5 design
- Status: Accepted on 2026-07-26
- Configuration owner: `backend/internal/runtime/config`
- Runtime mutation: prohibited in v1

## Objective

Produce one validated, immutable, redacted runtime configuration from explicit
sources before any logger sink, secret, certificate, database connection, or
adapter is created.

## Source Precedence

For ordinary non-secret settings, later sources override earlier sources:

```text
compiled profile defaults
        < optional strict JSON configuration file
        < environment variables
        < allowlisted command-line overrides
```

Every resolved field records its source category for diagnostics without
recording its value when sensitive. Duplicate values at the same precedence are
rejected rather than resolved by iteration order.

The configuration file is UTF-8 JSON decoded with unknown-field rejection.
JSON is chosen for the first runtime because Go supports strict decoding
without another parser dependency. A different format requires a reviewed
decision; silently accepting multiple formats is prohibited.

## Secret Precedence

Secrets do not use ordinary merge rules. One configured `SecretProvider`
resolves all enabled database-role passwords and the optional client-key
passphrase at startup.
Initial providers may include:

- environment-variable injection;
- mounted secret file with restricted operating-system permissions;
- CI secret injection exposed through either of the above;
- a future secret-manager adapter behind the same runtime-only interface.

If more than one source supplies the same secret, startup fails as ambiguous.
Secret values are returned in a short-lived internal structure, never in a
public config view, JSON output, error, metric, or log field.

## Command-Line Boundary

Command-line overrides are restricted to safe operational fields:

- profile;
- configuration-file path;
- log level/format;
- database host, port, and name;
- bounded pool and timeout settings;
- TLS mode and certificate file paths.

Passwords, tokens, private-key passphrases, or full authenticated connection
URLs are forbidden on the command line because process listings and shell
history may expose them.

## Configuration Model

Conceptual immutable model:

```text
RuntimeConfig
  Profile
  Startup
    StartupTimeout
    DrainTimeout
    ForcedShutdownTimeout
  Logging
    Level
    Format
    ServiceName
  Database
    Host
    Port
    Name
    User
    ApplicationName
    ConnectTimeout
    ConnectionBudget
    Pools
      Ingest { MaxConns, MinIdleConns, SecretReference }
      Read { MaxConns, MinIdleConns, SecretReference }
      Retention { MaxConns, MinIdleConns, SecretReference }
      MaxConnLifetime
      MaxConnLifetimeJitter
      MaxConnIdleTime
      HealthCheckPeriod
      PingTimeout
    TLS
      Mode
      RootCAPath
      ClientCertPath
      ClientKeyPath
      ServerName
    SecretReference
  Health
    CheckTimeout
    CheckInterval
    FailureThreshold
  Observability
    MetricsEnabled
    CollectionInterval
```

`MinConns` is fixed at zero and is not user-configurable in v1.

## Field Rules

| Field | Rule |
|---|---|
| profile | exactly `local`, `ci`, `staging`, or `production` |
| host | DNS name, IP literal, absolute Unix-socket directory, or loopback |
| port | 1–65535; ignored for Unix socket |
| database/user | safe non-empty identifiers; never logged together as a DSN |
| application name | fixed safe prefix plus deployment identifier, max 64 octets |
| connection budget | local/CI fixed default 4; staging/production explicit 3–64 |
| per-pool maximum | 1–budget; sum cannot exceed budget |
| per-pool minimum idle | 0–that pool maximum; explicit in staging/production |
| durations | positive, bounded, no numeric unit inference |
| TLS paths | absolute, clean, existing regular files; no symlinks in production |
| server name | required for production `verify-full` unless host is a DNS name |
| log level | `debug`, `info`, `warn`, or `error` |
| log format | `text` locally; `json` elsewhere unless explicitly stricter |

All duration text requires an explicit unit such as `5s` or `1m`. NaN,
infinite, negative, zero where prohibited, and overflow values are rejected.

Initial defaults:

| Setting | Default |
|---|---:|
| startup timeout | 30 seconds |
| connection timeout | 10 seconds per required pool, inside startup deadline |
| drain timeout | 30 seconds |
| forced-shutdown timeout | 5 seconds |
| health check timeout | 2 seconds per required pool |
| health interval | 15 seconds (5 seconds in CI) |
| readiness failure threshold | 3 after startup; no startup grace |
| metrics collection interval | 15 seconds |

## Environment Variable Names

Names are stable; values are intentionally absent from repository examples.

```text
AEGIS_PROFILE
AEGIS_CONFIG_FILE
AEGIS_LOG_LEVEL
AEGIS_LOG_FORMAT
AEGIS_DATABASE_HOST
AEGIS_DATABASE_PORT
AEGIS_DATABASE_NAME
AEGIS_DATABASE_USER
AEGIS_DATABASE_PASSWORD
AEGIS_DATABASE_CONNECTION_BUDGET
AEGIS_DATABASE_INGEST_MAX_CONNS
AEGIS_DATABASE_INGEST_MIN_IDLE_CONNS
AEGIS_DATABASE_INGEST_PASSWORD
AEGIS_DATABASE_READ_MAX_CONNS
AEGIS_DATABASE_READ_MIN_IDLE_CONNS
AEGIS_DATABASE_READ_PASSWORD
AEGIS_DATABASE_RETENTION_MAX_CONNS
AEGIS_DATABASE_RETENTION_MIN_IDLE_CONNS
AEGIS_DATABASE_RETENTION_PASSWORD
AEGIS_DATABASE_CONNECT_TIMEOUT
AEGIS_DATABASE_TLS_MODE
AEGIS_DATABASE_TLS_ROOT_CA_FILE
AEGIS_DATABASE_TLS_CLIENT_CERT_FILE
AEGIS_DATABASE_TLS_CLIENT_KEY_FILE
AEGIS_DATABASE_TLS_SERVER_NAME
AEGIS_STARTUP_TIMEOUT
AEGIS_DRAIN_TIMEOUT
```

`AEGIS_DATABASE_PASSWORD` is a local/CI combined-role convenience only.
Staging/production require separate ingest, read, and retention secrets. Every
password is consumed only by the environment secret provider and is never
included in `RuntimeConfig.String`, structured config output, or diagnostics. A
single `DATABASE_URL` variable is not part of the v1 contract because it mixes
secrets and non-secrets and is difficult to redact safely.

## TLS Policy

Runtime policy exposes two modes:

- `disabled`: only `local`/`ci`, and only loopback or Unix sockets;
- `verify-full`: required for `staging`/`production`, encrypts traffic, validates
  the certificate chain, and verifies the server hostname.

The libpq modes `allow`, `prefer`, and bare `require` are rejected. They do not
provide the production identity guarantee. `verify-ca` is also excluded from
the initial contract; a documented exception requires a security decision.

Custom root CAs are supported. Client authentication is optional but the client
certificate and key must be supplied together. On Unix, private-key permissions
must not grant group/world access unless the explicitly approved root-owned
group-read pattern is used. Certificate material is loaded once at startup;
rotation requires a controlled restart in v1.

## Profile Rules

| Profile | Sources | TLS | Pool | Logging | Strictness |
|---|---|---|---|---|---|
| local | defaults, optional file, env, CLI | disabled only to loopback/socket; verify-full allowed | combined disposable pool, budget 4 | text/info default | unknown keys fail |
| ci | file/env, safe CLI | disabled only to disposable loopback/socket | combined disposable pool, budget 4 | JSON/info | no developer fallback |
| staging | mounted non-secret file, env/secret manager | verify-full required | separate capability pools; explicit budgets | JSON/info | all production validation except scale |
| production | immutable mounted file, secret manager/injection | verify-full required | separate capability pools; explicit budgets | JSON/info or stricter | no insecure fallback |

Staging is intentionally production-like. A staging-only configuration that
would be rejected in production must be explicitly identified by the validator.

## Validation Order

1. Select profile from one unambiguous source.
2. Apply profile defaults.
3. Decode optional strict JSON file.
4. Apply known environment keys.
5. Apply allowlisted safe command-line overrides.
6. Reject unknown, duplicate, unsupported, and contradictory fields.
7. Validate cross-field profile/TLS/pool rules.
8. Freeze the ordinary configuration.
9. Resolve secrets through exactly one provider.
10. Build a separate private connection input and immediately redact temporary
    diagnostics.

No filesystem certificate or secret read occurs before paths and profile rules
validate.

## Immutability and Views

Implementation models use private fields, validated constructors, and detached
getters. Collections and byte slices are defensively copied. After startup,
neither environment changes nor file changes alter active behavior.

A redacted view may expose source category and safe value for diagnostics. It
must replace all secret values with the constant marker `[REDACTED]`; it must
not expose password length, digest, prefix, suffix, or presence beyond a
boolean `configured` state.

## Unsupported Configuration

Startup fails for:

- unknown file keys or environment variables with the `AEGIS_` prefix;
- deprecated aliases after their published removal window;
- multiple config files or secret providers;
- TLS disabled outside local/CI loopback;
- certificate/key mismatch;
- pool minimum above maximum;
- production implicit connection limits;
- authenticated URLs;
- any attempt to configure migration execution or Persistence Port behavior.

Failures use stable safe codes and field names, never supplied values.

## Evolution

New optional fields may be additive only when their zero/default behavior is
safe and profile-specific behavior remains explicit. Changing precedence,
secret boundaries, TLS guarantees, or interpreting an old value differently
requires an ADR and compatibility plan.
