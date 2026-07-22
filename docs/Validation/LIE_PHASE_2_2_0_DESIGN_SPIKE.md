# LIE Phase 2.2.0 Design Spike Report

## Status

- Date: 2026-07-21
- Spike status: Complete
- Review status: Accepted on 2026-07-22
- ADR 0008: `Accepted`
- Phase 2.2.1 production implementation: Authorized

## Purpose

Validate the architectural assumptions behind Go package identity and semantic resolution before accepting ADR 0008 or creating production packages.

The spike is reproducible at `experiments/phase-2.2.0-semantic-spike`. It is an isolated Go module that uses only the Go standard library and is not imported by `backend/`.

## Environment

| Item | Value |
|---|---|
| Operating system | Windows/amd64 |
| Go | `go1.26.2` |
| CPU | 12th Gen Intel Core i5-12450H |
| Logical benchmark suffix | `-12` |
| Default CGO | Disabled |
| External dependencies | None |

## Validation Matrix

| Architecture assumption | Evidence | Result |
|---|---|---|
| Missing external imports can remain partial | Rejecting `types.Importer` produced an import error while `go/types.Info` retained local definitions and uses | Pass |
| Generics are supported in-process | Generic constraint, function, and instantiation populated `types.Info.Instances` | Pass |
| Pointer/value method sets are distinguishable | `T` did not implement the fixture interfaces; `*T` implemented both the base and embedded interface | Pass |
| Package identity precedence is deterministic | Typed fixtures covered same module, workspace module, workspace-over-module replace, version-specific replace, vendor, nested module, standard library, inactive replace, and conflicting contexts | Pass |
| Unknown is preferred to guessing | Missing proof stayed external/unresolved; conflicting contexts became ambiguous | Pass |
| Every run is a full rebuild | Shuffled and changed-input runs parsed every supplied file and accepted no prior result/cache | Pass |
| Output is deterministic | Shuffled input order produced deeply equal results and stable-sorted collections | Pass |
| IDs have stable unambiguous encoding | Declaration, relation, and package-proof golden vectors passed with UTF-8 byte-length-prefixed variable segments | Pass |
| Interface candidates are bounded by evidence | Only approved event kinds produced candidates; duplicates were removed; global limit reported omissions | Pass |
| Cancellation checkpoints are enforceable | AST traversal stopped exactly at 1,024 nodes and relationship work stopped exactly at 256 items | Pass |
| Diagnostics are deterministic | Sort, exact duplicate suppression, per-file/global limits, aggregate-slot reservation, and omission count passed | Pass |
| Config can evolve additively | `Config{}` used defaults; negative limits failed | Pass |
| Spike has no command/network/package loader | Static import test rejects `os/exec`, network packages, and `go/packages`; spike `go.mod` has no `require` or `replace` | Pass |

## Package Identity Findings

The policy-level resolver validated these rules:

1. Same-module paths map only through the declared module prefix and exact package-relative path.
2. Workspace `use` modules can provide independent local package roots.
3. Workspace replacements override module-level replacements in a workspace context.
4. A version-specific matching replacement takes precedence over an unversioned replacement.
5. A replacement without a matching required/selected module does not activate a dependency.
6. Vendor contexts select the verified vendor package rather than bypassing it through a local replacement.
7. Parent modules do not cross nested-module boundaries.
8. Standard-library classification requires an exact versioned index; path shape is insufficient.
9. Different applicable contexts selecting different targets produce an ambiguous result with sorted candidates.

These tests operate on typed manifest facts. Parsing and source-range extraction for real `go.mod`, `go.work`, and `vendor/modules.txt` files remain Phase 2.2.1 implementation work.

