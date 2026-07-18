# Tool Pipeline

```text
Requested capability -> Permission check -> Scoped tool selection
-> Read-only observation or authorized action -> Capture result
-> Normalize into evidence -> Audit record
```

Tools must have narrow scopes. A failure, timeout, or missing permission is a result to report, never a reason to guess or escalate silently.
