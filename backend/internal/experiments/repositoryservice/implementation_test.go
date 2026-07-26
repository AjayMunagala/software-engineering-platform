package repositoryservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goengine "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
)

func TestCanonicalIdentityBytesAndIDAreFrozen(t *testing.T) {
	input := identityFixture()
	canonical, err := CanonicalIdentityBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	wantHex := "7265706f7369746f72792d736572766963652d61727469666163742d69642f763100000000087265706f2d303031000000077363616e2d303100000015676f2d73656d616e7469632d696e76656e746f727900000005312e302e3000000011676f2d73656d616e7469632d69642f7631"
	if got := hex.EncodeToString(canonical); got != wantHex {
		t.Fatalf("canonical bytes changed:\n got %s\nwant %s", got, wantHex)
	}
	id, err := ArtifactID(input)
	if err != nil {
		t.Fatal(err)
	}
	if id != "rsaid1_3c55ac33a130d92a42bd4f782ad7868d9310b94e3fbb91cc3ba9abb85df8fce8" {
		t.Fatalf("artifact ID changed: %s", id)
	}
	changed := input
	changed.ScanID = "scan-02"
	other, _ := ArtifactID(changed)
	if other == id {
		t.Fatal("material identity input did not change the ID")
	}
}

func TestCanonicalIdentityRejectsUnsafeFields(t *testing.T) {
	cases := []string{"", " scan", "scan ", "scan\nvalue", strings.Repeat("x", 1025), string([]byte{0xff})}
	for _, value := range cases {
		input := identityFixture()
		input.ScanID = value
		if _, err := ArtifactID(input); !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("value %q: expected invalid identity, got %v", value, err)
		}
	}
}

func TestConfigAndConstructorValidation(t *testing.T) {
	invalid := []Config{
		{SpoolDirectory: t.TempDir(), BufferBytes: 1, MaxArtifactBytes: 1},
		{SpoolDirectory: t.TempDir(), BufferBytes: 5 << 20, MaxArtifactBytes: 1},
		{SpoolDirectory: t.TempDir(), BufferBytes: 4096, MaxArtifactBytes: maximumPayloadBytes + 1},
	}
	for _, config := range invalid {
		if err := config.Validate(); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("config %+v: %v", config, err)
		}
	}
	if _, err := NewMaterializer(DefaultConfig(), DefaultConfig()); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("multiple configs: %v", err)
	}
	materializer, err := NewMaterializer(Config{SpoolDirectory: t.TempDir(), MaxArtifactBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = materializer.Materialize(nil, identityFixture(), EncodeJSON("value")); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil context: %v", err)
	}
	if _, err = materializer.Materialize(context.Background(), identityFixture(), nil); !errors.Is(err, ErrEncodeRequired) {
		t.Fatalf("nil encoder: %v", err)
	}
	invalidIdentity := identityFixture()
	invalidIdentity.ArtifactName = ""
	if _, err = materializer.Materialize(context.Background(), invalidIdentity, EncodeJSON("value")); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("invalid identity: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = materializer.Materialize(canceled, identityFixture(), EncodeJSON("value")); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled materialization: %v", err)
	}
}

func TestMaterializerWritesOnceVerifiesAndCleansUp(t *testing.T) {
	directory := t.TempDir()
	materializer, err := NewMaterializer(Config{SpoolDirectory: directory, MaxArtifactBytes: 4 << 20})
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("artifact-payload-"), 64*1024)
	var calls atomic.Int32
	artifact, err := materializer.Materialize(context.Background(), identityFixture(), func(_ context.Context, writer io.Writer) error {
		calls.Add(1)
		_, writeErr := writer.Write(payload)
		return writeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("encoder calls = %d", calls.Load())
	}
	var staged bytes.Buffer
	descriptor, err := VerifyAndCopy(context.Background(), artifact, &staged)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(payload)
	if descriptor.PayloadSize != uint64(len(payload)) || descriptor.PayloadDigest != hex.EncodeToString(digest[:]) || !bytes.Equal(staged.Bytes(), payload) {
		t.Fatalf("exact-byte proof failed: %+v", descriptor)
	}
	if entries, _ := os.ReadDir(directory); len(entries) != 1 {
		t.Fatalf("spool files before cleanup = %d", len(entries))
	}
	if err := artifact.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Close(context.Background()); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
	if entries, _ := os.ReadDir(directory); len(entries) != 0 {
		t.Fatalf("spool files after cleanup = %d", len(entries))
	}
	if _, err := artifact.Open(context.Background()); !errors.Is(err, ErrArtifactClosed) {
		t.Fatalf("open after cleanup = %v", err)
	}
}

