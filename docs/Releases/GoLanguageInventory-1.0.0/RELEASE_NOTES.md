# Release Notes

GoLanguageInventory 1.0.0 is the first stable source-code intelligence artifact
in Aegis CodeMind. It converts frozen repository facts into deterministic Go
syntax facts without running Go commands, loading packages, downloading modules,
or using an LLM.

The release parsed 19,380 Go files across five pinned public repositories with
no unexpected failures, crashes, or nondeterministic results. A controlled
malformed repository confirmed that one invalid file produces a bounded
diagnostic without discarding valid neighboring files.

One medium parser defect was discovered during Kubernetes validation and fixed
in commit `f598c51`. The full validation decision is recorded in the canonical
report.

The artifact is intended as a stable input to Phase 2.2 Semantic Resolution.
That phase must publish separate semantic artifacts rather than changing the
meaning of the frozen syntax inventory.
