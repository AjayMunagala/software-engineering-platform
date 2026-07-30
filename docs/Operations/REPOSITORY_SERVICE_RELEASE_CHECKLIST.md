# Repository Service 1.0 Release Checklist

## Engineering acceptance

- [ ] Phase 4.0.8 report and machine-readable results reviewed.
- [ ] Open Kubernetes qualification explicitly accepted, closed, or release
      blocked.
- [ ] No unresolved critical/high defect.
- [ ] API snapshot and compatibility policy approved.

## Database and runtime

- [ ] Accepted migrations applied and checksum-valid.
- [ ] Persistence compatibility record reports supported 1.0 contracts.
- [ ] Runtime pools, TLS, privileges, readiness, admission, and shutdown proven.
- [ ] No runtime migration or credential committed.

## Source and profile

- [ ] Source resolver accepts only authorized opaque handles.
- [ ] `repository-go/v1` name, version, digest, order, codecs, and dependencies
      match golden vectors.
- [ ] No repository fetch, command, mutation, build, or test path exists.

## Version promotion

- [ ] Change only approved Repository Service version constants and release
      metadata from `0.1.0` to `1.0.0`.
- [ ] Run full regression, vet, shuffle, Windows race, Ubuntu race, golden
      vectors, coverage, conformance, integration, audits, and doc-link checks.
- [ ] Record final public-source hashes and exact commit.
- [ ] Commit and push the version-freeze changes to `main`.
- [ ] Confirm the GitHub commit matches the reviewed hash and tree is clean.

## Tags

- [ ] Create annotated `repository-service/v1.0.0` at the accepted commit.
- [ ] Verify the annotated tag signature/object and target hash locally.
- [ ] Push the tag explicitly and verify the GitHub tag target.
- [ ] Do not create Phase 4.1 or transport tags.
