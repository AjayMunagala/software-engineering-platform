# Dependency Platform Codec and Publication Specification

Design proposal for Phase 5.0.5, 2026-09-11. No codec, vectors, or implementation
are created by this document. See [architecture](DEPENDENCY_PLATFORM_INTEGRATION.md)
and [ADR 0023](../Decisions/0023-dependency-platform-integration.md).

## Version boundaries

| Identity | Proposed value / treatment |
|---|---|
| Artifact | Existing `dependency-inventory` / `0.1.0` |
| Codec | `dependency-canonical-json` / `0.1.0`, media type `application/json` |
| Integration API/producer | `dependency-platform-integration` / `0.1.0` |
| Publication profile | `dependency-publication` / `0.1.0` |
| Envelope stable-ID scheme | `dependency-platform-artifact-id/v1` |
| Manifest scheme | `dependency-platform-manifest/v1` |
| Existing graph IDs and input/cursor fingerprints | Unchanged; reuse accepted vectors |
| Persistence / PostgreSQL / Runtime | Frozen 1.0.0 contracts; no schema migration |

The envelope producer identifies the materialization/publication component;
engine producer metadata inside the artifact remains unchanged. Codec identity
does not assert release readiness. Unknown artifact/codec/profile/scheme versions
fail closed. No fallback to a similarly named codec, lossy conversion, or in-place
rewrite. Future codec versions produce new publications; old exact bytes remain.

## Canonical payload

Encode the accepted immutable inventory's published values without normalizing
facts, running analysis, changing diagnostics, or inventing provenance.
For small validation fixtures the independent compatibility oracle is the
accepted Go JSON encoding of `inventory.View()` followed by one LF. This oracle
is not the proposed large-payload implementation.

Top-level order is fixed:
`artifact`, `source_artifacts`, `nodes`, `containment`, `dependencies`,
`strong_components`, `cycles`, `diagnostics`, `statistics`, and optional `analysis`.
Nested field order, JSON names, omission rules, and enum encodings are those of
the accepted concrete types in `backend/die/model.go` and `analysis_model.go` at
`2390347512149889f28d86c21a0e1682c991a0a9`, not earlier illustrative design structs.
An implementation milestone must capture a reviewable exhaustive schema/field
table and independent fixture bytes before production encoder code.

Rules:

- UTF-8, no BOM, no indentation or insignificant spaces, exactly one terminal LF.
- Use accepted Go `encoding/json` string escaping, including HTML escaping of
  `<`, `>`, `&` and escaping U+2028/U+2029. Do not introduce Unicode normalization.
- Integers use exact base-10 JSON numbers; never convert counts through float64.
- Preserve accepted collection order; maps use lexical key order. Do not sort
  facts differently or deduplicate them during serialization.
- Empty collections/evidence follow the immutable public View contract (`[]`
  where that contract normalizes empty slices). Omit absent `analysis`, not null.
  Preserve existing optional field rules; absent digest remains absent.
- Reject invalid UTF-8 rather than silently replacing it, zero inventories,
  unsupported schemes, or invalid publication metadata before successful sealing.
- SHA-256 covers every payload byte including final LF; size is that exact byte
  count. Hash after every successful write, handle short writes/errors correctly.
- The inventory is not mutated. Do not append integration fields to its JSON.

The streaming writer emits punctuation and individual records with bounded copy
buffers. It may use public defensive-copy accessors one collection at a time;
these copies and the largest record are not constant memory. It must not marshal
the whole inventory and then stream the resulting artifact-sized byte slice.
No new private-field bypass or change to accepted core/accessor behavior.

## Distinct digests

| Value | What it proves |
|---|---|
| Payload SHA-256 | Exact complete stored bytes, including optional analysis |
| `dependency-analysis-input/v1` | Accepted base graph consistency fingerprint; excludes derived SCC/cycle/envelope state as already specified |
| Node/edge/SCC/cycle IDs | Accepted structural identity, not storage integrity |
| Upstream file/manifest digest | Its original proof domain only |
| Manifest digest | Exact publication description under the scheme below |

Never substitute one for another. A canonical payload digest is not a common-scan
signature, source-freshness proof, authorization token, or cursor authentication.

## Canonical binary frames

New schemes use a single frame definition: `S(text)` is uint64 big-endian UTF-8
byte length followed by exact UTF-8 bytes; `U(n)` is uint64 big-endian; `H(d)` is
32 raw SHA-256 bytes. All strings are validated, with no implicit trimming or
case folding. UUID request IDs are canonical lowercase RFC-format strings.
Every preimage starts with `S(domain)`; no NUL suffix. This does not replace
the different framing of any earlier frozen scheme.

