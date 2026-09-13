package dependency

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSpoolLifecycle(t *testing.T) {
	for _, threshold := range []uint64{4, 1024} {
		t.Run(fmt.Sprint(threshold), func(t *testing.T) {
			f, _ := NewSpoolFactory(t.TempDir(), threshold, 2048)
			s, err := f.create(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.open(context.Background()); err != Conflict {
				t.Fatal(err)
			}
			if _, err = s.Write([]byte("hello world")); err != nil {
				t.Fatal(err)
			}
			if err = s.seal(); err != nil {
				t.Fatal(err)
			}
			if err = s.seal(); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Write(nil); err != Conflict {
				t.Fatal(err)
			}
			if _, err = s.open(nil); err != Invalid {
				t.Fatal(err)
			}
			r, err := s.open(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if err = s.close(); err != Busy {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(r)
			if string(b) != "hello world" {
				t.Fatal("bytes")
			}
			r.Close()
			r.Close()
			if err = s.close(); err != nil {
				t.Fatal(err)
			}
			if err = s.close(); err != nil {
				t.Fatal(err)
			}
			if _, err = s.open(context.Background()); err != Conflict {
				t.Fatal(err)
			}
			if _, err = s.Write(nil); err != Conflict {
				t.Fatal(err)
			}
			if err = s.seal(); err != Conflict {
				t.Fatal(err)
			}
		})
	}
}
func TestSpoolFailures(t *testing.T) {
	// Failed exclusive creation must never make an existing path ours to delete.
	dir := t.TempDir()
	existing := filepath.Join(dir, "preexisting")
	if err := os.WriteFile(existing, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	notOwned := &spool{root: root, name: "preexisting", poison: true}
	if err := notOwned.close(); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(existing); err != nil || string(b) != "keep" {
		t.Fatal("removed unowned file")
	}
	for _, c := range []struct {
		dir  string
		m, n uint64
	}{{"relative", 1, 2}, {t.TempDir(), 0, 1}, {t.TempDir(), 2, 1}, {t.TempDir(), 65 << 20, 4 << 30}, {t.TempDir(), 1, 5 << 30}} {
		if _, err := NewSpoolFactory(c.dir, c.m, c.n); err != Invalid {
			t.Fatal(err)
		}
	}
	f, _ := NewSpoolFactory(t.TempDir(), 2, 3)
	if _, err := f.create(nil); err != Invalid {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.create(ctx); err != Canceled {
		t.Fatal(err)
	}
	s, _ := f.create(context.Background())
	if _, err := s.Write([]byte("1234")); err != Limit {
		t.Fatal(err)
	}
	if err := s.seal(); err != Conflict {
		t.Fatal(err)
	}
	s.close()
	f, _ = NewSpoolFactory(filepath.Join(t.TempDir(), "missing"), 1, 100)
	s, _ = f.create(context.Background())
	if _, err := s.Write([]byte("xx")); err != Unavailable {
		t.Fatal(err)
	}
	s.close()
	f, _ = NewSpoolFactory(t.TempDir(), 1, 100)
	s, _ = f.create(context.Background())
	s.Write([]byte("xx"))
	name := filepath.Join(f.directory, s.name)
	s.seal()
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if _, err := s.open(context.Background()); err != Unavailable {
		t.Fatal(err)
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	f, _ = NewSpoolFactory(t.TempDir(), 1, 100)
	ctx, cancel = context.WithCancel(context.Background())
	s, _ = f.create(ctx)
	cancel()
	if _, err := s.Write(nil); err != Canceled {
		t.Fatal(err)
	}
	if err := s.seal(); err != Canceled {
		t.Fatal(err)
	}
	s.close()
}
func TestModelsAndSafeErrors(t *testing.T) {
	s, _, _, r, _ := fixture(t)
	scope, _ := p.NewScope(scopeID, "tester")
	if _, err := NewQuery(scope, "bad", scanID); err != Invalid {
		t.Fatal(err)
	}
	for _, revision := range []string{"/tmp/private", "C:\\secret", "bad\n", "\xff", strings.Repeat("x", 513)} {
		if _, err := NewPublishRequest(r.query, "x", revision); err != Invalid {
			t.Fatal(err)
		}
	}
	if _, err := NewPublishRequest(Query{}, "x", ""); err != Invalid {
		t.Fatal(err)
	}
	if _, err := NewExpected(r.query, "", 0, [32]byte{}); err != Invalid {
		t.Fatal(err)
	}
	if _, err := NewConfig(nil, 0); err != Invalid {
		t.Fatal(err)
	}
	if _, err := NewConfig([]Route{{scopeID, repoID}, {scopeID, repoID}}, 0); err != Invalid {
		t.Fatal(err)
	}
	if _, err := NewConfig([]Route{{"bad", repoID}}, 0); err != Invalid {
		t.Fatal(err)
	}
	if _, err := NewConfig([]Route{{scopeID, repoID}}, 31*time.Second); err != Invalid {
		t.Fatal(err)
	}
	routes := []Route{{scopeID, repoID}}
	c, _ := NewConfig(routes, 0)
	routes[0].ScopeID = "changed"
	if c.routes[0].ScopeID != scopeID {
		t.Fatal("mutable config")
	}
	if _, err := New(Dependencies{}, c); err != Invalid {
		t.Fatal(err)
	}
	if _, err := New(s.d, Config{}); err != Invalid {
		t.Fatal(err)
	}
	if _, err := FromRuntime(nil, s.d.Contract, s.d.Spools, c); err != Invalid {
		t.Fatal(err)
	}
	for _, value := range []any{r, r.query, ExpectedPublication{}, Publication{}, s.d.Spools} {
		if strings.Contains(fmt.Sprintf("%v %#v", value, value), "fixture-π") {
			t.Fatal("redaction")
		}
	}
	if r.query.Scope() != scope || r.query.RepositoryID() != repoID || r.query.ScanID() != scanID {
		t.Fatal("accessors")
	}
	if safe(errors.New("secret")) != Unavailable || safe(Error("secret")) != Internal || safe(context.Canceled) != Canceled || safe(context.DeadlineExceeded) != Timeout || safe(nil) != nil {
		t.Fatal("safe errors")
	}
	for _, kind := range []p.ErrorKind{p.ErrorNotFound, p.ErrorIntegrityFailure, p.ErrorIdempotencyConflict, p.ErrorPayloadTooLarge, p.ErrorTimeout, p.ErrorCanceled, p.ErrorInvalidInput, p.ErrorUnsupportedVersion} {
		if strings.Contains(safe(p.NewError(kind, "test", false, nil)).Error(), "test") {
			t.Fatal("leak")
		}
	}
	if !Busy.Retryable() || !Unknown.OutcomeUnknown() || Invalid.Retryable() {
		t.Fatal("classification")
	}
}
func TestCancellationBusyAndRecoveryStates(t *testing.T) {
	s, f, a, r, v := fixture(t)
	ctx, done, err := s.admit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.admit(context.Background()); err != Busy {
		t.Fatal(err)
	}
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	done()
	done()
	a.ctx = nil
	if _, _, err = s.admit(context.Background()); err != Internal {
		t.Fatal(err)
	}
	a.ctx = context.Background()
	a.err = errors.New("private")
	if _, _, err = s.admit(context.Background()); err != Unavailable {
		t.Fatal(err)
	}
	a.err = nil
	e, _ := NewExpected(r.query, r.revision, 614, [32]byte{1})
	rr, err := s.Reconcile(context.Background(), e)
	if err != nil || rr.State() != "missing_unknown" {
		t.Fatal(err)
	}
	if _, err = s.Reconcile(context.Background(), ExpectedPublication{query: r.query}); err != Invalid {
		t.Fatal(err)
	}
	f.stageFail = true
	if _, err = s.Publish(context.Background(), r, v); err != Unavailable {
		t.Fatal(err)
	}
	rr, err = s.Reconcile(context.Background(), e)
	if err != nil || rr.State() != "failed" {
		t.Fatal(err)
	}
	if _, err = s.Publish(context.Background(), r, v); err != Conflict {
		t.Fatal(err)
	}
	s, _, a, r, v = fixture(t)
	c, cancel := context.WithCancel(context.Background())
	a.ctx = c
	cancel()
	if _, err = s.Publish(context.Background(), r, v); err != Canceled {
		t.Fatal(err)
	}
	if a.acquired != a.released {
		t.Fatal("lease leak")
	}
}

type wf func([]byte) (int, error)

func (f wf) Write(b []byte) (int, error) { return f(b) }

type rf func([]byte) (int, error)

func (f rf) Read(b []byte) (int, error) { return f(b) }
func TestCopyBoundaries(t *testing.T) {
	for _, w := range []wf{func(b []byte) (int, error) { return 0, nil }, func(b []byte) (int, error) { return -1, nil }, func(b []byte) (int, error) { return len(b) + 1, nil }, func(b []byte) (int, error) { return 0, errors.New("secret") }} {
		if _, err := copyContext(context.Background(), w, strings.NewReader("x")); err == nil {
			t.Fatal("writer")
		}
	}
	for _, r := range []rf{func(b []byte) (int, error) { return 0, nil }, func(b []byte) (int, error) { return 0, errors.New("secret") }} {
		if _, err := copyContext(context.Background(), io.Discard, r); err == nil {
			t.Fatal("reader")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := copyContext(ctx, io.Discard, strings.NewReader("x")); err == nil {
		t.Fatal("context")
	}
}
func TestConcurrentReadsAndShutdownAccounting(t *testing.T) {
	s, _, a, r, v := fixture(t)
	if _, err := s.Publish(context.Background(), r, v); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Get(context.Background(), r.query)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if a.acquired != a.released {
		t.Fatal("accounting")
	}
	var out bytes.Buffer
	if _, err := s.Export(context.Background(), r.query, wf(func(b []byte) (int, error) { return 0, errors.New("secret") })); err != Unavailable {
		t.Fatal(err)
	}
	if _, err := s.Export(context.Background(), r.query, &out); err != nil {
		t.Fatal(err)
	}
}
func FuzzRequestValidation(f *testing.F) {
	f.Add("request", "revision")
	f.Add("bad\n", "/secret")
	f.Fuzz(func(t *testing.T, id, revision string) {
		scope, _ := p.NewScope(scopeID, "tester")
		q, _ := NewQuery(scope, repoID, scanID)
		r, err := NewPublishRequest(q, id, revision)
		if err == nil {
			if strings.Contains(fmt.Sprintf("%#v", r), revision) && len(revision) > 40 {
				t.Fatal("leak")
			}
		}
	})
}
