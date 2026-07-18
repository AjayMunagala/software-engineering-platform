# Execution Pipeline

Execution is not part of Version 1. When introduced, it must follow this sequence:

```text
Proposed action -> Explicit approval -> Preconditions -> Sandboxed execution
-> Capture stdout/stderr/status -> Validate outcome -> Report and audit
```

No generated patch, migration, build, test, or external operation runs merely because a model recommends it.
