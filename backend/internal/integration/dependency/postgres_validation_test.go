package dependency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	app "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/app"
	config "github.com/AjayMunagala/software-engineering-platform/backend/internal/runtime/config"
	p "github.com/AjayMunagala/software-engineering-platform/backend/persistence"
	"github.com/jackc/pgx/v5"
)

type validationSecrets struct{}

func (validationSecrets) Resolve(ctx context.Context, _ config.SecretReference) ([]byte, error) {
	return []byte("disposable-only"), ctx.Err()
}

// Lost replies are injected only after the real transaction succeeds.
type lostPublicationReply struct{ p.PublicationStore }

func (l lostPublicationReply) PublishScan(ctx context.Context, r p.PublishScanRequest) (p.PublicationReceipt, error) {
	out, err := l.PublicationStore.PublishScan(ctx, r)
	if err != nil {
		return out, err
	}
	return p.PublicationReceipt{}, errors.New("injected reply loss")
}

func TestDisposableDependencyPublication(t *testing.T) {
	if os.Getenv("AEGIS_DEPENDENCY_DISPOSABLE") != "1" {
		t.Skip("requires dedicated disposable harness")
	}
	database := os.Getenv("AEGIS_RUNTIME_POSTGRES_DATABASE")
	if len(database) < 13 || database[:13] != "aegis_die505_" {
		t.Fatal("not a disposable database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	runtime, err := app.NewDefaultStarter().Start(ctx, config.NewLoadRequest(config.LoadRequestParams{Environment: []string{
		"AEGIS_PROFILE=ci", "AEGIS_DATABASE_HOST=127.0.0.1", "AEGIS_DATABASE_PORT=" + os.Getenv("AEGIS_RUNTIME_POSTGRES_PORT"),
		"AEGIS_DATABASE_NAME=" + database, "AEGIS_DATABASE_USER=" + os.Getenv("AEGIS_RUNTIME_POSTGRES_USER")}, SecretProvider: validationSecrets{}}))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		out, e := runtime.Shutdown(c)
		if e != nil || !out.ResourcesClosed() {
			t.Error("runtime cleanup failed", e)
		}
	}()
	contract, _ := p.New()
	scope, _ := p.NewScope(scopeID, "disposable-validation")
	actor, _ := p.NewAuditActor("principal", scope.PrincipalID())
	factory, err := NewSpoolFactory(t.TempDir(), 32, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := NewConfig([]Route{{scopeID, repoID}}, 0)
	service, err := FromRuntime(runtime, contract, factory, cfg)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := NewQuery(scope, repoID, scanID)
	request, _ := NewPublishRequest(q, "validation-publish", "fixture-v1")
	coreConfig, _ := die.NewConfig(die.ConfigParams{})
	core, _ := die.New(coreConfig)
	inventory, err := core.Normalize(ctx, die.GraphInput{})
	if err != nil {
		t.Fatal(err)
	}
	// Independent accepted View serialization, not the production codec.
	expected, err := json.Marshal(inventory.View())
	if err != nil {
		t.Fatal(err)
	}
	expected = append(expected, '\n')
	digest := sha256.Sum256(expected)
	if os.Getenv("AEGIS_DEPENDENCY_RECOVERY") != "1" {
		source, _ := p.NewSourceIdentity("local", "sha256/v1", p.DigestBytes([]byte("disposable")))
		reg, e := contract.NewRegisterRepositoryRequest(p.RegisterRepositoryParams{Scope: scope, RequestID: "register-die", RepositoryID: p.RepositoryID(repoID), DisplayName: "Disposable DIE", Source: source, Actor: actor})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = runtime.Ingest().RegisterRepository(ctx, reg); e != nil {
			t.Fatal(e)
		}
		service.d.Publications = lostPublicationReply{runtime.Ingest()}
		receipt, e := service.Publish(ctx, request, inventory)
		if e != nil {
			t.Fatal(e)
		}
		if receipt.Disposition() != "reconciled" {
			t.Fatal("lost reply not reconciled")
		}
		service.d.Publications = runtime.Ingest()
		if _, e = service.Publish(ctx, request, inventory); e != nil {
			t.Fatal("retry", e)
		}
		// Independent instances sharing real database transaction state.
		cq, _ := NewQuery(scope, repoID, "33333333-3333-4333-8333-333333333334")
		cr, _ := NewPublishRequest(cq, "concurrent", "fixture-v1")
		var wg sync.WaitGroup
		outcomes := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, e := FromRuntime(runtime, contract, factory, cfg)
				if e == nil {
					_, e = s.Publish(ctx, cr, inventory)
				}
				outcomes <- e
			}()
		}
		wg.Wait()
		close(outcomes)
		for e := range outcomes {
			if e != nil && !errors.Is(e, Unknown) && e != Conflict {
				t.Errorf("concurrent outcome: %v", e)
			}
		}
		if _, e := service.Get(ctx, cq); e != nil {
			t.Fatal("concurrent publication missing", e)
		}
		canceled, stop := context.WithCancel(ctx)
		stop()
		if _, e := service.Publish(canceled, request, inventory); e != Canceled {
			t.Fatal("cancellation", e)
		}
	}
	var exported bytes.Buffer
	receipt, err := service.Export(ctx, q, &exported)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, exported.Bytes()) || receipt.Digest() != digest || receipt.Size() != uint64(len(expected)) {
		t.Fatal("exact bytes mismatch")
	}
	ep, _ := NewExpected(q, "fixture-v1", uint64(len(expected)), digest)
	reconciled, err := service.Reconcile(ctx, ep)
	if err != nil || reconciled.State() != "published_exact" {
		t.Fatal("recovery reconciliation", err)
	}
	// Test-only observer: never passed to production integration/runtime.
	observerConfig, err := pgx.ParseConfig("")
	if err != nil {
		t.Fatal("observer config")
	}
	observerConfig.Host = "127.0.0.1"
	observerConfig.Database = database
	observerConfig.User = "postgres"
	observerConfig.Password = ""
	observerConfig.TLSConfig = nil
	observerConfig.Fallbacks = nil
	port, err := strconv.ParseUint(os.Getenv("AEGIS_RUNTIME_POSTGRES_PORT"), 10, 16)
	if err != nil || port == 0 {
		t.Fatal("observer port")
	}
	observerConfig.Port = uint16(port)
	observer, err := pgx.ConnectConfig(ctx, observerConfig)
	if err != nil {
		t.Fatal("observer connect")
	}
	defer observer.Close(context.Background())
	var stored []byte
	var scheme string
	var count int
	err = observer.QueryRow(ctx, "SELECT artifact_set_digest, manifest_scheme, artifact_count FROM platform.scan_publications WHERE repository_id=$1 AND scan_id=$2", repoID, scanID).Scan(&stored, &scheme, &count)
	expectedManifest := reconciled.Publication().Manifest()
	if err != nil || !bytes.Equal(stored, expectedManifest[:]) || scheme != ManifestScheme || count != 1 {
		t.Fatal("stored certificate mismatch")
	}
	wrong := digest
	wrong[0] ^= 1
	bad, _ := NewExpected(q, "fixture-v1", uint64(len(expected)), wrong)
	if _, err = service.Reconcile(ctx, bad); err != Integrity {
		t.Fatal("digest mismatch not rejected", err)
	}
	otherScope, _ := p.NewScope("11111111-1111-4111-8111-111111111112", "disposable-validation")
	other, _ := NewQuery(otherScope, repoID, scanID)
	if _, err = service.Get(ctx, other); err != Missing {
		t.Fatal("scope leak", err)
	}
	if runtime.InFlight() != 0 {
		t.Fatal("admission leak")
	}
	t.Logf("dependency_real_result bytes=%d sha256=%x recovery=%s", len(expected), digest, os.Getenv("AEGIS_DEPENDENCY_RECOVERY"))
	// Nonempty Unicode inventory is rebuilt independently in every process.
	node := die.NodeCandidate{Identity: die.NodeIdentity{Kind: die.NodePackage, Language: "Go", QualifiedName: "example.org/π", RepositoryPath: "pkg", Resolution: die.ResolvedLocal}, Name: "π", SourceIdentity: die.SourceIdentity{ArtifactName: "go-semantic-inventory", ArtifactVersion: "1.0.0", SourceID: "fixture-node"}}
	nonempty, e := core.Normalize(ctx, die.GraphInput{Nodes: []die.NodeCandidate{node}})
	if e != nil {
		t.Fatal(e)
	}
	nq, _ := NewQuery(scope, repoID, "33333333-3333-4333-8333-333333333335")
	nr, _ := NewPublishRequest(nq, "nonempty", "fixture-v1")
	if os.Getenv("AEGIS_DEPENDENCY_RECOVERY") != "1" {
		if _, e = service.Publish(ctx, nr, nonempty); e != nil {
			t.Fatal(e)
		}
	}
	nbytes, _ := json.Marshal(nonempty.View())
	nbytes = append(nbytes, '\n')
	var nb bytes.Buffer
	if _, e = service.Export(ctx, nq, &nb); e != nil || !bytes.Equal(nbytes, nb.Bytes()) {
		t.Fatal("nonempty export", e)
	}
	t.Logf("dependency_nonempty bytes=%d sha256=%x", len(nbytes), sha256.Sum256(nbytes))
	if os.Getenv("AEGIS_DEPENDENCY_RETENTION") == "1" {
		ar, _ := contract.NewArchiveRepositoryRequest(scope, "archive", p.RepositoryID(repoID), actor)
		if _, e = runtime.Ingest().ArchiveRepository(ctx, ar); e != nil {
			t.Fatal(e)
		}
		if _, e = service.Publish(ctx, request, inventory); e != Conflict {
			t.Fatal("archived publication", e)
		}
		mark, _ := contract.NewMarkForPurgeRequest(scope, "mark", p.RepositoryID(repoID), actor)
		if _, e = runtime.Retention().MarkRepositoryForPurge(ctx, mark); e != nil {
			t.Fatal(e)
		}
		purge, _ := contract.NewPurgeBatchRequest(scope, "purge", p.RepositoryID(repoID), 100, actor)
		if _, e = runtime.Retention().PurgeRepositoryBatch(ctx, purge); e != nil {
			t.Fatal(e)
		}
		gc, _ := contract.NewGarbageCollectionRequest(scope, "gc", 100, actor)
		if _, e = runtime.Retention().GarbageCollectPayloads(ctx, gc); e != nil {
			t.Fatal(e)
		}
		if _, e = service.Get(ctx, q); e != Missing {
			t.Fatal("purged publication visible", e)
		}
	}
}
