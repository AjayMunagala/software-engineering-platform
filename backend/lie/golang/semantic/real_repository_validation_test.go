package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AjayMunagala/software-engineering-platform/backend/lie"
	golang "github.com/AjayMunagala/software-engineering-platform/backend/lie/golang"
	"github.com/AjayMunagala/software-engineering-platform/backend/lie/golang/packageidentity"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/discovery"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/ignore"
	"github.com/AjayMunagala/software-engineering-platform/backend/rie/language"
)

type realRepositoryValidationResult struct {
	Label                     string                                    `json:"label"`
	Commit                    string                                    `json:"commit"`
	Root                      string                                    `json:"root"`
	Workers                   int                                       `json:"workers"`
	SnapshotEntries           int                                       `json:"snapshot_entries"`
	Syntax                    golang.ParseStatistics                    `json:"syntax"`
	PackageIdentity           packageidentity.PackageIdentityStatistics `json:"package_identity"`
	ProofsByKind              map[string]int                            `json:"proofs_by_kind"`
	ProofConflicts            int                                       `json:"proof_conflicts"`
	Semantic                  SemanticStatistics                        `json:"semantic"`
	ReceiverBindingsByStatus  map[string]int                            `json:"receiver_bindings_by_status"`
	TypeRelationsByStatus     map[string]int                            `json:"type_relations_by_status"`
	InterfaceChecksByStatus   map[string]int                            `json:"interface_checks_by_status"`
	SnapshotDiagnostics       int                                       `json:"snapshot_diagnostics"`
	SyntaxDiagnostics         int                                       `json:"syntax_diagnostics"`
	IdentityDiagnostics       int                                       `json:"identity_diagnostics"`
	SemanticDiagnostics       int                                       `json:"semantic_diagnostics"`
	SemanticDiagnosticCodes   map[string]int                            `json:"semantic_diagnostic_codes"`
	SemanticDiagnosticSamples []lie.Diagnostic                          `json:"semantic_diagnostic_samples"`
	PipelineMilliseconds      float64                                   `json:"pipeline_milliseconds"`
	SemanticFirstMilliseconds float64                                   `json:"semantic_first_milliseconds"`
	SemanticWarmMilliseconds  float64                                   `json:"semantic_warm_milliseconds"`
	PeakHeapBytes             uint64                                    `json:"peak_heap_bytes"`
	AllocatedBytes            uint64                                    `json:"allocated_bytes"`
	Allocations               uint64                                    `json:"allocations"`
	DeterministicRepeat       bool                                      `json:"deterministic_repeat"`
	DeterministicWorkers      bool                                      `json:"deterministic_workers"`
	RepeatCheckSkipped        bool                                      `json:"repeat_check_skipped"`
	WorkerCheckSkipped        bool                                      `json:"worker_check_skipped"`
	ArtifactSHA256            string                                    `json:"artifact_sha256"`
	CancellationRequested     bool                                      `json:"cancellation_requested"`
	CancellationObserved      bool                                      `json:"cancellation_observed"`
	CancellationLatencyMS     float64                                   `json:"cancellation_latency_ms,omitempty"`
}

