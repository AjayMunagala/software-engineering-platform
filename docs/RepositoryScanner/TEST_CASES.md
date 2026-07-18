# Repository Scanner Test Cases

| ID | Scenario | Expected result |
| --- | --- | --- |
| RS-01 | Missing path | Clear validation error; no report. |
| RS-02 | Empty directory | Valid zero-count report. |
| RS-03 | Go module | Go language, `go.mod`, and declared dependencies detected. |
| RS-04 | React/TypeScript app | TypeScript/JavaScript, `package.json`, scripts, and React detection supported by evidence. |
| RS-05 | SQL project | SQL files counted; table/procedure detection only where parser rules support it. |
| RS-06 | Gitignored directories | Ignored paths excluded and recorded according to configured policy. |
| RS-07 | Nested manifests | All manifests reported with correct paths. |
| RS-08 | Unreadable file | Scan completes with a warning. |
| RS-09 | Binary and large files | Counted without unsafe or excessive content reads. |
| RS-10 | Docker and GitHub Actions | Dockerfiles and workflow definitions reported. |
| RS-11 | Repeated scan | Equivalent canonical JSON for an unchanged fixture. |
| RS-12 | Secret-like config | File is identified without exposing the value in output. |