The precedence rules remain grounded in the official [Go Modules Reference](https://go.dev/ref/mod), [go.mod reference](https://go.dev/doc/modules/gomod-ref), and [Go command workspace documentation](https://pkg.go.dev/cmd/go).

## `go/types` and Importer Findings

- `go/types` itself ran in-process without cgo, commands, network access, or `go/packages`.
- A rejecting importer made unavailable external packages explicit while preserving usable local semantic information.
- Generic instances and embedded/pointer method sets behaved as required by the proposed semantic model.
- Type errors were returned as data suitable for normalization and deterministic sorting.
- The candidate importer boundary is viable: repository imports can be supplied only from approved local proof/type facts, while unavailable dependencies remain partial/external.

The spike does not validate a production local-package importer because `GoPackageIdentityInventory` is intentionally not implemented before authorization.

## Stable ID Findings

The original delimiter-only examples were not sufficiently self-delimiting for arbitrary repository paths and nested IDs. The spike established byte-length-prefixed variable segments under the existing versioned schemes.

Golden vectors:

```text
go:semantic:v1:file:11:pkg/main.go#11:function:Run
go:semantic:v1:relation:50:go:semantic:v1:file:11:pkg/main.go#11:function:Run#implements#52:go:semantic:v1:file:12:pkg/types.go#10:struct:Worker
go:package-proof:v1:14:workspace:root#18:go:package:app#app#20:example.com/dep/util#local-replace,workspace-module
```

Lengths are UTF-8 byte lengths. Phase 2.1-compatible source offsets remain zero-based byte offsets. The architecture/model documents were updated with these candidate vectors; ADR 0008 remains proposed.

## Full-Rebuild and Determinism Findings

- Every `Run` call sorted a defensive copy of its input and parsed every file.
- No previous result, AST, type state, or cache was accepted.
- Shuffling files did not change IDs, counts, errors, or ordering.
- Changing one of three files still caused all three files to be parsed.

This validates full rebuild as the deterministic reference behavior. It does not authorize incremental caching.

## Cancellation Findings

The experimental checkpoints match the architecture:

- before synchronous work;
- at 1,024-node AST traversal intervals;
- at 256-relationship intervals;
- before and after package type checking.

Observed microbenchmarks:

| Unit | Time | Allocated bytes | Allocations |
|---|---:|---:|---:|
| Cancel at 1,024 AST nodes | 46.0 µs | 245 B | 6 |
| Cancel at 256 relationships | 6.1 µs | 96 B | 2 |
| Process 100,000 relationship checkpoints | 258.9 µs | 0 B | 0 |
| Type-check one 1,000-file simple package | 9.80 ms | 7,518,576 B | 46,422 |

Go parser and `go/types` remain synchronous inside one call. These values validate the checkpoint design on the reference machine, not a universal wall-clock guarantee. Production size limits and real-repository validation must establish the final cancellation target.

## Performance Baseline

Command:

```powershell
go test -run "^$" -bench . -benchtime=3x -benchmem ./...
```

| Full rebuild fixture | Time/op | Bytes/op | Allocations/op |
|---|---:|---:|---:|
| 100 files | 2.00 ms | 1,138,853 | 14,652 |
| 1,000 files | 15.95 ms | 11,618,701 | 144,486 |
| 10,000 files | 182.63 ms | 120,404,781 | 1,442,029 |

The generated files are small, valid, in-memory fixtures. Results show approximately linear scaling for this workload, but they are not a Phase 2.2 release performance gate and do not represent filesystem, real dependency, build-tag, or enterprise-repository costs.

## Verification Results

| Command | Result |
|---|---|
| `go test -count=1 ./...` | Pass |
| `go test -shuffle=on -count=10 ./...` | Pass |
| `go vet ./...` | Pass |
| Coverage run | Pass — 88.8% statements |
| Benchmarks with allocations | Pass |
| `go test -race -count=1 ./...` | Not run successfully — Windows race build requires CGO and configured `gcc`; CGO was disabled and `gcc` was not installed |

The race limitation does not change the `go/types` finding: normal parsing/type checking passed with CGO disabled. The experimental harness currently has no concurrent workers. Production race validation remains mandatory and must run on a suitable CI runner.

## Document Changes Resulting From the Spike

- Added UTF-8 byte-length prefixes and golden vectors to semantic/proof IDs.
- Clarified that the aggregate diagnostic is always the final emitted item after stable-sorted ordinary diagnostics.
- Preserved the approved full-rebuild, importer, package-proof, interface-bound, cancellation, and zero-config policies without architectural redesign.

## Known Limitations

- The spike uses typed package-identity facts; it does not implement real manifest parsers.
- It uses generated in-memory Go files rather than real repositories.
- It does not implement a production local package importer or standard-library index.
- It does not evaluate build tags, GOOS/GOARCH, cgo source selection, generated files, or compiler flags.
- It does not persist or publish candidate artifacts.
- It does not validate incremental execution, which remains explicitly deferred.
- Race instrumentation could not run on the current Windows environment.

## Exit-Gate Assessment

The design spike provides positive evidence for:

- bounded `go/types` operation with partial external imports;
- package-identity proof precedence and ambiguity;
- deterministic full rebuilds;
- stable ID versioning and golden encoding;
- bounded interface candidate derivation;
- cancellation checkpoints;
- diagnostic stability;
- command/network/importer restrictions;
- a provisional performance and memory baseline.

No finding requires changing the approved architecture. The length-prefixed ID encoding is a candidate-contract clarification and is already reflected in the architecture documents.

## Recommendation

Submit this report and the spike harness for engineering acceptance. If the findings and stated limitations are accepted, the next governance action is:

1. Change ADR 0008 from `Proposed` to `Accepted`.
2. Record explicit authorization for Phase 2.2.1.
3. Begin only the Go Package Identity prerequisite—not the semantic production engine.

## Engineering Acceptance

Engineering accepted this report on 2026-07-22 and changed ADR 0008 to `Accepted`. Authorization applies only to Phase 2.2.1 — Go Package Identity Engine. Phase 2.2.2 and later remain unauthorized.
