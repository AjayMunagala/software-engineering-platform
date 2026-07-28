# Repository Service Integration Golden Vectors

## Status

- Contract: Phase 4.0.6 integration `0.1.0`
- Status: Frozen pre-implementation contract
- Accepted with Phase 4.0.6 design review: 2026-07-28
- Breaking changes: require a new versioned scheme and engineering decision

This document is normative. Implementations and conformance fixtures must
match these values byte for byte before PostgreSQL/runtime integration code is
accepted.

## Canonical UUID identifier policy

`ScopeID`, `RepositoryID`, and `ScanID` must match:

```text
^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$
```

Rules:

- lowercase hexadecimal only;
- canonical `8-4-4-4-12` hyphen placement;
- RFC 4122/RFC 9562 variant (`8`, `9`, `a`, or `b`);
- version nibble `1` through `8`;
- no braces, whitespace, prefixes, or alternate encodings;
- the nil UUID and version-zero values are invalid.

Accepted fixtures:

```text
ScopeID:      00000000-0000-4000-8000-000000000001
RepositoryID: 11111111-1111-4111-8111-111111111111
ScanID:       22222222-2222-4222-8222-222222222222
```

Required rejected fixtures:

```text
11111111-1111-4111-8111-11111111111
11111111111141118111111111111111
{11111111-1111-4111-8111-111111111111}
11111111-1111-4111-8111-111111111111<space>
11111111-1111-4111-8111-111111111111 (when any A-F is uppercase)
00000000-0000-0000-0000-000000000000
11111111-1111-0111-8111-111111111111
11111111-1111-4111-7111-111111111111
```

`RequestID` and `PrincipalID` retain the accepted candidate rules and are not
subject to this UUID grammar.

## Public artifact identity fixtures

The already-frozen `repository-service-artifact-id/v1` algorithm is applied to
these canonical service identifiers:

```text
repository ID: 11111111-1111-4111-8111-111111111111
scan ID:       22222222-2222-4222-8222-222222222222
```

Expected public IDs:

```text
discovery-inventory / 1.0.0 / repository-service-artifact-id/v1
rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886

repository-snapshot / 1.0.0 / repository-service-artifact-id/v1
rsaid1_26ddb640b08b957c57512820bc99a18b2bd3b7f168edf5e6b95a77e12c2573b9
```

## Physical artifact UUID algorithm

Scheme: `repository-service-storage-artifact-id/v1`

Preimage:

```text
ASCII "repository-service-storage-artifact-id/v1"
one NUL byte
uint32-be byte length of the public artifact ID
exact UTF-8 public artifact ID bytes
```

Steps:

1. SHA-256 the complete preimage.
2. Copy the first 16 digest bytes.
3. Set byte 6 high nibble to `8` (RFC 9562 version 8).
4. Set byte 8 high bits to `10` (RFC variant).
5. Encode lowercase canonical UUID text.

Golden vectors:

```text
public:
rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886
physical:
628b097f-84cb-87cf-a81c-5627222e948c

public:
rsaid1_26ddb640b08b957c57512820bc99a18b2bd3b7f168edf5e6b95a77e12c2573b9
physical:
30594582-1bb4-8bef-bb5b-2aa43b338a9e
```

The physical value is internal. A public API, error, log, metric, or exported
artifact must never substitute it for the public artifact ID.

## Manifest golden vector

Scheme: `repository-service-manifest/v1`

Common scan fields:

```text
repository ID: 11111111-1111-4111-8111-111111111111
scan ID:       22222222-2222-4222-8222-222222222222
profile name:  repository-go
profile version: 1
profile digest: 63f2a1acd4bc3de83af9859d3308c7b62eae6b7b3e263581fcb0864a12296ba7
source revision: commit:0123456789abcdef
artifact count: 2
```

Artifact 0:

```text
public ID: rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886
name: discovery-inventory
version: 1.0.0
stable-ID scheme: repository-service-artifact-id/v1
codec: canonical-json / 1.0.0 / application/json
exact payload UTF-8: {"artifact":"discovery"}
payload size: 24
payload SHA-256: f06070af1b2f016e3993ed764952436f6affe825c3595146280e5ae9abdae5d1
producer: discovery / 0.1.1
dependencies: 0
```

Artifact 1:

```text
public ID: rsaid1_26ddb640b08b957c57512820bc99a18b2bd3b7f168edf5e6b95a77e12c2573b9
name: repository-snapshot
version: 1.0.0
stable-ID scheme: repository-service-artifact-id/v1
codec: canonical-json / 1.0.0 / application/json
exact payload UTF-8: {"artifact":"snapshot"}
payload size: 23
payload SHA-256: 66757705b582eff45757d33f05e79f367e9585eb6c3a67070650e17e72432e6f
producer: ignore / 0.2.1
dependencies: 1
dependency 0 source public ID: rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886
dependency 0 declared name: discovery-inventory
dependency 0 declared version: 1.0.0
```

Using the canonical encoding specified in the Phase 4.0.6 architecture:

```text
preimage length: 818 bytes
manifest SHA-256: 888b4f65c1a34881d2b762a474a65ada819d6064fd99d30f1958f6872c133598
```

Timestamps, physical UUIDs, source paths, source handles, request IDs,
principals, and persistence implementation data are absent from the preimage.

## Required implementation proof

The production implementation must:

- encode these constants from independent inputs, not hardcode outputs;
- test every accepted/rejected UUID fixture;
- test both physical UUID vectors;
- test public-to-physical-to-record verification;
- test the manifest byte length and digest;
- produce identical values on Windows and Ubuntu;
- demonstrate that one-bit input changes alter the appropriate output;
- retain this document unchanged unless a new versioned scheme is approved.
