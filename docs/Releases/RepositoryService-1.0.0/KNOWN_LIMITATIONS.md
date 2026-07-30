# Repository Service 1.0.0 Known Limitations

## Open release qualification

Engineering accepted this qualification for `1.0.0`. The available 8-GB-class
workstation could not complete Kubernetes with one
Windows worker or eight Ubuntu workers within safe memory ceilings. Two clean
Windows eight-worker runs completed deterministically. This is classified as a
host resource limitation, not an observed software defect. Larger-host evidence
may be appended later.

## Product limitations

- Only the frozen `repository-go/v1` profile is supported.
- Scans are synchronous and in-process; there is no distributed scheduler.
- Repository acquisition remains an operator responsibility.
- Repository-owned commands, dependency downloads, builds, and tests are not
  executed.
- External Go dependencies unavailable from repository evidence remain
  explicit unknown/partial semantic states.
- No transport, authentication, authorization, UI, IDE, or AI capability is
  included.
- The neutral Go service contract does not define a JSON or HTTP wire format.
