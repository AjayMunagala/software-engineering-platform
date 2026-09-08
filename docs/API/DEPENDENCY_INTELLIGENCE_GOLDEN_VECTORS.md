# Dependency Intelligence Candidate Golden Vectors

## Status

- Phase: 5.0.1 design spike, engineering accepted 2026-09-08
- Contract: frozen for the listed node, edge, containment, SCC, and cycle vectors
- ID schemes: frozen `v1` for all five listed schemes
- Production release: not authorized

These vectors remove ambiguity from the candidate ID representation. The
design-spike evidence was accepted on 2026-09-08, so the listed node, edge,
containment, SCC, and cycle values are frozen production acceptance conditions.

## Canonical segment encoding

Every ID hashes this byte stream:

1. the ID-scheme domain as the first segment;
2. each logical identity field in the order defined below;
3. each segment encoded as an unsigned 64-bit big-endian byte length followed
   by the exact UTF-8 bytes;
4. no delimiter, Unicode normalization, case folding, host path, timestamp,
   database identity, or slice position.

The digest is lowercase hexadecimal SHA-256. The textual form is:

```text
<scheme>:sha256:<64 lowercase hexadecimal characters>
```

Repository paths are slash-normalized and repository-relative before encoding.

## Field order

| Scheme | Ordered logical fields after domain |
|---|---|
| `dependency-node-id/v1` | node kind, language, qualified name, repository path, resolution |
| `dependency-edge-id/v1` | graph kind, dependency kind, source node ID, target node ID, resolution |
| `dependency-containment-id/v1` | containment kind, parent node ID, child node ID |
| `dependency-scc-id/v1` | graph kind, canonically sorted member node IDs |
| `dependency-cycle-id/v1` | graph kind, SCC ID |

Containment identity was frozen during Phase 5.0.2 before production
containment publication.

## Golden vectors

Node input:

```text
kind=package
language=Go
qualified_name=example.com/π/pkg
path=src\pkg (normalized to src/pkg)
resolution=resolved_local
```

Node output:

```text
dependency-node-id/v1:sha256:16c0f7392130f9bad33f66087e9c0b46c1f2823ba8cf7ea491187b35494c8a64
```

Edge output using the node above, target
`dependency-node-id/v1:sha256:target`, package/imports/resolved-local:

```text
dependency-edge-id/v1:sha256:bf8b464f5cbb5551b28068da1445e8e45359e2586f860ede189d9ac68b4c8ad1
```

Containment input:

```text
kind=module_contains_package
parent_id=dependency-node-id/v1:sha256:parent
child_id=dependency-node-id/v1:sha256:child
```

Containment output:

```text
dependency-containment-id/v1:sha256:3b47d635724cdb8ab06ff0319c2ded1e67d1289fc6a92df92a02a0c29f0c4334
```

Package SCC with sorted member IDs `a` and `β`:

```text
dependency-scc-id/v1:sha256:360e477842ffa8936cb211fa0ef8e621043e3f06105f680537a5d478d7f0a2f8
```

Package cycle derived from that SCC:

```text
dependency-cycle-id/v1:sha256:455fdfe1396a0c675860713ff4be826465b6eb83982e3bd7ecce66734c42eb8c
```

The complete canonical JSON result for the committed three-graph cyclic
fixture has SHA-256:

```text
5ee754fe619d0920f2473e06a04f6bd1372b768c565c82c65281c7b2e922b6dc
```

Windows and Ubuntu tests assert these exact values.
