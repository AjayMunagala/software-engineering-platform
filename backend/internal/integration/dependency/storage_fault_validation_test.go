package dependency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/die"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func faultObserver(t *testing.T) *pgx.Conn {
	t.Helper()
	cfg, e := pgx.ParseConfig("postgres://postgres@127.0.0.1/postgres?sslmode=disable")
	if e != nil {
		t.Fatal(e)
	}
	cfg.Database = os.Getenv("AEGIS_RUNTIME_POSTGRES_DATABASE")
	if !strings.HasPrefix(cfg.Database, "aegis_die505_") {
		t.Fatal("not disposable")
	}
	port, e := strconv.ParseUint(os.Getenv("AEGIS_RUNTIME_POSTGRES_PORT"), 10, 16)
	if e != nil || port == 0 {
		t.Fatal("bad port")
	}
	cfg.Port = uint16(port)
	conn, e := pgx.ConnectConfig(context.Background(), cfg)
	if e != nil {
		t.Fatal("observer connect")
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}
func TestRealStorageFaultsAndLargePublication(t *testing.T) {
	s, _, _ := realFaultService(t)
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Minute)
	defer stop()
	observer := faultObserver(t)
	started := time.Now()
	input := die.GraphInput{}
	// Unique names plus a 4-KiB evidence value produce >64 MiB encoded data.
	for i := 0; i < 20000; i++ {
		id := fmt.Sprintf("node-%05d", i)
		source := die.SourceIdentity{ArtifactName: "go-semantic-inventory", ArtifactVersion: "1.0.0", SourceID: id}
		input.Nodes = append(input.Nodes, die.NodeCandidate{Identity: die.NodeIdentity{Kind: die.NodePackage, Language: "Go", QualifiedName: id, Resolution: die.ResolvedLocal}, Name: id, SourceIdentity: source, Evidence: []die.DependencyEvidence{{Source: source, Rule: "fixture", Value: strings.Repeat("x", 4096)}}})
	}
	dc, _ := die.NewConfig(die.ConfigParams{})
	core, _ := die.New(dc)
	inventory, e := core.Normalize(ctx, input)
	if e != nil {
		t.Fatal(e)
	}
	input = die.GraphInput{}
	runtime.GC()
	prerequisite := time.Since(started)
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak atomic.Uint64
	peak.Store(before.HeapAlloc)
	sampleDone := make(chan struct{})
	samplerStopped := make(chan struct{})
	go func() {
		defer close(samplerStopped)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-sampleDone:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peak.Load() {
					peak.Store(m.HeapAlloc)
				}
			}
		}
	}()
	defer func() { close(sampleDone); <-samplerStopped }()
	request := faultRequest(t, 900)
	var spoolBytes int64
	installBoundary(s, func(point string) {
		if point == "before_stage" {
			entries, err := os.ReadDir(s.d.Spools.directory)
			if err != nil {
				t.Error(err)
			}
			for _, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					t.Error(err)
				} else {
					spoolBytes += info.Size()
				}
			}
		}
	})
	started = time.Now()
	published, e := s.Publish(ctx, request, inventory)
	if e != nil {
		t.Fatal(e)
	}
	publishTime := time.Since(started)
	if published.Publication().Size() <= 64<<20 || spoolBytes <= 64<<20 {
		t.Fatal("fixture did not exercise default disk spill")
	}
	started = time.Now()
	hash := sha256.New()
	exported, e := s.Export(ctx, request.query, hash)
	if e != nil {
		t.Fatal(e)
	}
	exportTime := time.Since(started)
	expectedDigest := published.Publication().Digest()
	if !bytes.Equal(hash.Sum(nil), expectedDigest[:]) || exported.Size() != published.Publication().Size() {
		t.Fatal("large exact-byte proof")
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf("large_result bytes=%d sha256=%x spool_bytes=%d prerequisite_ns=%d publish_including_encode_ns=%d export_ns=%d peak_heap_sampled_5ms=%d operation_alloc_bytes=%d", exported.Size(), exported.Digest(), spoolBytes, prerequisite.Nanoseconds(), publishTime.Nanoseconds(), exportTime.Nanoseconds(), peak.Load(), after.TotalAlloc-before.TotalAlloc)
	digest := published.Publication().Digest()
	// Capture only the two mutated chunks, not the artifact-sized payload.
	var first, second []byte
	if e = observer.QueryRow(ctx, "SELECT chunk_bytes FROM platform.artifact_payload_chunks WHERE payload_digest=$1 AND chunk_ordinal=0", digest[:]).Scan(&first); e != nil {
		t.Fatal(e)
	}
	if e = observer.QueryRow(ctx, "SELECT chunk_bytes FROM platform.artifact_payload_chunks WHERE payload_digest=$1 AND chunk_ordinal=1", digest[:]).Scan(&second); e != nil {
		t.Fatal(e)
	}
	restore := func() {
		_, e := observer.Exec(ctx, "INSERT INTO platform.artifact_payload_chunks(payload_digest,chunk_ordinal,chunk_bytes) VALUES($1,0,$2),($1,1,$3) ON CONFLICT(payload_digest,chunk_ordinal) DO UPDATE SET chunk_bytes=EXCLUDED.chunk_bytes", digest[:], first, second)
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, kind := range []string{"corrupt", "truncate", "missing", "reorder"} {
		t.Run(kind, func(t *testing.T) {
			defer restore()
			switch kind {
			case "corrupt":
				b := append([]byte(nil), first...)
				b[0] ^= 1
				_, e = observer.Exec(ctx, "UPDATE platform.artifact_payload_chunks SET chunk_bytes=$2 WHERE payload_digest=$1 AND chunk_ordinal=0", digest[:], b)
			case "truncate":
				_, e = observer.Exec(ctx, "UPDATE platform.artifact_payload_chunks SET chunk_bytes=$2 WHERE payload_digest=$1 AND chunk_ordinal=0", digest[:], first[:len(first)-1])
			case "missing":
				_, e = observer.Exec(ctx, "DELETE FROM platform.artifact_payload_chunks WHERE payload_digest=$1 AND chunk_ordinal=0", digest[:])
			case "reorder":
				_, e = observer.Exec(ctx, "UPDATE platform.artifact_payload_chunks SET chunk_bytes=CASE chunk_ordinal WHEN 0 THEN $2::bytea ELSE $3::bytea END WHERE payload_digest=$1 AND chunk_ordinal IN (0,1)", digest[:], second, first)
			}
			if e != nil {
				t.Fatal(e)
			}
			writer := &countWriter{}
			if _, err := s.Export(ctx, request.query, writer); err == nil || writer.n != 0 {
				t.Fatal("corruption escaped verification", err, writer.n)
			}
		})
	}
	// Mutate each directly stored envelope contract field, restoring committed data.
	fields := []string{"artifact_id", "artifact_name", "artifact_version", "stable_id_scheme", "codec_name", "codec_version", "media_type", "producer_name", "producer_version"}
	for _, field := range fields {
		t.Run("metadata_"+field, func(t *testing.T) {
			var original string
			if e := observer.QueryRow(ctx, "SELECT "+field+"::text FROM platform.artifact_envelopes WHERE scan_id=$1", request.query.scan).Scan(&original); e != nil {
				t.Fatal(e)
			}
			changed := "mismatch"
			if field == "artifact_id" {
				changed = "99999999-9999-4999-8999-999999999999"
			}
			if strings.HasSuffix(field, "version") {
				changed = "9.9.9"
			}
			if field == "media_type" {
				changed = "text/plain"
			}
			if _, e := observer.Exec(ctx, "UPDATE platform.artifact_envelopes SET "+field+"=$2 WHERE scan_id=$1", request.query.scan, changed); e != nil {
				t.Fatal(e)
			}
			defer func() {
				if _, e := observer.Exec(ctx, "UPDATE platform.artifact_envelopes SET "+field+"=$2 WHERE scan_id=$1", request.query.scan, original); e != nil {
					t.Fatal(e)
				}
			}()
			if _, e := s.Reconcile(ctx, published.Publication().Expected()); e == nil {
				t.Fatal("metadata mismatch accepted")
			}
		})
	}
	for _, field := range []string{"payload_size", "payload_digest", "scan_id"} {
		t.Run("constraint_"+field, func(t *testing.T) {
			expression := "payload_size+1"
			if field == "payload_digest" {
				expression = "decode(repeat('ff',32),'hex')"
			}
			if field == "scan_id" {
				expression = "'99999999-9999-4999-8999-999999999999'::uuid"
			}
			if _, e := observer.Exec(ctx, "UPDATE platform.artifact_envelopes SET "+field+"="+expression+" WHERE scan_id=$1", request.query.scan); !foreignKeyFailure(e) {
				t.Fatal("expected referential constraint", e)
			}
		})
	}
	for _, field := range []string{"source_revision", "analysis_profile_digest"} {
		t.Run("metadata_"+field, func(t *testing.T) {
			var original any
			if e := observer.QueryRow(ctx, "SELECT "+field+" FROM platform.repository_scans WHERE scan_id=$1", request.query.scan).Scan(&original); e != nil {
				t.Fatal(e)
			}
			var changed any = "changed-revision"
			if field == "analysis_profile_digest" {
				changed = bytes.Repeat([]byte{0xff}, 32)
			}
			if _, e := observer.Exec(ctx, "UPDATE platform.repository_scans SET "+field+"=$2 WHERE scan_id=$1", request.query.scan, changed); e != nil {
				t.Fatal(e)
			}
			defer func() {
				if _, e := observer.Exec(ctx, "UPDATE platform.repository_scans SET "+field+"=$2 WHERE scan_id=$1", request.query.scan, original); e != nil {
					t.Fatal(e)
				}
			}()
			if _, e := s.Reconcile(ctx, published.Publication().Expected()); e == nil {
				t.Fatal("scan metadata mismatch accepted")
			}
		})
	}
	t.Run("metadata_scope", func(t *testing.T) {
		if _, e := observer.Exec(ctx, "UPDATE platform.repositories SET security_scope_id='99999999-9999-4999-8999-999999999999' WHERE repository_id=$1", repoID); e != nil {
			t.Fatal(e)
		}
		defer func() {
			if _, e := observer.Exec(ctx, "UPDATE platform.repositories SET security_scope_id=$2 WHERE repository_id=$1", repoID, scopeID); e != nil {
				t.Fatal(e)
			}
		}()
		if _, e := s.Get(ctx, request.query); e != Missing {
			t.Fatal("scope mutation leaked", e)
		}
	})
	t.Run("constraint_repository", func(t *testing.T) {
		if _, e := observer.Exec(ctx, "UPDATE platform.repository_scans SET repository_id='99999999-9999-4999-8999-999999999999' WHERE scan_id=$1", request.query.scan); !foreignKeyFailure(e) {
			t.Fatal("repository FK bypass", e)
		}
	})
	// The frozen read port deliberately does not expose the stored certificate.
	// Verify tampering with that certificate through the test observer only.
	var original []byte
	if e := observer.QueryRow(ctx, "SELECT artifact_set_digest FROM platform.scan_publications WHERE scan_id=$1", request.query.scan).Scan(&original); e != nil {
		t.Fatal(e)
	}
	changed := append([]byte(nil), original...)
	changed[0] ^= 1
	if _, e := observer.Exec(ctx, "UPDATE platform.scan_publications SET artifact_set_digest=$2 WHERE scan_id=$1", request.query.scan, changed); e != nil {
		t.Fatal(e)
	}
	var actual []byte
	_ = observer.QueryRow(ctx, "SELECT artifact_set_digest FROM platform.scan_publications WHERE scan_id=$1", request.query.scan).Scan(&actual)
	if bytes.Equal(original, actual) {
		t.Fatal("certificate mutation ineffective")
	}
	if _, e := observer.Exec(ctx, "UPDATE platform.scan_publications SET artifact_set_digest=$2 WHERE scan_id=$1", request.query.scan, original); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Export(ctx, request.query, io.Discard); e != nil {
		t.Fatal("restored payload", e)
	}
}

type countWriter struct{ n int }

func foreignKeyFailure(err error) bool {
	var p *pgconn.PgError
	return errors.As(err, &p) && p.Code == "23503"
}

func (w *countWriter) Write(b []byte) (int, error) { w.n += len(b); return len(b), nil }
