# Go Dependency Adapter — candidate 0.1.0

Phase 5.0.3 translates released immutable RIE/Go LIE artifacts into the accepted
neutral graph core. Implementation commit `a0a1a77` is engineering accepted;
ADR 0021 is Accepted. Candidate 0.1.0 is not a production release.

## Use

Construct `Inputs` with `NewInputs(ctx, InputParams{...})`, then construct a
validated `Config` with `NewConfig(ConfigParams{})`, and an engine with `New`.
Call `Analyze(ctx, inputs)` to obtain one `die.DependencyInventory`.
Zero Inputs/Config and nil contexts are invalid. Inspect errors using
`errors.As(err, *Error)` and `Kind`/`Code`; only cancellation causes unwrap.

The four required concrete artifacts are RepositorySnapshot, GoLanguageInventory,
GoPackageIdentityInventory, and GoSemanticInventory, all 1.0.0. Stored metadata,
prerequisite references, membership, digest equality, proof joins, aliases, and
source locations are validated. Name/version references do **not** prove a common
scan or current filesystem freshness. The caller owns coherent input selection.

## Translation

- Modules, packages, and files retain released source identities.
- Module ownership uses deepest declared path-segment ancestry, with conflicting
  roots rejected. No module ownership is invented for unmanaged packages.
- Every syntax import is counted once per relevant graph, regardless of how much
  binding/proof evidence accompanies it. Imports never guess a target file.
- Resolved references create cross-file edges only for exact verified targets.
- Missing/partial, ambiguous, stale, external, and standard-library boundaries
  remain explicit. Standard-library classification requires supplied proof.
- The neutral core alone owns canonical IDs, ordering, duplicate aggregation,
  output limits, and immutable accessors. SCC/cycle collections remain empty.

The boundary-name vectors were independently computed and committed in `7eb2a98`
before production encoding. See `docs/API/GO_DEPENDENCY_BOUNDARY_GOLDEN_VECTORS.md`.

## Bounds and cancellation

Defaults are 2,000,000 raw/generated records and 4,000,000 evidence contributions;
ceilings are 10,000,000 and 20,000,000. Counts include nested membership/candidate
IDs, imports, snapshot entries, generated containments/edges/diagnostics, and
proof evidence. Exceeding a bound returns no artifact; no input-order truncation.
Unique nodes obey the supplied core MaxNodes before graph publication.

Released accessor clones occur before their returned sizes can be checked.
These allocations are not a hard byte-memory ceiling. Context checks surround
accessors and occur during validation/translation, before normalization and return.
An individual clone or core sort cannot be interrupted by this adapter. Calls
share no mutable working state. Graph evidence is budget-checked before expansion.

Direct-core hardening DIE-HARDEN-001 and DIE-HARDEN-002 remain open. This adapter's
guards do not close those separate obligations.

## Tests and scope

Run `go test ./die/conformance ./die/golang` from backend before broader gates.
Fixtures run released engines only in test setup. `BenchmarkAdapter` excludes
fixture creation; `BenchmarkStages` separates extraction, translation, and core
normalization. `DIE_ADAPTER_MEMORY_TEST=1` enables the opt-in 10,000-file memory
measurement. It includes live prerequisites in sampled peak heap.

Production code does not read source/manifests, parse, execute tools, use network,
or integrate persistence/runtime/Repository Service. Transitive released-artifact
packages contain engine code, but the adapter does not invoke those engines.
Phase 5.0.4, SCC/cycle/impact, integration, language expansion, and release remain
unauthorized. See the dedicated validation report for measured evidence.
