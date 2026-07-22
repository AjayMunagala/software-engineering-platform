# Semantic Spike Package

## Purpose

Validate Phase 2.2 architecture assumptions without creating a production
semantic engine.

## Responsibilities

- Perform complete in-memory Go re-parses.
- Exercise bounded `go/types` with an importer that always rejects external
  packages.
- Model deterministic package-identity precedence.
- Validate ID, diagnostics, candidate, and cancellation policies.

## Non-Responsibilities

- No filesystem repository scanning.
- No production artifact or public API.
- No command execution, network access, module cache, patching, or AI.

## Inputs and Outputs

Inputs are generated in-memory Go fixtures and typed identity facts. Outputs
are experimental counts, stable IDs, decisions, test assertions, and benchmark
measurements.

## Package Files

The package follows the project standard: interface, implementation,
configuration, models, errors, README, tests, and benchmarks.

## Exit

The package is retained as reproducible evidence. It must not be imported by
`backend/` production code.
