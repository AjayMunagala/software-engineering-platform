# Go Semantic Resolution Engine

## Purpose

Produce a deterministic, immutable semantic artifact from snapshot-authorized,
digest-verified Go source and the frozen Go syntax and package-identity
artifacts.

## Implemented Scope

Phase 2.2.2 implements:

- candidate engine and artifact contracts;
- configuration and prerequisite validation;
- repository-root and snapshot-path enforcement;
- source size, regular-file, symlink, and SHA-256 verification;
- immutable semantic file outcomes, diagnostics, provenance, and statistics;
- deterministic concurrency, ordering, cancellation, and diagnostic limits.

Phase 2.2.3 additionally:

- re-parses only digest-verified source into ephemeral AST/token state;
- reconciles top-level structs, interfaces, functions, methods, constants, and
  variables with exact Phase 2.1 symbol IDs;
- records defined types, aliases, parameters, results, fields, locals, labels,
  and type parameters with stable semantic IDs;
- builds package, file, function, type, and nested lexical scopes during each
  run without persisting the scope tree.

Phase 2.2.4 additionally:

- binds value, pointer, and generic method receivers only to exact local types
  in the same package;
- preserves unresolved and ambiguous receiver states without selecting by
  name alone;
- emits deterministic `uses`, `embeds`, `alias-of`, `instantiates`, and
  `constrains` type relations from declared type contexts;
- distinguishes proven local targets, predeclared types, structural types,
  unresolved qualified types, and ambiguous local targets;
- applies the configured relationship budget with explicit omission counts
  and diagnostics.

Phase 2.2.5 additionally:

- emits stable identifier, selector, type, and instantiation references;
- resolves lexical, package-local, same-package cross-file, and proven local
  imported-package targets;
- models default, named, dot, and blank imports without executing tools;
- re-hashes every repository manifest used by a package-identity proof before
  consuming the proof;
- combines applicable resolution contexts conservatively: they must agree on
  one fresh target before a repository import is resolved;
- preserves external, unresolved, ambiguous, and partial states rather than
  inferring package identities from path suffixes;
- budgets imports first, then receiver bindings, type relations, and references
  so foundational import evidence is not silently displaced by derived uses.

Phase 2.2.6 additionally:

- rebuilds package type state in process with `go/types` and no importer;
- derives bounded interface candidates from compile-time assertions, typed
  variable assignments, assignments, conversions, arguments, returns, and
  exact local embedded-interface relations;
- proves value and pointer method sets, including embedded interfaces and
  instantiated local generic types;
- reports deterministic `proven`, `disproven`, or `unknown` outcomes and sorted
  missing or mismatched method names;
- treats missing imports, unrelated package type errors, ambiguous declarations,
  and incomplete type information as `unknown`;
- never compares every concrete type with every interface.

Phase 2.2.7 additionally:

- retrieves the three exact typed prerequisites from `rie.ArtifactStore`;
- performs a fresh full semantic rebuild on every integration run;
- publishes `GoSemanticInventory 1.0.0` exactly once without mutable context
  fields or changes to the Phase 2.1 artifact;
- rejects missing, wrongly typed, canceled, and duplicate-publication runs;
- exposes `GoSemanticInventoryView` as a detached JSON/reporting model; and
- retains no semantic state between repositories or runs.

Files remain `partial` because later semantic relationships are not yet
authorized. Exact declaration matches are independently marked `resolved`.

## Inputs

- `RepositorySnapshot 1.0.0`
- `GoLanguageInventory 1.0.0`
- `GoPackageIdentityInventory 1.0.0`

## Output

- `GoSemanticInventory 1.0.0`
- ID scheme `go-semantic-id/v1`

## Boundaries

The engine is local and read-only. It executes no commands, performs no
network access or downloads, reads no module cache, writes no repository
files, and persists no source, AST, token, or `go/types` state.

Real-repository validation remains outside this package's implementation scope
and was performed and accepted through Phase 2.2.8. Phase 2.2.9 stabilization
and the `1.0.0` freeze are authorized. A default import of an external
package keeps an empty local name when no exact package-name proof exists;
selectors through that unknown name remain unresolved rather than using the
import-path suffix as a guess. Interface checking likewise does not import
external packages or claim complete method sets when package errors remain.

## Package Standard

This package contains the mandatory interface, implementation, configuration,
models, errors, README, tests, and benchmarks.

## Candidate Integration Example

After RIE, Go Phase 2.1, and package identity have published their immutable
artifacts into the same per-run store:

```go
candidate, err := semantic.NewIntegrator()
if err != nil {
    return err
}
inventory, err := candidate.Run(ctx, store)
if err != nil {
    return err
}
encoded, err := json.MarshalIndent(inventory.View(), "", "  ")
```

`Run` never consumes an earlier `GoSemanticInventory`. Because artifact names
are single-assignment, a second run requires a new per-run store and therefore
cannot silently reuse or overwrite previous semantic state.

## Real-Repository Validation Harness

`TestRealRepositoryValidation` is skipped during ordinary tests. Set
`SEMANTIC_VALIDATION_ROOT` and run the named test to collect the Phase 2.2.8
JSON evidence. Optional variables select the label, pinned commit, output path,
controlled stale path, cancellation delay, and large-repository isolation
behavior. The harness executes no target-repository commands and downloads no
dependencies.

Phase 2.2.9 stabilization reduced OpenTelemetry peak heap from 1,384.6 MiB to
1,059.1 MiB and Kubernetes peak heap from 5.08–5.25 GiB to 3.88–3.92 GiB while
preserving accepted semantic facts and omission counts. Public API review,
release evidence, version promotion, and the annotated `1.0.0` tag are
complete.
