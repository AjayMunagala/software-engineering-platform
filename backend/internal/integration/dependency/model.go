// Package dependency publishes immutable dependency artifacts through frozen ports.
package dependency

import (
	"context"
	"errors"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Error string

func (e Error) Error() string { return string(e) }

func (e Error) Retryable() bool      { return e == Busy || e == Unavailable || e == Timeout }
func (e Error) OutcomeUnknown() bool { return e == Unknown }

const (
	Invalid     Error = "invalid_input"
	Unsupported Error = "unsupported_contract"
	Missing     Error = "scope_not_found"
	Conflict    Error = "conflict"
	Busy        Error = "busy"
	Limit       Error = "limit_exceeded"
	Integrity   Error = "integrity_failure"
	Unavailable Error = "unavailable"
	Timeout     Error = "timeout"
	Canceled    Error = "canceled"
	Unknown     Error = "outcome_unknown"
	Internal    Error = "internal"
)

func safe(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	var own Error
	if errors.As(err, &own) {
		switch own {
		case Invalid, Unsupported, Missing, Conflict, Busy, Limit, Integrity, Unavailable, Timeout, Canceled, Unknown, Internal:
			return own
		}
		return Internal
	}
	switch p.KindOf(err) {
	case p.ErrorNotFound, p.ErrorAuthorizationDenied:
		return Missing
	case p.ErrorIntegrityFailure:
		return Integrity
	case p.ErrorIdempotencyConflict, p.ErrorLifecycleConflict, p.ErrorDuplicateArtifact:
		return Conflict
	case p.ErrorPayloadTooLarge:
		return Limit
	case p.ErrorCanceled:
		return Canceled
	case p.ErrorTimeout:
		return Timeout
	case p.ErrorInvalidInput:
		return Invalid
	case p.ErrorUnsupportedVersion:
		return Unsupported
	}
	return Unavailable
}
func nilValue(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Map, reflect.Slice:
		return r.IsNil()
	}
	return false
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var requestPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)

type Query struct {
	scope            p.Scope
	repository, scan string
}

func NewQuery(scope p.Scope, repository, scan string) (Query, error) {
	if scope.IsZero() || !uuidPattern.MatchString(scope.ScopeID()) || !uuidPattern.MatchString(repository) || !uuidPattern.MatchString(scan) {
		return Query{}, Invalid
	}
	return Query{scope, repository, scan}, nil
}
func (q Query) Scope() p.Scope       { return q.scope }
func (q Query) RepositoryID() string { return q.repository }
func (q Query) ScanID() string       { return q.scan }
func (q Query) String() string       { return "dependency query (redacted)" }
func (q Query) GoString() string     { return q.String() }

type PublishRequest struct {
	query             Query
	request, revision string
}

func NewPublishRequest(q Query, request, revision string) (PublishRequest, error) {
	if q.scope.IsZero() || !requestPattern.MatchString(request) || len(revision) > 512 || !utf8.ValidString(revision) || strings.ContainsAny(revision, "\\/") {
		return PublishRequest{}, Invalid
	}
	for _, r := range revision {
		if unicode.IsControl(r) {
			return PublishRequest{}, Invalid
		}
	}
	return PublishRequest{q, request, revision}, nil
}
func (r PublishRequest) String() string   { return "dependency publication request (redacted)" }
func (r PublishRequest) GoString() string { return r.String() }

type Route struct{ ScopeID, RepositoryID string }
type Config struct {
	routes       []Route
	finalization time.Duration
}

func NewConfig(routes []Route, finalization time.Duration) (Config, error) {
	if finalization == 0 {
		finalization = 5 * time.Second
	}
	if len(routes) == 0 || len(routes) > 1024 || finalization < time.Second || finalization > 30*time.Second {
		return Config{}, Invalid
	}
	seen := map[Route]bool{}
	for _, r := range routes {
		if !uuidPattern.MatchString(r.ScopeID) || !uuidPattern.MatchString(r.RepositoryID) || seen[r] {
			return Config{}, Invalid
		}
		seen[r] = true
	}
	return Config{append([]Route(nil), routes...), finalization}, nil
}

type ExpectedPublication struct {
	query    Query
	revision string
	size     uint64
	digest   [32]byte
}

// Uncertain retains the exact expectation needed to reconcile an ambiguous write.
// It never formats the expectation, revision or dependency cause into its message.
type Uncertain struct{ expected ExpectedPublication }

func (e *Uncertain) Error() string { return string(Unknown) }
func (e *Uncertain) Unwrap() error { return Unknown }
func (e *Uncertain) Expected() ExpectedPublication {
	if e == nil {
		return ExpectedPublication{}
	}
	return e.expected
}
func (e *Uncertain) GoString() string { return string(Unknown) }

func (e ExpectedPublication) String() string   { return "dependency expectation (redacted)" }
func (e ExpectedPublication) GoString() string { return e.String() }
func (e ExpectedPublication) Query() Query     { return e.query }

type Publication struct {
	expected ExpectedPublication
	scan     p.ScanRecord
	artifact p.ArtifactRecord
	manifest [32]byte
}

func (p Publication) Expected() ExpectedPublication        { return p.expected }
func (p Publication) Digest() [32]byte                     { return p.expected.digest }
func (p Publication) Size() uint64                         { return p.expected.size }
func (p Publication) Manifest() [32]byte                   { return p.manifest }
func (publication Publication) Scan() p.ScanRecord         { return publication.scan }
func (publication Publication) Artifact() p.ArtifactRecord { return publication.artifact }
func (p Publication) String() string                       { return "dependency publication (redacted)" }
func (p Publication) GoString() string                     { return p.String() }

type Receipt struct {
	publication Publication
	disposition string
}

func (r Receipt) Publication() Publication { return r.publication }
func (r Receipt) Disposition() string      { return r.disposition }
func (r Receipt) String() string           { return "dependency receipt (redacted)" }
func (r Receipt) GoString() string         { return r.String() }

type Reconciliation struct {
	state       string
	publication Publication
}

func (r Reconciliation) State() string            { return r.state }
func (r Reconciliation) Publication() Publication { return r.publication }
func (r Reconciliation) String() string           { return "dependency reconciliation (redacted)" }
func (r Reconciliation) GoString() string         { return r.String() }

type ExportReceipt struct {
	digest [32]byte
	size   uint64
}

func (r ExportReceipt) Digest() [32]byte { return r.digest }
func (r ExportReceipt) Size() uint64     { return r.size }
