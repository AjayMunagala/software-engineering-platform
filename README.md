# Aegis CodeMind

Working name for an evidence-first AI software engineering platform.

## Purpose

Help engineers understand codebases, investigate failures, identify root causes, propose the smallest safe patch, and validate it with evidence.

This is not a general-purpose chatbot. The first release is deliberately narrow: local repository intelligence.

## Status

Foundation and product design. No production code has been started.

## Documentation

- [Vision](docs/Vision/PROJECT_VISION.md)
- [Principles](docs/Vision/CORE_PRINCIPLES.md)
- [Version 1 scope](docs/PRD/V1_SCOPE.md)
- [Architecture](docs/Architecture/HIGH_LEVEL_ARCHITECTURE.md)
- [Technology decisions](docs/Architecture/TECHNOLOGY_STACK.md)
- [Module map](docs/Architecture/MODULES.md)
- [Roadmap](docs/Roadmap/DEVELOPMENT_ROADMAP.md)

## First coding milestone

Build a repository scanner that produces factual metadata only: repository name, languages, directory structure, files, manifests, configuration files, frameworks, build tools, and dependencies. It will not use an LLM.
