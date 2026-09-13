package dependency

import (
	"context"
	app "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	runtimepostgres "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/postgres"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"sync"
)

type Lease interface {
	Context() context.Context
	Done()
}
type Admission interface {
	Acquire(context.Context) (Lease, error)
}
type Runtime interface {
	Admit(context.Context) (app.Work, error)
	Ingest() runtimepostgres.IngestCapabilities
	Read() runtimepostgres.ReadCapabilities
}
type Capabilities interface {
	p.RepositoryStore
	p.ScanStore
	p.PayloadStager
	p.PublicationStore
	p.ArtifactReader
}
type Dependencies struct {
	Repositories p.RepositoryStore
	Scans        p.ScanStore
	Stager       p.PayloadStager
	Publications p.PublicationStore
	Reader       p.ArtifactReader
	Admission    Admission
	Contract     *p.Contract
	Spools       *SpoolFactory
}
type runtimeAdmission struct{ runtime Runtime }

func (r runtimeAdmission) Acquire(ctx context.Context) (Lease, error) { return r.runtime.Admit(ctx) }
func FromRuntime(runtime Runtime, contract *p.Contract, spools *SpoolFactory, config Config) (*Service, error) {
	if nilValue(runtime) {
		return nil, Invalid
	}
	ingest, read := runtime.Ingest(), runtime.Read()
	return New(Dependencies{ingest, ingest, ingest, ingest, read, runtimeAdmission{runtime}, contract, spools}, config)
}

type Service struct {
	d      Dependencies
	config Config
	mu     sync.Mutex
	busy   bool
}

func (s *Service) String() string   { return "dependency integration (redacted)" }
func (s *Service) GoString() string { return s.String() }

func New(d Dependencies, c Config) (*Service, error) {
	if len(c.routes) == 0 || c.finalization == 0 || d.Contract == nil || d.Spools == nil || d.Spools.memory == 0 || d.Spools.maximum == 0 {
		return nil, Invalid
	}
	for _, v := range []any{d.Repositories, d.Scans, d.Stager, d.Publications, d.Reader, d.Admission} {
		if nilValue(v) {
			return nil, Invalid
		}
	}
	c.routes = append([]Route(nil), c.routes...)
	return &Service{d: d, config: c}, nil
}
func (s *Service) check(ctx context.Context, q Query) error {
	if s == nil || ctx == nil || q.scope.IsZero() {
		return Invalid
	}
	if err := ctx.Err(); err != nil {
		return safe(err)
	}
	for _, r := range s.config.routes {
		if r.ScopeID == q.scope.ScopeID() && r.RepositoryID == q.repository {
			return nil
		}
	}
	return Missing
}
func (s *Service) admit(ctx context.Context) (context.Context, func(), error) {
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return nil, nil, Busy
	}
	s.busy = true
	s.mu.Unlock()
	unlock := func() { s.mu.Lock(); s.busy = false; s.mu.Unlock() }
	lease, err := s.d.Admission.Acquire(ctx)
	if err != nil {
		if !nilValue(lease) {
			lease.Done()
		}
		unlock()
		return nil, nil, safe(err)
	}
	if nilValue(lease) {
		unlock()
		return nil, nil, Internal
	}
	if lease.Context() == nil {
		lease.Done()
		unlock()
		return nil, nil, Internal
	}
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(lease.Context(), cancel)
	if lease.Context().Err() != nil {
		cancel()
	}
	var once sync.Once
	done := func() { once.Do(func() { stop(); cancel(); lease.Done(); unlock() }) }
	return work, done, nil
}