func TestSixtyFourMiBMaterializationKeepsHeapBounded(t *testing.T) {
	materializer, err := NewMaterializer(Config{SpoolDirectory: t.TempDir(), MaxArtifactBytes: 128 << 20})
	if err != nil {
		t.Fatal(err)
	}
	block := make([]byte, 64*1024)
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	artifact, err := materializer.Materialize(context.Background(), identityFixture(), func(_ context.Context, writer io.Writer) error {
		for range (64 << 20) / len(block) {
			if _, writeErr := writer.Write(block); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer artifact.Close(context.Background())
	if _, err = VerifyAndCopy(context.Background(), artifact, io.Discard); err != nil {
		t.Fatal(err)
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 8<<20 {
		t.Fatalf("64-MiB materialization allocated %d bytes on the Go heap", allocated)
	}
	t.Logf("64-MiB exact materialization allocated %d Go heap bytes", allocated)
}

func TestSealedArtifactNilAndVerificationValidation(t *testing.T) {
	var artifact *SealedArtifact
	if descriptor := artifact.Descriptor(); descriptor != (ArtifactDescriptor{}) {
		t.Fatalf("nil descriptor = %+v", descriptor)
	}
	if _, err := artifact.Open(context.Background()); !errors.Is(err, ErrArtifactClosed) {
		t.Fatalf("nil open = %v", err)
	}
	if err := artifact.Close(context.Background()); err != nil {
		t.Fatalf("nil close = %v", err)
	}
	if _, err := VerifyAndCopy(nil, artifact, io.Discard); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil verify context = %v", err)
	}
	if _, err := VerifyAndCopy(context.Background(), artifact, io.Discard); !errors.Is(err, ErrArtifactIntegrity) {
		t.Fatalf("nil artifact = %v", err)
	}
}

func TestMaterializerRejectsLimitCancellationAndForbiddenRoot(t *testing.T) {
	directory := t.TempDir()
	materializer, err := NewMaterializer(Config{SpoolDirectory: directory, MaxArtifactBytes: 32})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = materializer.Materialize(context.Background(), identityFixture(), func(_ context.Context, writer io.Writer) error {
		_, writeErr := writer.Write(bytes.Repeat([]byte("x"), 33))
		return writeErr
	}); !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("large artifact error = %v", err)
	}
	materializer, err = NewMaterializer(Config{SpoolDirectory: directory, MaxArtifactBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(directory, "private", "repository")
	if _, err = materializer.Materialize(context.Background(), identityFixture(), EncodeJSON(map[string]string{"root": root}), root); !errors.Is(err, ErrForbiddenSourceValue) {
		t.Fatalf("forbidden root error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	if _, err = materializer.Materialize(canceled, identityFixture(), func(_ context.Context, writer io.Writer) error {
		if _, writeErr := writer.Write([]byte("first")); writeErr != nil {
			return writeErr
		}
		cancel()
		_, writeErr := writer.Write([]byte("second"))
		return writeErr
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if entries, _ := os.ReadDir(directory); len(entries) != 0 {
		t.Fatalf("failed materialization leaked %d files", len(entries))
	}
}

func TestDurableRIEReportRedactsAndStabilizesRunMetadata(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private-repository")
	report := rie.Report{
		SchemaVersion: "1.0.0",
		Scan:          rie.ScanMetadata{ID: "random", StartedAt: time.Now(), FinishedAt: time.Now().Add(time.Second), DurationMilliseconds: 1000},
		Repository:    rie.Repository{Name: "private-repository", RootPath: root},
		Metadata:      rie.RepositoryMetadataSummary{Name: "private-repository", RootPath: root},
		Warnings:      []rie.Diagnostic{{Message: "could not inspect " + root, Path: filepath.Join(root, "main.go")}},
		Metrics:       rie.Metrics{FilesPerSecond: 123},
	}
	durable := DurableRIEReport(report, root)
	encoded, err := json.Marshal(durable)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(root)) || durable.Repository.RootPath != "" || durable.Metadata.RootPath != "" {
		t.Fatalf("root leaked: %s", encoded)
	}
	if durable.Scan.ID != "" || durable.Scan.DurationMilliseconds != 0 || durable.Metrics.FilesPerSecond != 0 {
		t.Fatalf("ephemeral values survived: %+v", durable.Scan)
	}
	if durable.Warnings[0].Path != "main.go" || !strings.Contains(durable.Warnings[0].Message, "<repository>") {
		t.Fatalf("diagnostic not sanitized: %+v", durable.Warnings[0])
	}
}

func TestForbiddenPatternAcrossReadBoundary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	pattern := "private-root-value"
	payload := append(bytes.Repeat([]byte("x"), 4090), []byte(pattern)...)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	found, err := fileContains(path, []byte(pattern), 4096)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	found, err = fileContains(path, []byte("not-present"), 4096)
	if err != nil || found {
		t.Fatalf("unexpected pattern: found=%v err=%v", found, err)
	}
}

func TestFlightGroupRunsOneExecutionForOneHundredCallers(t *testing.T) {
	var group FlightGroup[string]
	var executions atomic.Int32
	start := make(chan struct{})
	results := make(chan string, 100)
	errorsChannel := make(chan error, 100)
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			value, _, err := group.Do(context.Background(), "repo/scan", func(context.Context) (string, error) {
				executions.Add(1)
				time.Sleep(20 * time.Millisecond)
				return "published", nil
			})
			results <- value
			errorsChannel <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsChannel)
	if executions.Load() != 1 {
		t.Fatalf("executions = %d", executions.Load())
	}
	for err := range errorsChannel {
		if err != nil {
			t.Fatal(err)
		}
	}
	for value := range results {
		if value != "published" {
			t.Fatalf("result = %q", value)
		}
	}
}

func TestFlightGroupCancelsLeaderAfterEveryWaiterLeaves(t *testing.T) {
	var group FlightGroup[string]
	started := make(chan struct{})
	stopped := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := group.Do(ctx, "scan", func(flightContext context.Context) (string, error) {
			close(started)
			<-flightContext.Done()
			close(stopped)
			return "", flightContext.Err()
		})
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter error = %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shared execution was not canceled")
	}
}

func TestFlightGroupKeepsExecutionForRemainingWaiter(t *testing.T) {
	var group FlightGroup[string]
	started := make(chan struct{})
	release := make(chan struct{})
	firstContext, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, _, err := group.Do(firstContext, "shared", func(ctx context.Context) (string, error) {
			close(started)
			select {
			case <-release:
				return "complete", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		})
		firstDone <- err
	}()
	<-started
	secondDone := make(chan struct {
		value string
		err   error
	}, 1)
	go func() {
		value, disposition, err := group.Do(context.Background(), "shared", func(context.Context) (string, error) {
			return "wrong execution", nil
		})
		if disposition != FlightJoined {
			err = fmt.Errorf("disposition = %s", disposition)
		}
		secondDone <- struct {
			value string
			err   error
		}{value, err}
	}()
	time.Sleep(10 * time.Millisecond)
	cancelFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first waiter = %v", err)
	}
	close(release)
	second := <-secondDone
	if second.err != nil || second.value != "complete" {
		t.Fatalf("remaining waiter = %+v", second)
	}
}

func TestFlightAndPublicationValidation(t *testing.T) {
	var group FlightGroup[string]
	if _, _, err := group.Do(nil, "key", func(context.Context) (string, error) { return "", nil }); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil flight context = %v", err)
	}
	if _, _, err := group.Do(context.Background(), "", func(context.Context) (string, error) { return "", nil }); !errors.Is(err, ErrFlightKeyRequired) {
		t.Fatalf("empty flight key = %v", err)
	}
	if _, _, err := group.Do(context.Background(), "key", nil); !errors.Is(err, ErrFlightFuncRequired) {
		t.Fatalf("nil flight function = %v", err)
	}
	outcome, err := ReconcilePublication(context.Background(), "scan", nil, nil)
	if err != nil || !outcome.Published || outcome.Reconciled {
		t.Fatalf("ordinary publication = %+v, %v", outcome, err)
	}
	if _, err = ReconcilePublication(nil, "scan", errors.New("lost"), nil); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil reconciliation context = %v", err)
	}
	if _, err = ReconcilePublication(context.Background(), "", errors.New("lost"), nil); !errors.Is(err, ErrPublicationAmbiguous) {
		t.Fatalf("missing reconciliation proof = %v", err)
	}
	if _, err = ReconcilePublication(context.Background(), "scan", errors.New("lost"), stateReader{err: errors.New("offline")}); !errors.Is(err, ErrPublicationAmbiguous) {
		t.Fatalf("unavailable reconciliation proof = %v", err)
	}
}

