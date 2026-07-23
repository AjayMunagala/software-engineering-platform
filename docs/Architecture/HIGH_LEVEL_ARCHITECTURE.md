# High-Level Architecture

## Current platform flow

```text
Authorized repository
  -> Repository Intelligence artifacts 1.0.0
  -> Go Language Inventory 1.0.0
  -> Go Package Identity Inventory 1.0.0
  -> Go Semantic Inventory 1.0.0
  -> Application orchestration
  -> Persistence Boundary (Phase 3 design)
  -> PostgreSQL adapter (future)
  -> REST / gRPC (future)
  -> React UI (future)
```

## Delivered boundary

Repository Intelligence and the frozen Go syntax, package-identity, and semantic
artifacts are released. Persistence is design-only and is not yet a runtime
dependency.

## Future component responsibilities

- Repository intelligence: scan files and publish repository facts.
- Language/semantic intelligence: publish versioned syntax and semantic facts.
- Persistence boundary: store exact immutable artifacts without interpreting
  or modifying them.
- Knowledge/query layer: expose durable artifact history and rebuildable
  projections.
- Reasoning engine: collect evidence, rank hypotheses, and explain conclusions.
- Validation engine: run approved builds, tests, linters, and static analysis.
- Model adapter: provide a stable interface to replaceable language models.

## Safety boundary

No component may edit a repository or execute commands without an explicit, reviewable request and result capture.

No intelligence engine may require a database to produce its artifact.
