# Phase 4.0.1 Repository Service Design Spike

This package is isolated experimental evidence for ADR 0016. It is not the
production Repository Service package and must not be imported by released
engine, persistence, runtime, API, or UI code.

It validates:

- `repository-service-artifact-id/v1` canonical identity bytes;
- sealed exact-byte file-backed materialization;
- independent SHA-256 verification while staging;
- source-root redaction and leak rejection;
- composition of released RIE and Go LIE engines;
- keyed cancellation-aware in-process single-flight;
- committed-but-response-lost publication reconciliation.

## Canonical artifact identity

The preimage is exactly:

```text
ASCII "repository-service-artifact-id/v1" followed by one NUL byte
uint32-be length + UTF-8 repository ID
uint32-be length + UTF-8 scan ID
uint32-be length + UTF-8 artifact name
uint32-be length + UTF-8 artifact version
uint32-be length + UTF-8 stable-ID scheme
```

Every field is non-empty, already trimmed, valid UTF-8, at most 1,024 bytes,
and contains no ASCII control character. The artifact ID is `rsaid1_` followed
by lowercase hexadecimal SHA-256 of the preimage.

Changing the field order, encoding, validation, prefix, or digest requires a
new identity scheme version.

Golden vector:

```text
repository=repo-001
scan=scan-01
artifact=go-semantic-inventory
version=1.0.0
stable-ID scheme=go-semantic-id/v1
ID=rsaid1_3c55ac33a130d92a42bd4f782ad7868d9310b94e3fbb91cc3ba9abb85df8fce8
```

## Isolation

The materializer writes permission-restricted temporary files outside the
analyzed repository, seals them before opening, and removes them explicitly.
No database, SQL, network, listener, queue, background worker, authentication,
transport, UI, or production service implementation is included.
