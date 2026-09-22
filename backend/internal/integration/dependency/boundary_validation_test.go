package dependency

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	app "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	config "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
)

func realFaultService(t *testing.T) (*Service, *app.Runtime, die.DependencyInventory) {
	t.Helper()
	if os.Getenv("AEGIS_DEPENDENCY_DISPOSABLE") != "1" {
		t.Skip("disposable harness only")
	}
	db := os.Getenv("AEGIS_RUNTIME_POSTGRES_DATABASE")
	if !strings.HasPrefix(db, "aegis_die505_") {
		t.Fatal("not disposable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rt, e := app.NewDefaultStarter().Start(ctx, config.NewLoadRequest(config.LoadRequestParams{Environment: []string{"AEGIS_PROFILE=ci", "AEGIS_DATABASE_HOST=127.0.0.1", "AEGIS_DATABASE_PORT=" + os.Getenv("AEGIS_RUNTIME_POSTGRES_PORT"), "AEGIS_DATABASE_NAME=" + db, "AEGIS_DATABASE_USER=" + os.Getenv("AEGIS_RUNTIME_POSTGRES_USER")}, SecretProvider: validationSecrets{}}))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		out, e := rt.Shutdown(c)
		if e != nil || !out.ResourcesClosed() {
			t.Error("shutdown", e)
		}
	})
	c, _ := p.New()
	f, _ := NewSpoolFactory(t.TempDir(), 0, 0)
	cfg, _ := NewConfig([]Route{{scopeID, repoID}}, 0)
	s, e := FromRuntime(rt, c, f, cfg)
	if e != nil {
		t.Fatal(e)
	}
	scope, _ := p.NewScope(scopeID, "disposable-validation")
	actor, _ := p.NewAuditActor("principal", scope.PrincipalID())
	source, _ := p.NewSourceIdentity("local", "sha256/v1", p.DigestBytes([]byte("disposable")))
	reg, e := c.NewRegisterRepositoryRequest(p.RegisterRepositoryParams{Scope: scope, RequestID: "fault-register", RepositoryID: p.RepositoryID(repoID), DisplayName: "Fault fixture", Source: source, Actor: actor})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = rt.Ingest().RegisterRepository(ctx, reg); e != nil {
		t.Fatal(e)
	}
	dc, _ := die.NewConfig(die.ConfigParams{})
	core, _ := die.New(dc)
	v, e := core.Normalize(ctx, die.GraphInput{})
	if e != nil {
		t.Fatal(e)
	}
	return s, rt, v
}

type boundaryScans struct {
	p.ScanStore
	hit func(string)
}

func (w boundaryScans) BeginScan(c context.Context, r p.BeginScanRequest) (p.ScanRecord, error) {
	w.hit("before_begin")
	v, e := w.ScanStore.BeginScan(c, r)
	if e == nil {
		w.hit("after_begin")
	}
	return v, e
}

type boundaryStage struct {
	p.PayloadStager
	hit func(string)
}

func (w boundaryStage) StagePayload(c context.Context, r p.StagePayloadRequest, b io.Reader) (p.PayloadReceipt, error) {
	w.hit("before_stage")
	v, e := w.PayloadStager.StagePayload(c, r, &boundaryReader{Reader: b, hit: w.hit})
	if e == nil {
		w.hit("after_stage")
	}
	return v, e
}

type boundaryReader struct {
	io.Reader
	hit  func(string)
	done bool
}

func (r *boundaryReader) Read(b []byte) (int, error) {
	n, e := r.Reader.Read(b)
	if n > 0 && !r.done {
		r.done = true
		r.hit("during_stage")
	}
	return n, e
}

type boundaryPublication struct {
	p.PublicationStore
	hit func(string)
}

