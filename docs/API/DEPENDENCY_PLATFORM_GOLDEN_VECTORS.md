# Phase 5.0.5 — Independent Vector Freeze

Frozen for review on 2026-09-12 against approved design `1b9f4d6` and accepted
artifact model `2390347`. Implementation is authorized, but production encoding
must wait for this vector commit's review. PostgreSQL/runtime integration testing,
Phase 5.0.6 and release remain unauthorized. DIE-HARDEN-001/002 remain open.

## Authoritative fixtures and independence

[Literal vectors](../../experiments/dependency-platform-vectors/vectors.json)
contain exact UTF-8 payload strings, equivalent base64 bytes, sizes, SHA-256,
full binary preimages, UUIDs, manifests and child operation IDs. Decode base64
for exact bytes; do not hash this pretty-printed JSON file or Markdown fences.
This representation avoids Git CRLF conversion of individual payload files.

[PowerShell calculator](../../experiments/dependency-platform-vectors/calculate.ps1)
constructs literals and binary frames independently of production code.
[Node verifier](../../experiments/dependency-platform-vectors/verify.mjs) independently
reconstructs frames and validates frozen values using Buffer/BigInt/OpenSSL.
Neither imports backend packages or production encoders. The calculator prints
results only; verification never overwrites expected values. No production codec
or integration implementation exists in this commit.

Six cases cover empty, Unicode/omission (reusing accepted base vector bytes),
analyzed empty, string escaping, uint64 limits/map ordering and multiple evidence
records. Wire-only fixtures intentionally use synthetic IDs/counters and are not
claims of valid normalized graphs. The escaping diagnostic may fail semantic safe
text validation; it tests the JSON string alphabet, not permission to publish NUL.
Frozen graph IDs/SCC/cycle/cursor vectors are not modified or newly derived.
The analyzed-empty envelope reuses the accepted empty input digest.

All six envelopes use the same synthetic scope/repository/scan IDs, so their UUID
is deliberately equal; differing contents produce different manifests/child IDs.
They are alternate test cases, not six artifacts to publish into one scan.
Five identity mutations change scope, repository, scan, artifact name and version.
Unsupported names/versions are frame-only cases, not newly supported contracts.

## Summary

Profile SHA-256: `ac9133786e72ab4f3fe4e75a8b930aeae09ec4fde04cd08f26493956a18b2484`.

Baseline artifact UUID: `70869f84-b92b-858b-b2df-e226e6d78742`.

| Fixture | Bytes including LF | Payload SHA-256 |
|---|---:|---|
| empty | 614 | `2341cdbfd54d1819086af76f07ebb9bf8c4c55dbd6a7e73b395a3114db122088` |
| unicode_omission | 665 | `007dcdc3ec75703670ba29d3d60433f3d7db43f0a72e1f9967358a575c520242` |
| analyzed_empty | 1556 | `928b24720adc79021a45d231e48580a76ac4afb0b3c82557eb9152ff492c3b2a` |
| escaping | 697 | `84d50bb0eb102ae56a395a090cd01e9095081cbae09535b21e2603d420b278b7` |
| uint64_wire_only | 707 | `54c6f833cd1f5b2a51f7f3120fc3f21b405a54ef9e9fe7682a03b4019e3495c9` |
| multi_evidence_wire_only | 1216 | `ba4a73b2201cdf02f22677ba549cff299f3aba3951ffb45ae188815259c1ed8c` |

## Complete payload field-order table

Transcribed from the accepted Go struct JSON tags; each model's rows are in wire
order. `omitempty` omits its zero value. Arrays retain accepted order and public
View empty normalization; maps are lexical-key ordered. GraphKind/NodeKind and
other named string types encode as strings, integer types as exact JSON integers.
No new artifact schema or private-value decoder is introduced.

