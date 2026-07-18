# High-Level Architecture

## Product flow

```text
User interface
  -> Planner / workflow controller
  -> Repository intelligence
  -> Knowledge layer
  -> Reasoning engine
  -> Validation engine
  -> Model adapter
  -> Coding model
```

## Version 1 boundary

Only **Repository Intelligence** is implemented in Version 1, starting with the Repository Scanner. All other components are architectural placeholders, not current deliverables.

## Future component responsibilities

- Repository intelligence: scan files, parse source, extract symbols and dependencies.
- Knowledge layer: store repository facts and relationships.
- Reasoning engine: collect evidence, rank hypotheses, and explain conclusions.
- Validation engine: run approved builds, tests, linters, and static analysis.
- Model adapter: provide a stable interface to replaceable language models.

## Safety boundary

No component may edit a repository or execute commands without an explicit, reviewable request and result capture.
