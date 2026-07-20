# Aegis CodeMind

Working name for an evidence-first AI software engineering platform.

## Purpose

Help engineers understand codebases, investigate failures, identify root causes, propose the smallest safe patch, and validate it with evidence.

This is not a general-purpose chatbot. The first release is deliberately narrow: local repository intelligence.

## Status

Repository Intelligence Engine **1.0.0** is implemented and frozen. It deterministically discovers a local repository, applies ignore rules, detects languages/frameworks/build tooling, synthesizes repository metadata, and publishes an immutable executive summary. It does not use an LLM or execute repository code.

Real-world validation passed on 15 pinned public repositories across the supported ecosystems. No RIE 1.0.1 bug-fix release was required.

Go Language Intelligence **1.0.0** is also implemented, validated, and frozen.
It inventories Go packages, imports, structs, interfaces, functions, methods,
constants, and variables with deterministic IDs, exact source locations, and
content digests. Its `1.0.x` line is restricted to compatible defect fixes.

## Run RIE

```powershell
cd backend
go run ./cmd/rie -path D:\Projects\MyRepository
```

The command emits the versioned JSON schema `1.0.0`.

## Documentation

- [Vision](docs/Vision/PROJECT_VISION.md)
- [Principles](docs/Vision/CORE_PRINCIPLES.md)
- [Version 1 scope](docs/PRD/V1_SCOPE.md)
- [Architecture](docs/Architecture/HIGH_LEVEL_ARCHITECTURE.md)
- [Technology decisions](docs/Architecture/TECHNOLOGY_STACK.md)
- [Module map](docs/Architecture/MODULES.md)
- [Roadmap](docs/Roadmap/DEVELOPMENT_ROADMAP.md)
- [Artifact dependency graph](docs/Architecture/ARTIFACT_DEPENDENCY_GRAPH.md)
- [RIE public API](docs/API/RIE_PUBLIC_API_V1.md)
- [RIE stabilization report](docs/Roadmap/RIE_STABILIZATION_REPORT.md)
- [RIE real-world validation report](docs/Validation/RIE_VALIDATION_REPORT.md)
- [LIE architecture candidate](docs/Architecture/LANGUAGE_INTELLIGENCE_ENGINE.md)
- [Go Language Engine candidate](docs/Architecture/GO_LANGUAGE_ENGINE.md)
- [LIE implementation roadmap](docs/Roadmap/LIE_IMPLEMENTATION_ROADMAP.md)
- [GoLanguageInventory 1.0 public API](docs/API/GO_LANGUAGE_INVENTORY_V1.md)
- [GoLanguageInventory 1.0 release package](docs/Releases/GoLanguageInventory-1.0.0/README.md)

## Next milestone

Language Intelligence Engine Phase 2.2 Semantic Resolution is the next approved
development cycle. No LLM, patch generation, or repository code modification
capability has started.