func TestReconcilePublicationHandlesLostResponseWithoutRawErrorLeak(t *testing.T) {
	reader := stateReader{state: PublicationSucceeded}
	outcome, err := ReconcilePublication(context.Background(), "scan-01", errors.New("password=secret SQLSTATE 08006"), reader)
	if err != nil || !outcome.Published || !outcome.Reconciled {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	_, err = ReconcilePublication(context.Background(), "scan-01", errors.New("secret"), stateReader{state: PublicationRunning})
	if !errors.Is(err, ErrPublicationAmbiguous) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("ambiguous error = %v", err)
	}
}

func TestReleasedProfileComposesDeterministicallyAndRedactsPath(t *testing.T) {
	repository := t.TempDir()
	writeFixture(t, filepath.Join(repository, "go.mod"), "module example.com/spike\n\ngo 1.26.2\n")
	writeFixture(t, filepath.Join(repository, "main.go"), "package main\n\ntype Runner interface { Run() error }\ntype Service struct{}\nfunc (Service) Run() error { return nil }\nvar _ Runner = Service{}\nfunc main() {}\n")
	first, err := RunReleasedProfile(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunReleasedProfile(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if !first.GoPresent || first.Syntax.ArtifactVersion() != "1.0.0" || first.PackageIdentities.ArtifactVersion() != "1.0.0" || first.Semantics.ArtifactVersion() != "1.0.0" {
		t.Fatalf("released artifacts missing: %+v", first)
	}
	firstSemantic, _ := json.Marshal(first.Semantics)
	secondSemantic, _ := json.Marshal(second.Semantics)
	if !bytes.Equal(firstSemantic, secondSemantic) {
		t.Fatal("semantic artifact bytes are nondeterministic")
	}
	firstReport, _ := json.Marshal(DurableRIEReport(first.Report, repository))
	secondReport, _ := json.Marshal(DurableRIEReport(second.Report, repository))
	if !bytes.Equal(firstReport, secondReport) || bytes.Contains(firstReport, []byte(repository)) {
		t.Fatal("durable RIE report is nondeterministic or leaked the root")
	}
	materializer, _ := NewMaterializer(Config{SpoolDirectory: t.TempDir(), MaxArtifactBytes: 32 << 20})
	artifact, err := materializer.Materialize(context.Background(), identityFixture(), EncodeJSON(first.Semantics), repository)
	if err != nil {
		t.Fatal(err)
	}
	defer artifact.Close(context.Background())
	if _, err = VerifyAndCopy(context.Background(), artifact, io.Discard); err != nil {
		t.Fatal(err)
	}
	view := DetachedGoLanguageView(first.Syntax)
	if len(view.Files) != 1 || view.Artifact.Version != "1.0.0" {
		t.Fatalf("syntax view = %+v", view)
	}
}

func TestNonGoReleasedProfileStopsAfterRIE(t *testing.T) {
	repository := t.TempDir()
	writeFixture(t, filepath.Join(repository, "README.md"), "# fixture\n")
	result, err := RunReleasedProfile(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if result.GoPresent {
		t.Fatal("non-Go repository unexpectedly ran Go analysis")
	}
}

func TestReleasedProfileAndEncodingValidation(t *testing.T) {
	if _, err := RunReleasedProfile(nil, t.TempDir()); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("nil profile context = %v", err)
	}
	if _, err := RunReleasedProfile(context.Background(), filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing repository was accepted")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := EncodeJSON(map[string]string{"value": "safe"})(canceled, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled JSON encoding = %v", err)
	}
	view := DetachedGoLanguageView(goengine.GoLanguageInventory{})
	if view.Files == nil || view.Packages == nil || view.Symbols == nil || view.Diagnostics == nil || view.SourceArtifacts == nil {
		t.Fatal("detached empty collections must be explicit")
	}
}

func identityFixture() IdentityInput {
	return IdentityInput{
		RepositoryID: "repo-001", ScanID: "scan-01", ArtifactName: "go-semantic-inventory",
		ArtifactVersion: "1.0.0", StableIDScheme: "go-semantic-id/v1",
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

type stateReader struct {
	state PublicationState
	err   error
}

func (reader stateReader) ScanState(context.Context, string) (PublicationState, error) {
	return reader.state, reader.err
}

func TestSpoolPermissionsOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix mode semantics")
	}
	materializer, _ := NewMaterializer(Config{SpoolDirectory: t.TempDir(), MaxArtifactBytes: 1024})
	artifact, err := materializer.Materialize(context.Background(), identityFixture(), EncodeJSON(map[string]string{"value": "safe"}))
	if err != nil {
		t.Fatal(err)
	}
	defer artifact.Close(context.Background())
	info, err := os.Stat(artifact.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("spool permissions = %o", info.Mode().Perm())
	}
}

func TestMaterializedPayloadDetectsTampering(t *testing.T) {
	materializer, _ := NewMaterializer(Config{SpoolDirectory: t.TempDir(), MaxArtifactBytes: 1024})
	artifact, err := materializer.Materialize(context.Background(), identityFixture(), EncodeJSON(map[string]string{"value": "safe"}))
	if err != nil {
		t.Fatal(err)
	}
	defer artifact.Close(context.Background())
	if err := os.WriteFile(artifact.path, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifyAndCopy(context.Background(), artifact, io.Discard); !errors.Is(err, ErrArtifactIntegrity) {
		t.Fatalf("tamper error = %v", err)
	}
}

func ExampleArtifactID() {
	id, _ := ArtifactID(identityFixture())
	fmt.Println(id)
	// Output: rsaid1_3c55ac33a130d92a42bd4f782ad7868d9310b94e3fbb91cc3ba9abb85df8fce8
}
