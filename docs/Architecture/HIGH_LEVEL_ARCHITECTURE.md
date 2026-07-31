# High-Level Architecture

## Current platform flow

```text
Authorized repository
  -> Repository Intelligence artifacts 1.0.0
  -> Go Language Inventory 1.0.0
  -> Go Package Identity Inventory 1.0.0
  -> Go Semantic Inventory 1.0.0
  -> Repository Service 1.0.0
  -> Persistence Port 1.0.0
  -> PostgreSQL Adapter 1.0.0
  -> Runtime Infrastructure 1.0.0
  -> Dependency Intelligence (Phase 5.0 design candidate)
  -> REST / gRPC (future)
  -> React UI (future)
```

## Delivered boundary

Repository Intelligence, frozen Go syntax/package-identity/semantic artifacts,
Persistence Port, PostgreSQL Adapter, Runtime Infrastructure, and Repository
Service are released at `1.0.0`. Phase 5.0 designs Dependency Intelligence as a
downstream artifact consumer without changing those released contracts.

## Future component responsibilities

- Repository intelligence: scan files and publish repository facts.
- Language/semantic intelligence: publish versioned syntax and semantic facts.
- Dependency intelligence: publish deterministic module, package, and file
  graphs, SCCs, cycles, and bounded impact evidence.
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

Dependency Intelligence may consume released repository and language artifacts
but must not reread source, execute repository tools, or infer architecture
policy. Architecture Intelligence remains a later consumer.