| Model | Go field | Type | JSON tag |
|---|---|---|---|
| DependencyInventoryView | Artifact | `ArtifactMetadata` | `artifact` |
| DependencyInventoryView | SourceArtifacts | `[]ArtifactReference` | `source_artifacts` |
| DependencyInventoryView | Nodes | `[]DependencyNode` | `nodes` |
| DependencyInventoryView | Containment | `[]ContainmentEdge` | `containment` |
| DependencyInventoryView | Dependencies | `[]DependencyEdge` | `dependencies` |
| DependencyInventoryView | StrongComponents | `[]StrongComponent` | `strong_components` |
| DependencyInventoryView | Cycles | `[]DependencyCycle` | `cycles` |
| DependencyInventoryView | Diagnostics | `[]Diagnostic` | `diagnostics` |
| DependencyInventoryView | Statistics | `DependencyStatistics` | `statistics` |
| DependencyInventoryView | Analysis | `*AnalysisMetadata` | `analysis,omitempty` |
| ArtifactMetadata | Name | `string` | `name` |
| ArtifactMetadata | Version | `string` | `version` |
| ArtifactMetadata | EngineName | `string` | `engine_name` |
| ArtifactMetadata | EngineVersion | `string` | `engine_version` |
| ArtifactMetadata | NodeIDSchemeVersion | `string` | `node_id_scheme_version` |
| ArtifactMetadata | EdgeIDSchemeVersion | `string` | `edge_id_scheme_version` |
| ArtifactMetadata | ContainmentIDSchemeVersion | `string` | `containment_id_scheme_version` |
| ArtifactReference | Name | `string` | `name` |
| ArtifactReference | Version | `string` | `version` |
| ArtifactReference | Digest | `string` | `digest,omitempty` |
| SourceIdentity | ArtifactName | `string` | `artifact_name` |
| SourceIdentity | ArtifactVersion | `string` | `artifact_version` |
| SourceIdentity | SourceID | `string` | `source_id` |
| DependencyEvidence | Source | `SourceIdentity` | `source` |
| DependencyEvidence | File | `string` | `file,omitempty` |
| DependencyEvidence | StartLine | `int` | `start_line,omitempty` |
| DependencyEvidence | StartColumn | `int` | `start_column,omitempty` |
| DependencyEvidence | Rule | `string` | `rule` |
| DependencyEvidence | Value | `string` | `value,omitempty` |
| DependencyNode | ID | `string` | `id` |
| DependencyNode | Kind | `NodeKind` | `kind` |
| DependencyNode | Language | `string` | `language,omitempty` |
| DependencyNode | Name | `string` | `name` |
| DependencyNode | QualifiedName | `string` | `qualified_name` |
| DependencyNode | RepositoryPath | `string` | `repository_path,omitempty` |
| DependencyNode | Resolution | `ResolutionState` | `resolution` |
| DependencyNode | SourceIdentity | `SourceIdentity` | `source_identity` |
| DependencyNode | Evidence | `[]DependencyEvidence` | `evidence` |
| ContainmentEdge | ID | `string` | `id` |
| ContainmentEdge | Kind | `ContainmentKind` | `kind` |
| ContainmentEdge | ParentID | `string` | `parent_id` |
| ContainmentEdge | ChildID | `string` | `child_id` |
| ContainmentEdge | Evidence | `[]DependencyEvidence` | `evidence` |
| ContainmentEdge | OmittedEvidence | `uint64` | `omitted_evidence` |
| DependencyEdge | ID | `string` | `id` |
| DependencyEdge | Graph | `GraphKind` | `graph` |
| DependencyEdge | Kind | `DependencyKind` | `kind` |
| DependencyEdge | FromNodeID | `string` | `from_node_id` |
| DependencyEdge | ToNodeID | `string` | `to_node_id` |
| DependencyEdge | Resolution | `ResolutionState` | `resolution` |
| DependencyEdge | Occurrences | `uint64` | `occurrences` |
| DependencyEdge | Evidence | `[]DependencyEvidence` | `evidence` |
| DependencyEdge | OmittedEvidence | `uint64` | `omitted_evidence` |
| StrongComponent | ID | `string` | `id` |
| StrongComponent | Graph | `GraphKind` | `graph` |
| StrongComponent | NodeIDs | `[]string` | `node_ids` |
| StrongComponent | Cyclic | `bool` | `cyclic` |
| StrongComponent | SelfLoop | `bool` | `self_loop` |
| DependencyCycle | ID | `string` | `id` |
| DependencyCycle | Graph | `GraphKind` | `graph` |
| DependencyCycle | ComponentID | `string` | `component_id` |
| DependencyCycle | NodeIDs | `[]string` | `node_ids` |
| DependencyCycle | Classification | `string` | `classification` |
| DependencyCycle | Rule | `string` | `rule` |
| Diagnostic | Code | `string` | `code` |
| Diagnostic | Graph | `GraphKind` | `graph,omitempty` |
| Diagnostic | File | `string` | `file,omitempty` |
| Diagnostic | StartLine | `int` | `start_line,omitempty` |
| Diagnostic | StartColumn | `int` | `start_column,omitempty` |
| Diagnostic | StableID | `string` | `stable_id,omitempty` |
| Diagnostic | Message | `string` | `message` |
| DependencyStatistics | NodesByKind | `map[string]uint64` | `nodes_by_kind` |
| DependencyStatistics | EdgesByGraph | `map[string]uint64` | `edges_by_graph` |
| DependencyStatistics | EdgesByResolution | `map[string]uint64` | `edges_by_resolution` |
| DependencyStatistics | Diagnostics | `uint64` | `diagnostics` |
| DependencyStatistics | OmittedDiagnostics | `uint64` | `omitted_diagnostics` |
| DependencyStatistics | OmittedNodes | `uint64` | `omitted_nodes` |
| DependencyStatistics | OmittedContainment | `uint64` | `omitted_containment` |
| DependencyStatistics | OmittedEdges | `uint64` | `omitted_edges` |
| DependencyStatistics | OmittedEvidence | `uint64` | `omitted_evidence` |
| AnalysisMetadata | EngineName | `string` | `engine_name` |
| AnalysisMetadata | EngineVersion | `string` | `engine_version` |
| AnalysisMetadata | InputDigest | `string` | `input_digest` |
| AnalysisMetadata | DigestScheme | `string` | `digest_scheme` |
| AnalysisMetadata | SCCIDScheme | `string` | `scc_id_scheme` |
| AnalysisMetadata | CycleIDScheme | `string` | `cycle_id_scheme` |
| AnalysisMetadata | ProjectionPolicy | `string` | `projection_policy` |
| AnalysisMetadata | Graphs | `[]GraphAnalysis` | `graphs` |
| GraphAnalysis | Graph | `GraphKind` | `graph` |
| GraphAnalysis | EligibleNodes | `uint64` | `eligible_nodes` |
| GraphAnalysis | EligibleEdges | `uint64` | `eligible_edges` |
| GraphAnalysis | Components | `uint64` | `components` |
| GraphAnalysis | CyclicComponents | `uint64` | `cyclic_components` |
| GraphAnalysis | BoundaryEdges | `uint64` | `boundary_edges` |
| GraphAnalysis | TopologyLimited | `bool` | `topology_limited` |
| GraphAnalysis | ExplanationLimited | `bool` | `explanation_limited` |
| GraphAnalysis | ReasonCodes | `[]string` | `reason_codes` |

## Verification record

Executed on Windows, 2026-09-12:

- `pwsh -NoProfile -File experiments/dependency-platform-vectors/calculate.ps1`:
  PASS; PowerShell 7.6.5 / .NET 10.0.11. Repeated output equals frozen data.
- `node experiments/dependency-platform-vectors/verify.mjs`: PASS; Node v24.14.1 /
  OpenSSL 3.5.5. Six payload/profile/UUID/manifest cases, 30 child IDs, five identity
  mutations, every manifest-byte mutation, final-LF sensitivity and two existing
  digest anchors checked.
- Documentation/whitespace and no-backend-change audit required before commit.

No backend regression, Go codec conformance, Ubuntu run, race/fuzz, database/runtime
integration or performance validation is claimed. These are vector checks only.
The former design's wording 'UUID request IDs' does not restrict opaque parent
request IDs: scope/repository/scan are UUIDs; parent is S(exact bounded opaque ID),
and generated child IDs are lowercase hex hashes. No framing values are changed.

## Governance

ADR 0023's design is approved, not yet implementation-accepted. Stop after this
commit and request vector review. Expected values must not be regenerated from
future production code to make a failing test pass; corrections require review.