func (w boundaryPublication) PublishScan(c context.Context, r p.PublishScanRequest) (p.PublicationReceipt, error) {
	w.hit("before_publish")
	v, e := w.PublicationStore.PublishScan(c, r)
	if e == nil {
		w.hit("after_publish")
	}
	return v, e
}
func installBoundary(s *Service, hit func(string)) {
	s.d.Scans = boundaryScans{s.d.Scans, hit}
	s.d.Stager = boundaryStage{s.d.Stager, hit}
	s.d.Publications = boundaryPublication{s.d.Publications, hit}
}
func faultRequest(t *testing.T, id int) PublishRequest {
	t.Helper()
	scope, _ := p.NewScope(scopeID, "disposable-validation")
	q, e := NewQuery(scope, repoID, fmt.Sprintf("33333333-3333-4333-8333-%012d", id))
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewPublishRequest(q, fmt.Sprintf("fault-%d", id), "fault-v1")
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestBoundaryProcessChild(t *testing.T) {
	point := os.Getenv("AEGIS_FAULT_CHILD")
	if point == "" {
		t.Skip("child only")
	}
	s, _, v := realFaultService(t)
	id := 0
	if _, e := fmt.Sscan(os.Getenv("AEGIS_FAULT_ID"), &id); e != nil {
		t.Fatal(e)
	}
	// Parent-owned spool allows deterministic orphan cleanup after kill.
	f, e := NewSpoolFactory(os.Getenv("AEGIS_FAULT_CHILD_SPOOL"), 1, 1<<20)
	if e != nil {
		t.Fatal(e)
	}
	s.d.Spools = f
	hit := func(p string) {
		if p == point {
			fmt.Println("FAULT_BOUNDARY_REACHED")
			for {
				time.Sleep(time.Second)
			}
		}
	}
	installBoundary(s, hit)
	hit("before_encode")
	_, e = s.Publish(context.Background(), faultRequest(t, id), v)
	t.Fatalf("boundary not reached: %v", e)
}

func TestRealBoundaryMatrix(t *testing.T) {
	points := []string{"before_encode", "before_begin", "after_begin", "before_stage", "during_stage", "after_stage", "before_publish", "after_publish"}
	s, _, v := realFaultService(t)
	for i, point := range points {
		t.Run("terminate_"+point, func(t *testing.T) {
			id := 100 + i
			ctx, stop := context.WithTimeout(context.Background(), 45*time.Second)
			defer stop()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBoundaryProcessChild$", "-test.v")
			cmd.Env = append(os.Environ(), "AEGIS_FAULT_CHILD="+point, fmt.Sprintf("AEGIS_FAULT_ID=%d", id), "AEGIS_FAULT_CHILD_SPOOL="+t.TempDir())
			stdout, e := cmd.StdoutPipe()
			if e != nil {
				t.Fatal(e)
			}
			if e = cmd.Start(); e != nil {
				t.Fatal(e)
			}
			scanner := bufio.NewScanner(stdout)
			reached := false
			for scanner.Scan() {
				if scanner.Text() == "FAULT_BOUNDARY_REACHED" {
					reached = true
					break
				}
			}
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			if !reached {
				t.Fatal("child did not reach boundary")
			}
			r := faultRequest(t, id)
			out, e := s.Get(context.Background(), r.query)
			if point == "after_publish" {
				if e != nil || out.Size() != 614 {
					t.Fatal("durable commit lost", e)
				}
			} else {
				if e == nil {
					t.Fatal("partial scan became visible")
				}
				if e != Missing && e != Conflict {
					t.Fatal(e)
				}
			}
			t.Log("terminated observed boundary; no deferred child cleanup executed")
		})
		t.Run("cancel_"+point, func(t *testing.T) {
			local, _, _ := realFaultService(t)
			ctx, stop := context.WithCancel(context.Background())
			defer stop()
			hit := false
			installBoundary(local, func(p string) {
				if p == point {
					hit = true
					stop()
				}
			})
			if point == "before_encode" {
				hit = true
				stop()
			}
			r := faultRequest(t, 200+i)
			out, e := local.Publish(ctx, r, v)
			if !hit {
				t.Fatal("boundary missed")
			}
			if point == "after_publish" {
				if e != nil || out.Publication().Size() != 614 {
					t.Fatal("postcommit reconciliation", e)
				}
			} else {
				if e == nil {
					t.Fatal("precommit cancellation succeeded")
				}
			}
			durable, de := s.Get(context.Background(), r.query)
			if point == "after_publish" {
				if de != nil || durable.Size() != 614 {
					t.Fatal(de)
				}
			} else if de == nil {
				t.Fatal("canceled scan published")
			}
		})
	}
}