// TestRealRepositoryValidation is an opt-in, reproducible Phase 2.2.8 harness.
// It never downloads dependencies or executes commands in the target repository.
func TestRealRepositoryValidation(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_ROOT"))
	if root == "" {
		t.Skip("set SEMANTIC_VALIDATION_ROOT to run Phase 2.2.8 validation")
	}
	label := strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_LABEL"))
	if label == "" {
		label = "repository"
	}
	commit := strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_COMMIT"))

	pipelineStarted := time.Now()
	run := rie.NewRunContext(root, rie.DefaultConfig())
	pipeline := rie.New()
	for _, engine := range []rie.Engine{discovery.New(), ignore.New(), language.New()} {
		if err := pipeline.Register(engine); err != nil {
			t.Fatal(err)
		}
	}
	if err := pipeline.Run(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	goEngine, err := golang.New()
	if err != nil {
		t.Fatal(err)
	}
	languageRunner, err := lie.New(goEngine)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := languageRunner.Run(context.Background(), run.Artifacts); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := rie.ArtifactAs[rie.RepositorySnapshot](run.Artifacts, rie.RepositorySnapshotArtifactName)
	if !ok {
		t.Fatal("RepositorySnapshot unavailable")
	}
	syntax, ok := golang.InventoryFrom(run.Artifacts)
	if !ok {
		t.Fatal("GoLanguageInventory unavailable")
	}
	identityEngine, err := packageidentity.New()
	if err != nil {
		t.Fatal(err)
	}
	identities, err := identityEngine.Analyze(context.Background(), packageidentity.Input{Snapshot: snapshot, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Artifacts.Put(identities); err != nil {
		t.Fatal(err)
	}
	pipelineDuration := time.Since(pipelineStarted)
	if stalePath := strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_STALE_PATH")); stalePath != "" {
		cleaned := filepath.Clean(filepath.FromSlash(stalePath))
		if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			t.Fatalf("stale fixture path escapes repository: %q", stalePath)
		}
		absolute := filepath.Join(root, cleaned)
		original, readErr := os.ReadFile(absolute)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(absolute, append(append([]byte(nil), original...), []byte("\n// controlled stale mutation\n")...), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
		defer func() {
			if restoreErr := os.WriteFile(absolute, original, 0o600); restoreErr != nil {
				t.Errorf("restore stale fixture: %v", restoreErr)
			}
		}()
	}

	config := DefaultConfig()
	candidate, err := NewIntegrator(config)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	peakDone := make(chan struct{})
	peakResult := make(chan uint64, 1)
	go samplePeakHeap(peakDone, peakResult)
	firstStarted := time.Now()
	first, err := candidate.Run(context.Background(), run.Artifacts)
	firstDuration := time.Since(firstStarted)
	close(peakDone)
	peakHeap := <-peakResult
	if err != nil {
		t.Fatal(err)
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	firstStatistics := first.Statistics()
	firstReceivers := receiverStatuses(first)
	firstTypeRelations := typeRelationStatuses(first)
	firstDiagnostics := first.Diagnostics()
	firstDigest := inventoryDigest(t, first)
	first = GoSemanticInventory{}
	run.Artifacts = nil
	runtime.GC()

	repeatCheckSkipped := strings.EqualFold(strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_SKIP_REPEAT")), "true")
	warmDigest := firstDigest
	warmDuration := time.Duration(0)
	if !repeatCheckSkipped {
		warmStore := semanticArtifactStore(t, Input{Snapshot: snapshot, Syntax: syntax, PackageIdentities: identities})
		warmStarted := time.Now()
		warm, warmErr := candidate.Run(context.Background(), warmStore)
		warmDuration = time.Since(warmStarted)
		if warmErr != nil {
			t.Fatal(warmErr)
		}
		warmDigest = inventoryDigest(t, warm)
		warm = GoSemanticInventory{}
		warmStore = nil
		runtime.GC()
	}
	workerDigest := firstDigest
	workerCheckSkipped := strings.EqualFold(strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_SKIP_ONE_WORKER")), "true")
	if !workerCheckSkipped {
		oneWorker := config
		oneWorker.MaxWorkers = 1
		oneWorkerCandidate, workerErr := NewIntegrator(oneWorker)
		if workerErr != nil {
			t.Fatal(workerErr)
		}
		oneWorkerInventory, workerErr := oneWorkerCandidate.Run(context.Background(), semanticArtifactStore(t, Input{Snapshot: snapshot, Syntax: syntax, PackageIdentities: identities}))
		if workerErr != nil {
			t.Fatal(workerErr)
		}
		workerDigest = inventoryDigest(t, oneWorkerInventory)
	}
	result := realRepositoryValidationResult{
		Label: label, Commit: commit, Root: root, Workers: config.MaxWorkers,
		SnapshotEntries: len(snapshot.Entries()), Syntax: syntax.Statistics(), PackageIdentity: identities.Statistics(),
		ProofsByKind: proofKinds(identities), ProofConflicts: proofConflicts(identities), Semantic: firstStatistics,
		ReceiverBindingsByStatus: firstReceivers, TypeRelationsByStatus: firstTypeRelations, InterfaceChecksByStatus: firstStatistics.InterfaceChecksByStatus,
		SnapshotDiagnostics: len(snapshot.Diagnostics()), SyntaxDiagnostics: len(syntax.Diagnostics()), IdentityDiagnostics: len(identities.Diagnostics()), SemanticDiagnostics: len(firstDiagnostics),
		SemanticDiagnosticCodes: diagnosticCodes(firstDiagnostics), SemanticDiagnosticSamples: diagnosticSamples(firstDiagnostics, 10),
		PipelineMilliseconds: milliseconds(pipelineDuration), SemanticFirstMilliseconds: milliseconds(firstDuration), SemanticWarmMilliseconds: milliseconds(warmDuration),
		PeakHeapBytes: peakHeap, AllocatedBytes: after.TotalAlloc - before.TotalAlloc, Allocations: after.Mallocs - before.Mallocs,
		DeterministicRepeat: firstDigest == warmDigest, DeterministicWorkers: firstDigest == workerDigest, RepeatCheckSkipped: repeatCheckSkipped, WorkerCheckSkipped: workerCheckSkipped, ArtifactSHA256: firstDigest,
	}
	if delayText := strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_CANCEL_AFTER_MS")); delayText != "" {
		delay, parseErr := strconv.Atoi(delayText)
		if parseErr != nil || delay < 0 {
			t.Fatalf("invalid SEMANTIC_VALIDATION_CANCEL_AFTER_MS %q", delayText)
		}
		result.CancellationRequested = true
		result.CancellationObserved, result.CancellationLatencyMS = measureCancellation(t, candidate, Input{Snapshot: snapshot, Syntax: syntax, PackageIdentities: identities}, time.Duration(delay)*time.Millisecond)
	}
	if !result.DeterministicRepeat || !result.DeterministicWorkers {
		t.Fatal("real-repository output is nondeterministic")
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := strings.TrimSpace(os.Getenv("SEMANTIC_VALIDATION_OUTPUT")); output != "" {
		if err := os.WriteFile(output, append(encoded, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(string(encoded))
}

func samplePeakHeap(done <-chan struct{}, result chan<- uint64) {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	var peak uint64
	for {
		var current runtime.MemStats
		runtime.ReadMemStats(&current)
		if current.HeapAlloc > peak {
			peak = current.HeapAlloc
		}
		select {
		case <-done:
			result <- peak
			return
		case <-ticker.C:
		}
	}
}

func measureCancellation(t testing.TB, candidate Integrator, input Input, delay time.Duration) (bool, float64) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	store := semanticArtifactStore(t, input)
	go func() {
		_, err := candidate.Run(ctx, store)
		done <- err
	}()
	time.Sleep(delay)
	cancelledAt := time.Now()
	cancel()
	err := <-done
	if err == nil {
		return false, 0
	}
	if err != context.Canceled {
		t.Fatalf("cancellation returned %v", err)
	}
	return true, milliseconds(time.Since(cancelledAt))
}

func proofKinds(inventory packageidentity.GoPackageIdentityInventory) map[string]int {
	result := map[string]int{}
	for _, proof := range inventory.Proofs() {
		for _, kind := range proof.Kinds {
			result[kind.String()]++
		}
	}
	return result
}

func proofConflicts(inventory packageidentity.GoPackageIdentityInventory) int {
	count := 0
	for _, proof := range inventory.Proofs() {
		if proof.Status == packageidentity.ProofAmbiguous || len(proof.CandidatePackageIDs) > 1 {
			count++
		}
	}
	return count
}

func receiverStatuses(inventory GoSemanticInventory) map[string]int {
	result := map[string]int{}
	for _, binding := range inventory.ReceiverBindings() {
		result[binding.Status.String()]++
	}
	return result
}

func typeRelationStatuses(inventory GoSemanticInventory) map[string]int {
	result := map[string]int{}
	for _, relation := range inventory.TypeRelations() {
		result[relation.Status.String()]++
	}
	return result
}

func diagnosticCodes(diagnostics []lie.Diagnostic) map[string]int {
	result := map[string]int{}
	for _, diagnostic := range diagnostics {
		result[diagnostic.Code]++
	}
	return result
}

func diagnosticSamples(diagnostics []lie.Diagnostic, limit int) []lie.Diagnostic {
	if len(diagnostics) < limit {
		limit = len(diagnostics)
	}
	return append([]lie.Diagnostic(nil), diagnostics[:limit]...)
}

func inventoryDigest(t testing.TB, inventory GoSemanticInventory) string {
	t.Helper()
	digest := sha256.New()
	if err := json.NewEncoder(digest).Encode(inventory); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