Profile digest preimage:

`S("dependency-platform-profile/v1")`, `S("dependency-publication")`,
`S("0.1.0")`, `S("dependency-inventory")`, `S("0.1.0")`,
`S("dependency-canonical-json")`, `S("0.1.0")`, `S("application/json")`,
`S("dependency-platform-artifact-id/v1")`,
`S("dependency-platform-integration")`, `S("0.1.0")`,
`S("dependency-platform-manifest/v1")`, `S("logical-source-references-only")`,
`U(1)` (artifact count), `U(0)` (dependency count), `U(0)` (MakeCurrent).
Operational lower limits are not part of profile identity.

Artifact UUID preimage:

`S("dependency-platform-artifact-id/v1")`, `S(scopeID)`, `S(repositoryID)`,
`S(scanID)`, `S("dependency-inventory")`, `S("0.1.0")`.
Hash SHA-256, take first 16 bytes, set UUID version nibble to 8 and RFC variant
bits to binary 10, format lowercase UUID. Collision/inconsistent metadata is an
integrity failure, never an overwrite. This is a new namespace, not a reuse of
Repository Service's public textual IDs or internal UUID mapping.

Manifest preimage:

`S("dependency-platform-manifest/v1")`, `S(scopeID)`, `S(repositoryID)`,
`S(scanID)`, `H(profileDigest)`, `S(sourceRevision)`, `U(1)`,
`S(artifactUUID)`, `S(artifactName)`, `S(artifactVersion)`, `S(stableIDScheme)`,
`S(codecName)`, `S(codecVersion)`, `S(mediaType)`, `S(producerName)`,
`S(producerVersion)`, `H(payloadDigest)`, `U(payloadSize)`,
`U(0)` (dependencies), `U(0)` (projections), `U(0)` (MakeCurrent).
No paths, clocks, principal IDs, arbitrary maps, or request IDs in the manifest.
Different scope/repository/scan IDs deliberately produce different manifests
even if the inventory payload bytes are identical.

Child operation request ID is lowercase hex SHA-256 over
`S("dependency-platform-request/v1")`, `S(parentRequestID)`, `S(operation)`,
`S(scopeID)`, `S(repositoryID)`, `S(scanID)`, `H(manifestDigest)`.
Operation is one of `begin`, `stage`, `publish`, `fail`, `cancel`. Audit identity
comes from the scoped caller, not serialized source data. Same parent request
with changed content is rejected by the integration before replay is accepted;
existing scan/profile/revision/manifest comparisons are authoritative.

## Read/export/recovery contract

Get resolves a scoped succeeded scan and its unique artifact; checks exact
profile, envelope identity, codec, producer and limits, and reconstructs the
manifest using the frozen scheme. The read port does not expose the stored
manifest certificate. Reconciliation compares reconstructed metadata with the
sealed expected publication; successful Publish receipts also have their scheme
and digest checked. Get alone proves neither payload integrity nor equality with
the original requested manifest if the caller has not supplied that expectation.

Export acquires admission and the shared instance spool budget, then first
verifies into an owned spool via ArtifactReader.ExportPayload,
recomputes hash and size and compares the port receipt and envelope, then copies
verified bytes to the caller. Do not expose corrupt DB bytes before verification.
A caller writer failure can still leave a partial valid prefix: no generic writer
rollback is promised. A successful receipt is returned only after full delivery.
No decoder/typed rehydration is proposed; retrieval is exact bytes plus metadata.
After restart, query algorithms require caller-held/recomputed typed artifacts,
not unmarshalling into private inventory fields.

Recovery verifies published metadata and streams the entire payload for integrity.
Do not mark a running scan successful, reconstruct missing chunks, heal digests,
repair schema, or change immutable payloads automatically. Corruption is reported
for operator action. Existing backup/restore and retention ownership is unchanged.

## Mandatory vector gate

After design approval AND explicit implementation authorization, independently
calculate and commit literal payload bytes, payload SHA/size, profile digest,
artifact UUID, manifest digest, and child request IDs before implementing their
production encoders. Include empty/base/analyzed, Unicode/escaping, omissions,
multi-evidence, count boundaries and identity mutation cases. Use independent
construction, not expected values regenerated from the code under test.
Reuse existing graph/digest/cursor vectors without regeneration. Vectors and
performance results are not claimed to exist in this design-only package.
