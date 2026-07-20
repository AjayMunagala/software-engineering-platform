# Go Language Engine 1.0.0

The Go engine consumes `RepositorySnapshot 1.0.0` and `LanguageInventory
1.0.0`, verifies their `.go` file counts agree, then parses authorized files
with `go/parser` and `parser.SkipObjectResolution`.

It inventories packages, imports, named structs and interfaces, functions,
methods, package constants, and package variables. It stores exact source ranges,
byte counts, and SHA-256 digests, but never stores source bodies, literals, tags,
parameters, struct fields, or interface members.

Unknown repositories produce an empty inventory. File-local failures are recorded
with stable diagnostic codes. Paths are contained beneath the canonical root,
including parent symlink checks, and reads are bounded without executing tools or
accessing the network.

The `go-language-inventory 1.0.0` artifact and its public API are frozen.
Post-release `1.0.x` changes are limited to compatible defect fixes.

## Benchmarks

`implementation_benchmark_test.go` creates deterministic 10-, 1,000-, and
10,000-file repositories. It warms the operating-system file cache before
starting the benchmark timer so Windows antivirus scanning caused by fixture
creation is not attributed to engine analysis. File reads, SHA-256 hashing,
parsing, collection, and immutable-artifact construction remain inside the
timed operation.
