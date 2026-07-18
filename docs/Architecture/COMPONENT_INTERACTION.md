# Component Interaction

## Current interaction contract

The scanner exposes one bounded capability: convert a local repository path into a `RepositoryReport`. Manifest readers and classifiers are internal collaborators; they return structured facts with source-file references and confidence only when a detection is heuristic.

```text
Scanner -> File inventory
Scanner -> Language detector
Scanner -> Manifest readers
Scanner -> Framework/tooling classifier
Scanner -> Report writer
```

## Future interaction rules

- Repository intelligence owns repository facts.
- The reasoning engine consumes facts and records hypotheses separately.
- The validation engine owns command execution and results.
- The model adapter never becomes the system of record.
- Every conclusion carries its evidence references.
