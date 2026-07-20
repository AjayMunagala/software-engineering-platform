# Repository Intelligence Engine (RIE)

## Decision

The first platform layer is named the **Repository Intelligence Engine**. The Repository Scanner is its first capability, not the long-term name of the entire component.

## Why this name

The Scanner inventories repository facts and publishes the frozen artifacts required by later platform layers. Source parsing, symbols, dependency graphs, and architecture graphs belong to separate Language, Dependency, and Architecture Intelligence Engines that consume RIE artifacts without expanding RIE into a God engine.

## Version 1 boundary

```text
Repository -> RIE Scanner capability -> Metadata -> JSON report
```

No LLM, prompts, agents, patching, execution, or remote access belongs in this milestone.

## Layer progression

1. Repository understanding — Version 1 RIE scanner.
2. Code understanding — syntax, symbols, imports, dependencies.
3. Architecture understanding — components, interfaces, data and service relationships.
4. Reasoning — evidence collection and root-cause hypotheses.
5. Patch generation — minimal, reviewable proposals.
6. Validation — approved checks and evidence capture.
7. Autonomous engineering — only with mature safeguards and explicit governance.

Progress to the next layer only after the previous one has demonstrable reliability and acceptance tests.
