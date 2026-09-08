# Go Dependency Boundary Name Golden Vectors

Frozen before adapter implementation, 2026-09-08, pursuant to the approval of
design commit `4b77de2034af7619704d8df5f5f65ef31b5dd552`.

The domain is `go-dependency-boundary-name/v1`. Hash four segments: domain,
importing package ID, context ID, import path. Each segment is its uint64
big-endian UTF-8 byte length followed by exact UTF-8 bytes. Output is
`go-dependency-boundary-name/v1:sha256:` followed by the lowercase SHA-256.
Do not normalize Unicode, fold case, omit an empty segment, or concatenate
unframed fields.

These expected bytes were assembled with .NET UTF-8 and big-endian lengths,
then hashed using .NET SHA256, before any production Go encoder existed.
Production tests must consume these constants, not regenerate expectations.

| Case | Package | Context | Import | SHA-256 |
|---|---|---|---|---|
| Unicode | `pkg:π` | `ctx:工作` | `example.com/β` | `73a932f02665c68184b80cd9d36f2a138061eaced6fdd3fc65e928b26009ce52` |
| Empty context | `pkg:a` | empty string | `fmt` | `c397d0371aeaace5958aeed411a75c6cbac46885a4fbd6f41260fcbf6a09d6f2` |
| Split A | `ab` | `c` | `d` | `73a9c0c2ba1156f5aa5d662f0955bdc5cbcb50abac1696119c5df348122be620` |
| Split B | `a` | `bc` | `d` | `4ce6c7abf4f42427511275958983b3b3b7f0be80b5f87f1b960d5e1d5e03334e` |

## Exact input bytes (hex)

Unicode:

```text
000000000000001e676f2d646570656e64656e63792d626f756e646172792d6e616d652f76310000000000000006706b673acf80000000000000000a6374783ae5b7a5e4bd9c000000000000000e6578616d706c652e636f6d2fceb2
```

Empty context:

```text
000000000000001e676f2d646570656e64656e63792d626f756e646172792d6e616d652f76310000000000000005706b673a6100000000000000000000000000000003666d74
```

Split A:

```text
000000000000001e676f2d646570656e64656e63792d626f756e646172792d6e616d652f763100000000000000026162000000000000000163000000000000000164
```

Split B:

```text
000000000000001e676f2d646570656e64656e63792d626f756e646172792d6e616d652f763100000000000000016100000000000000026263000000000000000164
```
