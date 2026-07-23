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

Files remain `partial` because later semantic relationships are not yet
authorized. Exact declaration matches are independently marked `resolved`.

## Inputs

- `RepositorySnapshot 1.0.0`
- `GoLanguageInventory 1.0.0`
- `GoPackageIdentityInventory 0.1.0`

## Output

- `GoSemanticInventory 0.1.0`
- ID scheme `go-semantic-id/v1`

## Boundaries

The engine is local and read-only. It executes no commands, performs no
network access or downloads, reads no module cache, writes no repository
files, and persists no source, AST, token, or `go/types` state.

Interface satisfaction remains outside the authorized scope. Phase 2.2.6 and
later are not authorized by this package. A default import of an external
package keeps an empty local name when no exact package-name proof exists;
selectors through that unknown name remain unresolved rather than using the
import-path suffix as a guess.

## Package Standard

This package contains the mandatory interface, implementation, configuration,
models, errors, README, tests, and benchmarks.
