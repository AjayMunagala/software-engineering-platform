# Phase 5.0.4 Independent Digest and Cursor Vectors

Frozen before production implementation, 2026-09-09. Implementation was explicitly
authorized after design approval at `140457d`. These values were calculated using
PowerShell/.NET UTF8, MemoryStream, SHA256.HashData, and Convert.ToBase64String.
No production Go encoder, cursor code, or graph algorithm was used.
Existing SCC/cycle vectors in `DEPENDENCY_INTELLIGENCE_GOLDEN_VECTORS.md` remain unchanged.

## Digest encoding

Write the 28-byte UTF-8 domain `dependency-analysis-input/v1`, preceded by its
unsigned uint64 big-endian byte length (`000000000000001c`). Append the exact
compact base JSON below and ONE LF byte (0a). SHA-256 the whole stream. Text form
is `dependency-analysis-input/v1:sha256:` followed by lowercase digest hex.

Empty base JSON (613 UTF-8 bytes, 614 with LF):

```json
{"artifact":{"name":"dependency-inventory","version":"0.1.0","engine_name":"dependency-intelligence-core","engine_version":"0.1.0","node_id_scheme_version":"dependency-node-id/v1","edge_id_scheme_version":"dependency-edge-id/v1","containment_id_scheme_version":"dependency-containment-id/v1"},"source_artifacts":[],"nodes":[],"containment":[],"dependencies":[],"strong_components":[],"cycles":[],"diagnostics":[],"statistics":{"nodes_by_kind":{},"edges_by_graph":{},"edges_by_resolution":{},"diagnostics":0,"omitted_diagnostics":0,"omitted_nodes":0,"omitted_containment":0,"omitted_edges":0,"omitted_evidence":0}}
```

Expected empty digest:

```text
dependency-analysis-input/v1:sha256:27f88fcd44316dd1e7b7b93288fc9a98cb212d1f8cb82c315c537732c66060b2
```

Unicode/escaping/omission vector: replace the exact `"source_artifacts":[]` fragment
above with `"source_artifacts":[{"name":"fixture/π\u003c\u0026","version":"1.0.0"}]`
and `"omitted_edges":0` with `"omitted_edges":1`. All other bytes are identical.
The source name is logically `fixture/π<&`; π remains UTF-8, while < and & have
the Go JSON HTML-safe escapes shown. Result: 665 bytes including LF.

```text
dependency-analysis-input/v1:sha256:937f6b3fcad90715eae637ec428946ef628aaf7db5902b74d1f2730a76e951c1
```

These are codec vectors; omitted-edge statistics need not arise from an empty
successful scan. They do not establish source freshness or common-scan proof.

## Cursor vectors

Compact JSON array, no LF, encoded UTF-8 -> standard base64 -> replace + with -, /
with _, remove = padding. Root A means `dependency-node-id/v1:sha256:` plus exactly
64 lowercase a characters; B is the same prefix plus 64 lowercase b characters.
These syntactic IDs test encoding, not membership in the empty inventory. Public
cursor consumption must independently reject absent roots/neighbors.

Vector 1 fields: domain `dependency-node-page/v1`, empty digest above, graph
`package`, direction `dependencies`, root A, numeric page size 100, last ID B.

```text
WyJkZXBlbmRlbmN5LW5vZGUtcGFnZS92MSIsImRlcGVuZGVuY3ktYW5hbHlzaXMtaW5wdXQvdjE6c2hhMjU2OjI3Zjg4ZmNkNDQzMTZkZDFlN2I3YjkzMjg4ZmM5YTk4Y2IyMTJkMWY4Y2I4MmMzMTVjNTM3NzMyYzY2MDYwYjIiLCJwYWNrYWdlIiwiZGVwZW5kZW5jaWVzIiwiZGVwZW5kZW5jeS1ub2RlLWlkL3YxOnNoYTI1NjphYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhIiwxMDAsImRlcGVuZGVuY3ktbm9kZS1pZC92MTpzaGEyNTY6YmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYiJd
```

Vector 2: same fields, replacing only the fingerprint with the Unicode/omission digest.

```text
WyJkZXBlbmRlbmN5LW5vZGUtcGFnZS92MSIsImRlcGVuZGVuY3ktYW5hbHlzaXMtaW5wdXQvdjE6c2hhMjU2OjkzN2Y2YjNmY2FkOTA3MTVlYWU2MzdlYzQyODk0NmVmNjI4YWFmN2RiNTkwMmI3NGQxZjI3MzBhNzZlOTUxYzEiLCJwYWNrYWdlIiwiZGVwZW5kZW5jaWVzIiwiZGVwZW5kZW5jeS1ub2RlLWlkL3YxOnNoYTI1NjphYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhIiwxMDAsImRlcGVuZGVuY3ktbm9kZS1pZC92MTpzaGEyNTY6YmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYiJd
```

Vector 3: empty digest, graph `file`, direction `dependents`, root B, page size 1,
last ID A. Domain unchanged.

```text
WyJkZXBlbmRlbmN5LW5vZGUtcGFnZS92MSIsImRlcGVuZGVuY3ktYW5hbHlzaXMtaW5wdXQvdjE6c2hhMjU2OjI3Zjg4ZmNkNDQzMTZkZDFlN2I3YjkzMjg4ZmM5YTk4Y2IyMTJkMWY4Y2I4MmMzMTVjNTM3NzMyYzY2MDYwYjIiLCJmaWxlIiwiZGVwZW5kZW50cyIsImRlcGVuZGVuY3ktbm9kZS1pZC92MTpzaGEyNTY6YmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYmJiYiIsMSwiZGVwZW5kZW5jeS1ub2RlLWlkL3YxOnNoYTI1NjphYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhIl0
```

Changing these expected bytes to fit future production code is not validation.
Any discrepancy requires investigation against the approved canonical definition.

### Prose erratum (2026-09-10)

The initial document misstated the domain length as 27 / `1b`. Independent .NET
recalculation confirms 28 UTF-8 bytes and prefix `1c` reproduce the already-frozen
empty digest `27f88fcd...`; prefix `1b` instead gives
`b51c0e1f00275dbcad7b2715e932863c4ef2e4bcea5e7a440a2c5d14b28607d9`.
All frozen digest/cursor expected values are unchanged. This corrects the prose,
not the approved length-prefixed algorithm or the independent vector outputs.
