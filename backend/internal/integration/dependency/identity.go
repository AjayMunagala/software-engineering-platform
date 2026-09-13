package dependency

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
)

const Version = "0.1.0"
const Scheme = "dependency-platform-artifact-id/v1"
const ManifestScheme = "dependency-platform-manifest/v1"

func text(h hash.Hash, s string)   { number(h, uint64(len(s))); h.Write([]byte(s)) }
func number(h hash.Hash, n uint64) { var b [8]byte; binary.BigEndian.PutUint64(b[:], n); h.Write(b[:]) }
func sum(h hash.Hash) (r [32]byte) { copy(r[:], h.Sum(nil)); return }
func profileDigest() [32]byte {
	h := sha256.New()
	for _, s := range []string{"dependency-platform-profile/v1", "dependency-publication", Version, "dependency-inventory", Version, "dependency-canonical-json", Version, "application/json", Scheme, "dependency-platform-integration", Version, ManifestScheme, "logical-source-references-only"} {
		text(h, s)
	}
	number(h, 1)
	number(h, 0)
	number(h, 0)
	return sum(h)
}
func artifactID(q Query) string {
	h := sha256.New()
	for _, s := range []string{Scheme, q.scope.ScopeID(), q.repository, q.scan, "dependency-inventory", Version} {
		text(h, s)
	}
	b := h.Sum(nil)[:16]
	b[6] = (b[6] & 15) | 128
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func manifest(e ExpectedPublication) [32]byte {
	h := sha256.New()
	q := e.query
	for _, s := range []string{ManifestScheme, q.scope.ScopeID(), q.repository, q.scan} {
		text(h, s)
	}
	p := profileDigest()
	h.Write(p[:])
	text(h, e.revision)
	number(h, 1)
	for _, s := range []string{artifactID(q), "dependency-inventory", Version, Scheme, "dependency-canonical-json", Version, "application/json", "dependency-platform-integration", Version} {
		text(h, s)
	}
	h.Write(e.digest[:])
	number(h, e.size)
	number(h, 0)
	number(h, 0)
	number(h, 0)
	return sum(h)
}
func child(r PublishRequest, operation string, e ExpectedPublication) string {
	h := sha256.New()
	for _, s := range []string{"dependency-platform-request/v1", r.request, operation, r.query.scope.ScopeID(), r.query.repository, r.query.scan} {
		text(h, s)
	}
	m := manifest(e)
	h.Write(m[:])
	return hex.EncodeToString(h.Sum(nil))
}
