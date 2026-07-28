package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	testScopeID      = "00000000-0000-4000-8000-000000000001"
	testRepositoryID = "11111111-1111-4111-8111-111111111111"
	testScanID       = "22222222-2222-4222-8222-222222222222"
)

func TestContractDefaultsAndProfilesAreDetached(t *testing.T) {
	contract, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if contract.Config() != DefaultConfig() {
		t.Fatalf("config = %+v", contract.Config())
	}
	definitions := contract.Profiles().Definitions()
	if len(definitions) != 1 || definitions[0].Profile().Name() != "repository-go" || definitions[0].Profile().Version() != "1" || len(definitions[0].Artifacts()) != 10 {
		t.Fatalf("profiles = %+v", definitions)
	}
	artifacts := definitions[0].Artifacts()
	artifacts[0] = ProfileArtifact{}
	definitionsAgain := contract.Profiles().Definitions()
	if definitionsAgain[0].Artifacts()[0].Name() == "" {
		t.Fatal("profile artifacts were mutable")
	}
	if _, ok := contract.Profiles().Resolve("repository-go", "1", definitions[0].Profile().Digest()); !ok {
		t.Fatal("default profile did not resolve")
	}
	wrong := DigestBytes([]byte("wrong"))
	if _, ok := contract.Profiles().Resolve("repository-go", "1", wrong); ok {
		t.Fatal("wrong profile digest resolved")
	}
}

func TestConfigurationValidation(t *testing.T) {
	if _, err := New(DefaultConfig(), DefaultConfig()); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("multiple configs: %v", err)
	}
	cases := []Config{
		{FinalizationTimeout: time.Millisecond},
		{MaxArtifactsPerScan: 65},
		{MaxArtifactBytes: defaultMaxArtifactBytes + 1},
		{MaterializerMemoryBytes: defaultMaterializerMemoryBytes + 1},
		{MaxDiagnostics: defaultMaxDiagnostics + 1},
		{MaxConcurrentScans: defaultMaxConcurrentScans + 1},
		{MaxPageSize: defaultMaxPageSize + 1},
		{MaxDisplayNameBytes: defaultMaxDisplayNameBytes + 1},
		{MaxSourceHandleBytes: defaultMaxSourceHandleBytes + 1},
	}
	for _, config := range cases {
		if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("config %+v: %v", config, err)
		}
	}
}

func TestRequestConstructorsAndSensitiveSourceBoundary(t *testing.T) {
	contract, _ := New()
	scope := mustScope(t, testScopeID, "principal-a")
	profile := contract.Profiles().Definitions()[0].Profile()
	register, err := contract.NewRegisterRepositoryRequest(RegisterRepositoryParams{Scope: scope, RequestID: "request-1", RepositoryID: testRepositoryID, DisplayName: "Example Repository", SourceHandle: "local-source-token"})
	if err != nil {
		t.Fatal(err)
	}
	if register.SourceHandle().Reveal() != "local-source-token" || register.DisplayName() != "Example Repository" {
		t.Fatalf("register = %+v", register)
	}
	if formatted := fmt.Sprintf("%v %#v", register.SourceHandle(), register.SourceHandle()); strings.Contains(formatted, "local-source-token") {
		t.Fatalf("source handle formatting leaked: %s", formatted)
	}
	if formatted := fmt.Sprintf("%+v %#v", register, register); strings.Contains(formatted, "local-source-token") {
		t.Fatalf("request formatting leaked source handle: %s", formatted)
	}
	if register.Scope() != scope || register.RequestID() != "request-1" || register.RepositoryID() != testRepositoryID {
		t.Fatalf("register accessors = %+v", register)
	}
	execute, err := contract.NewExecuteScanRequest(ExecuteScanParams{Scope: scope, RequestID: "request-2", RepositoryID: testRepositoryID, ScanID: testScanID, SourceHandle: "local-source-token", Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	if execute.Profile().Digest() != profile.Digest() || execute.ScanID() != testScanID {
		t.Fatalf("execute = %+v", execute)
	}
	if execute.Scope() != scope || execute.RequestID() != "request-2" || execute.RepositoryID() != testRepositoryID || execute.SourceHandle().Reveal() != "local-source-token" {
		t.Fatalf("execute accessors = %+v", execute)
	}
	if formatted := fmt.Sprintf("%+v %#v", execute, execute); strings.Contains(formatted, "local-source-token") {
		t.Fatalf("execute request formatting leaked source handle: %s", formatted)
	}
	wrongProfile, _ := NewAnalysisProfile(profile.Name(), profile.Version(), DigestBytes([]byte("wrong")))
	if _, err = contract.NewExecuteScanRequest(ExecuteScanParams{Scope: scope, RequestID: "request-2", RepositoryID: testRepositoryID, ScanID: testScanID, SourceHandle: "token", Profile: wrongProfile}); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("wrong profile = %v", err)
	}
	invalids := []RegisterRepositoryParams{
		{},
		{Scope: scope, RequestID: "bad request", RepositoryID: "repo", DisplayName: "name", SourceHandle: "token"},
		{Scope: scope, RequestID: "request", RepositoryID: "repo", DisplayName: " name", SourceHandle: "token"},
		{Scope: scope, RequestID: "request", RepositoryID: "repo", DisplayName: "name", SourceHandle: "token\nsecret"},
	}
	for _, params := range invalids {
		if _, err = contract.NewRegisterRepositoryRequest(params); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("invalid register %+v: %v", params, err)
		}
	}
}

func TestPrimitiveValidationAndAccessors(t *testing.T) {
	digest := DigestBytes([]byte("digest"))
	parsed, err := ParseDigest(digest.String())
	if err != nil || parsed != digest || parsed.IsZero() {
		t.Fatalf("parsed=%v err=%v", parsed, err)
	}
	for _, input := range []string{"bad", strings.Repeat("00", 31)} {
		if _, err = ParseDigest(input); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("invalid digest %q: %v", input, err)
		}
	}
	scope := mustScope(t, testScopeID, "principal-a")
	if scope.ScopeID() != testScopeID || scope.PrincipalID() != "principal-a" {
		t.Fatalf("scope=%+v", scope)
	}
	for _, input := range [][2]string{{"", "principal"}, {"bad scope", "principal"}, {"scope", ""}} {
		if _, err = NewScope(ScopeID(input[0]), PrincipalID(input[1])); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("invalid scope %q: %v", input, err)
		}
	}
	if _, err = NewSourceHandle("", 10); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("empty source handle: %v", err)
	}
	if _, err = NewAnalysisProfile("Bad", "1", digest); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("invalid profile: %v", err)
	}
	profile, err := NewAnalysisProfile("profile", "1", digest)
	if err != nil || profile.Name() != "profile" || profile.Version() != "1" || profile.Digest() != digest {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	var nilContract *Contract
	if nilContract.Config() != (Config{}) || nilContract.Profiles() != nil {
		t.Fatal("nil contract accessors were not safe")
	}
	var nilRegistry *ProfileRegistry
	if nilRegistry.Clone() != nil || len(nilRegistry.Definitions()) != 0 {
		t.Fatal("nil registry accessors were not safe")
	}
}

func TestCanonicalUUIDIdentifierPolicy(t *testing.T) {
	accepted, err := NewScope(testScopeID, "principal")
	if err != nil || accepted.ScopeID() != testScopeID {
		t.Fatalf("accepted UUID: %+v %v", accepted, err)
	}
	rejected := []string{
		"11111111-1111-4111-8111-11111111111",
		"11111111111141118111111111111111",
		"{11111111-1111-4111-8111-111111111111}",
		"11111111-1111-4111-8111-111111111111 ",
		"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaA",
		"00000000-0000-0000-0000-000000000000",
		"11111111-1111-0111-8111-111111111111",
		"11111111-1111-4111-7111-111111111111",
	}
	for _, value := range rejected {
		if _, err := NewScope(ScopeID(value), "principal"); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("accepted invalid UUID %q: %v", value, err)
		}
	}
	contract, _ := New()
	if _, err := contract.NewRegisterRepositoryRequest(RegisterRepositoryParams{Scope: accepted, RequestID: "request", RepositoryID: "repository", DisplayName: "Repository", SourceHandle: "source"}); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("accepted invalid repository ID: %v", err)
	}
	if _, err := contract.NewExecuteScanRequest(ExecuteScanParams{Scope: accepted, RequestID: "request", RepositoryID: testRepositoryID, ScanID: "scan", SourceHandle: "source", Profile: DefaultRepositoryGoProfile().Profile()}); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("accepted invalid scan ID: %v", err)
	}
}

func TestEveryQueryAndMutationConstructor(t *testing.T) {
	contract, _ := New()
	scope := mustScope(t, testScopeID, "principal-a")
	repositoryQuery, err := NewRepositoryQuery(scope, testRepositoryID)
	if err != nil || repositoryQuery.Scope() != scope || repositoryQuery.RepositoryID() != testRepositoryID {
		t.Fatal(err)
	}
	repositoryList, err := contract.NewRepositoryListRequest(RepositoryListParams{Scope: scope, PageSize: 10, Cursor: "2"})
	if err != nil || repositoryList.Scope() != scope || repositoryList.PageSize() != 10 || repositoryList.Cursor() != "2" {
		t.Fatal(err)
	}
	archive, err := NewArchiveRepositoryRequest(ArchiveRepositoryParams{Scope: scope, RequestID: "request-1", RepositoryID: testRepositoryID})
	if err != nil || archive.Scope() != scope || archive.RequestID() != "request-1" || archive.RepositoryID() != testRepositoryID {
		t.Fatal(err)
	}
	scanQuery, err := NewScanQuery(scope, testRepositoryID, testScanID)
	if err != nil || scanQuery.Scope() != scope || scanQuery.RepositoryID() != testRepositoryID || scanQuery.ScanID() != testScanID {
		t.Fatal(err)
	}
	scanList, err := contract.NewScanListRequest(ScanListParams{Scope: scope, RepositoryID: testRepositoryID, PageSize: 10, Cursor: "3"})
	if err != nil || scanList.Scope() != scope || scanList.RepositoryID() != testRepositoryID || scanList.PageSize() != 10 || scanList.Cursor() != "3" {
		t.Fatal(err)
	}
	cancel, err := NewCancelScanRequest(CancelScanParams{Scope: scope, RequestID: "request-2", RepositoryID: testRepositoryID, ScanID: testScanID})
	if err != nil || cancel.Scope() != scope || cancel.RequestID() != "request-2" || cancel.RepositoryID() != testRepositoryID || cancel.ScanID() != testScanID {
		t.Fatal(err)
	}
	query, err := NewArtifactQuery(scope, testRepositoryID, testScanID, "artifact-1")
	if err != nil {
		t.Fatal(err)
	}
	if query.Scope() != scope || query.RepositoryID() != testRepositoryID || query.ScanID() != testScanID || query.ArtifactID() != "artifact-1" {
		t.Fatalf("artifact query=%+v", query)
	}
	artifactList, err := contract.NewArtifactListRequest(ArtifactListParams{Scope: scope, RepositoryID: testRepositoryID, ScanID: testScanID, PageSize: 10, Cursor: "4"})
	if err != nil || artifactList.Scope() != scope || artifactList.RepositoryID() != testRepositoryID || artifactList.ScanID() != testScanID || artifactList.PageSize() != 10 || artifactList.Cursor() != "4" {
		t.Fatal(err)
	}
	export, err := NewExportArtifactRequest(query)
	if err != nil || export.Query().ArtifactID() != query.ArtifactID() {
		t.Fatal(err)
	}
	if _, err = contract.NewRepositoryListRequest(RepositoryListParams{Scope: scope, PageSize: 1001}); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("large page = %v", err)
	}
	if _, err = NewArtifactQuery(Scope{}, "repo", "scan", "artifact"); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("zero scope = %v", err)
	}
	invalid := []func() error{
		func() error { _, e := NewRepositoryQuery(scope, "bad id"); return e },
		func() error { _, e := NewArchiveRepositoryRequest(ArchiveRepositoryParams{}); return e },
		func() error { _, e := NewScanQuery(scope, "repo", ""); return e },
		func() error { _, e := NewCancelScanRequest(CancelScanParams{}); return e },
		func() error { _, e := NewExportArtifactRequest(ArtifactQuery{}); return e },
	}
	for index, candidate := range invalid {
		if err := candidate(); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("invalid constructor %d: %v", index, err)
		}
	}
}

func TestImmutableResponseModelsAndPages(t *testing.T) {
	now := time.Now().UTC()
	profile := DefaultRepositoryGoProfile().Profile()
	digest := DigestBytes([]byte("payload"))
	repository, err := NewRepository(RepositoryParams{RepositoryID: testRepositoryID, DisplayName: "Repository", SourceKind: "local", FingerprintScheme: "sha256/v1", Fingerprint: digest, State: RepositoryActive, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if repository.RepositoryID() != testRepositoryID || repository.DisplayName() != "Repository" || repository.SourceKind() != "local" || repository.FingerprintScheme() != "sha256/v1" || repository.Fingerprint() != digest || repository.State() != RepositoryActive || repository.CurrentScanID() != "" || repository.CreatedAt() != now || repository.UpdatedAt() != now {
		t.Fatalf("repository accessors=%+v", repository)
	}
	scan, err := NewScan(ScanParams{RepositoryID: testRepositoryID, ScanID: testScanID, Profile: profile, SourceRevision: "revision", State: ScanSucceeded, RequestedAt: now, StartedAt: now, FinishedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if scan.RepositoryID() != testRepositoryID || scan.ScanID() != testScanID || scan.Profile().Digest() != profile.Digest() || scan.SourceRevision() != "revision" || scan.State() != ScanSucceeded || scan.ReasonCode() != "" || scan.RequestedAt() != now || scan.StartedAt() != now || scan.FinishedAt() != now {
		t.Fatalf("scan accessors=%+v", scan)
	}
	artifactID, err := NewArtifactID(testRepositoryID, testScanID, "go-semantic-inventory", "1.0.0", "go-semantic-id/v1")
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := NewArtifact(ArtifactParams{ArtifactID: artifactID, ScanID: testScanID, Name: "go-semantic-inventory", Version: "1.0.0", StableIDScheme: "go-semantic-id/v1", CodecName: "canonical-json", CodecVersion: "1.0.0", MediaType: "application/json", PayloadDigest: digest, PayloadSize: 7, ProducerName: "go-semantic", ProducerVersion: "1.0.0", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.ArtifactID() != artifactID || artifact.ScanID() != testScanID || artifact.Name() != "go-semantic-inventory" || artifact.Version() != "1.0.0" || artifact.StableIDScheme() != "go-semantic-id/v1" || artifact.CodecName() != "canonical-json" || artifact.CodecVersion() != "1.0.0" || artifact.MediaType() != "application/json" || artifact.PayloadDigest() != digest || artifact.PayloadSize() != 7 || artifact.ProducerName() != "go-semantic" || artifact.ProducerVersion() != "1.0.0" || artifact.CreatedAt() != now {
		t.Fatalf("artifact accessors=%+v", artifact)
	}
	repositories := []Repository{repository}
	repositoryPage, _ := NewRepositoryPage(repositories, "next")
	repositories[0] = Repository{}
	if repositoryPage.Items()[0].RepositoryID() != testRepositoryID {
		t.Fatal("repository page retained caller slice")
	}
	if repositoryPage.NextCursor() != "next" {
		t.Fatalf("repository cursor=%q", repositoryPage.NextCursor())
	}
	items := repositoryPage.Items()
	items[0] = Repository{}
	if repositoryPage.Items()[0].RepositoryID() != testRepositoryID {
		t.Fatal("repository page exposed internal slice")
	}
	scanPage, _ := NewScanPage([]Scan{scan}, "")
	artifactPage, _ := NewArtifactPage([]Artifact{artifact}, "")
	result, err := NewScanResult(scan, []Artifact{artifact}, DispositionCreated)
	if err != nil {
		t.Fatal(err)
	}
	resultItems := result.Artifacts()
	resultItems[0] = Artifact{}
	if len(scanPage.Items()) != 1 || len(artifactPage.Items()) != 1 || result.Artifacts()[0].ArtifactID() != artifactID {
		t.Fatal("detached response collections failed")
	}
	if scanPage.NextCursor() != "" || artifactPage.NextCursor() != "" || result.Scan().ScanID() != testScanID || result.Disposition() != DispositionCreated {
		t.Fatal("response accessors failed")
	}
	receipt, err := NewExportReceipt(digest, 7)
	if err != nil || receipt.PayloadSize() != 7 {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if receipt.PayloadDigest() != digest {
		t.Fatal("receipt digest accessor failed")
	}
}

func TestResponseValidationRejectsInvalidValues(t *testing.T) {
	now := time.Now().UTC()
	digest := DigestBytes([]byte("payload"))
	profile := DefaultRepositoryGoProfile().Profile()
	invalids := []func() error{
		func() error { _, e := NewRepository(RepositoryParams{}); return e },
		func() error {
			_, e := NewRepository(RepositoryParams{RepositoryID: "repo", DisplayName: "name", SourceKind: "local", FingerprintScheme: "sha256/v1", Fingerprint: digest, State: RepositoryActive, CreatedAt: now, UpdatedAt: now.Add(-time.Second)})
			return e
		},
		func() error { _, e := NewScan(ScanParams{}); return e },
		func() error { _, e := NewArtifact(ArtifactParams{}); return e },
		func() error { _, e := NewRepositoryPage(nil, "bad cursor"); return e },
		func() error { _, e := NewScanPage(nil, "bad cursor"); return e },
		func() error { _, e := NewArtifactPage(nil, "bad cursor"); return e },
		func() error { _, e := NewRepositoryPage([]Repository{{}}, ""); return e },
		func() error { _, e := NewScanPage([]Scan{{}}, ""); return e },
		func() error { _, e := NewArtifactPage([]Artifact{{}}, ""); return e },
		func() error { _, e := NewScanResult(Scan{}, nil, DispositionCreated); return e },
		func() error { _, e := NewExportReceipt(Digest{}, 1); return e },
	}
	for index, candidate := range invalids {
		if err := candidate(); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("invalid response %d: %v", index, err)
		}
	}
	scan, _ := NewScan(ScanParams{RepositoryID: "repo", ScanID: "scan", Profile: profile, State: ScanSucceeded, RequestedAt: now, StartedAt: now, FinishedAt: now})
	otherID, _ := NewArtifactID("repo", "other", "artifact", "1", ArtifactIdentityScheme)
	artifact, _ := NewArtifact(ArtifactParams{ArtifactID: otherID, ScanID: "other", Name: "artifact", Version: "1", StableIDScheme: ArtifactIdentityScheme, CodecName: "canonical-json", CodecVersion: "1", MediaType: "application/json", PayloadDigest: digest, PayloadSize: 1, ProducerName: "producer", ProducerVersion: "1", CreatedAt: now})
	if _, err := NewScanResult(scan, []Artifact{artifact}, DispositionCreated); KindOf(err) != ErrorInvalidInput {
		t.Fatalf("mismatched artifact: %v", err)
	}
}

func TestArtifactIdentityGoldenVector(t *testing.T) {
	canonical, err := CanonicalArtifactIdentity(testRepositoryID, testScanID, "discovery-inventory", "1.0.0", ArtifactIdentityScheme)
	if err != nil {
		t.Fatal(err)
	}
	want := "7265706f7369746f72792d736572766963652d61727469666163742d69642f7631000000002431313131313131312d313131312d343131312d383131312d3131313131313131313131310000002432323232323232322d323232322d343232322d383232322d32323232323232323232323200000013646973636f766572792d696e76656e746f727900000005312e302e30000000217265706f7369746f72792d736572766963652d61727469666163742d69642f7631"
	if hex := strings.ToLower(strings.TrimSpace(fmtHex(canonical))); hex != want {
		t.Fatalf("canonical bytes = %s", hex)
	}
	id, err := NewArtifactID(testRepositoryID, testScanID, "discovery-inventory", "1.0.0", ArtifactIdentityScheme)
	if err != nil || id != "rsaid1_19546f40503fdddd85481edb5cf47f7189874a252c63bddbb6b39e8c9b032886" {
		t.Fatalf("id=%s err=%v", id, err)
	}
}

func TestSafeErrorModel(t *testing.T) {
	raw := errors.New("password=secret SQLSTATE 08006")
	err := NewError(ErrorPersistenceUnavailable, "execute-scan", "database-unavailable", true, raw)
	if strings.Contains(err.Error(), "secret") || KindOf(err) != ErrorPersistenceUnavailable || !IsRetryable(err) {
		t.Fatalf("unsafe error = %v", err)
	}
	failure := err.(*Error)
	if failure.Operation() != "execute-scan" || failure.ReasonCode() != "database-unavailable" || !failure.Retryable() || failure.Kind() != ErrorPersistenceUnavailable || failure.Unwrap() != nil {
		t.Fatalf("error accessors=%+v", failure)
	}
	canceled := NewError(ErrorInternal, "bad operation", "bad reason", true, context.Canceled)
	if KindOf(canceled) != ErrorCanceled || !errors.Is(canceled, context.Canceled) || IsRetryable(canceled) {
		t.Fatalf("canceled = %v", canceled)
	}
	deadline := NewError("unknown", "", "", false, context.DeadlineExceeded)
	if KindOf(deadline) != ErrorTimeout || !errors.Is(deadline, context.DeadlineExceeded) || !IsRetryable(deadline) {
		t.Fatalf("deadline = %v", deadline)
	}
	if KindOf(errors.New("plain")) != ErrorInternal || KindOf(nil) != "" || IsRetryable(errors.New("plain")) {
		t.Fatal("unknown error classification failed")
	}
	var nilError *Error
	if nilError.Error() != "repository-service: internal" || nilError.Kind() != ErrorInternal || nilError.Operation() != "repository-service" || nilError.ReasonCode() != "internal" || nilError.Retryable() || nilError.Unwrap() != nil {
		t.Fatal("nil error accessors were not safe")
	}
}

func TestProfileRegistryRejectsDuplicatesAndDetaches(t *testing.T) {
	artifact, _ := NewProfileArtifact("artifact", "1.0.0", ArtifactIdentityScheme)
	definition, err := NewProfileDefinition("custom", "1", []ProfileArtifact{artifact})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewProfileDefinition("custom", "1", []ProfileArtifact{artifact, artifact}); KindOf(err) != ErrorConflict {
		t.Fatalf("duplicate artifact = %v", err)
	}
	if _, err = NewProfileRegistry(definition, definition); KindOf(err) != ErrorConflict {
		t.Fatalf("duplicate profile = %v", err)
	}
	registry, _ := NewProfileRegistry(definition)
	copyDefinitions := registry.Definitions()
	copyArtifacts := copyDefinitions[0].Artifacts()
	copyArtifacts[0] = ProfileArtifact{}
	if registry.Definitions()[0].Artifacts()[0].Name() != "artifact" {
		t.Fatal("registry state was mutable")
	}
	if copyDefinitions[0].Profile().Name() != "custom" || copyDefinitions[0].Artifacts()[0].Version() != "1.0.0" || copyDefinitions[0].Artifacts()[0].StableIDScheme() != ArtifactIdentityScheme {
		t.Fatal("profile accessors failed")
	}
	secondArtifact, _ := NewProfileArtifact("second", "1", ArtifactIdentityScheme)
	second, _ := NewProfileDefinition("aaa", "1", []ProfileArtifact{secondArtifact})
	sorted, _ := NewProfileRegistry(definition, second)
	if got := sorted.Definitions(); len(got) != 2 || got[0].Profile().Name() != "aaa" {
		t.Fatalf("profile ordering=%+v", got)
	}
	if _, ok := (*ProfileRegistry)(nil).Resolve("x", "1", DigestBytes([]byte("x"))); ok {
		t.Fatal("nil registry resolved profile")
	}
	for _, candidate := range []func() error{
		func() error { _, e := NewProfileArtifact("Bad", "1", ArtifactIdentityScheme); return e },
		func() error { _, e := NewProfileDefinition("custom", "1", nil); return e },
		func() error { _, e := NewProfileRegistry(ProfileDefinition{}); return e },
	} {
		if err := candidate(); KindOf(err) != ErrorInvalidInput {
			t.Fatalf("profile validation: %v", err)
		}
	}
}

func FuzzArtifactIdentityNeverPanics(f *testing.F) {
	f.Add("repo", "scan", "artifact", "1.0.0", "scheme/v1")
	f.Add("", "scan", "artifact", "1", "scheme")
	f.Fuzz(func(t *testing.T, repositoryID, scanID, name, version, scheme string) {
		canonical, err := CanonicalArtifactIdentity(RepositoryID(repositoryID), ScanID(scanID), name, version, scheme)
		if err == nil {
			if len(canonical) == 0 {
				t.Fatal("valid identity produced empty bytes")
			}
			if _, idErr := NewArtifactID(RepositoryID(repositoryID), ScanID(scanID), name, version, scheme); idErr != nil {
				t.Fatalf("canonical accepted but ID failed: %v", idErr)
			}
		}
	})
}

func FuzzSourceHandleValidationNeverPanics(f *testing.F) {
	f.Add("local-token", 1024)
	f.Add("", 1)
	f.Fuzz(func(t *testing.T, value string, maximum int) {
		if maximum < -4096 || maximum > 4096 {
			return
		}
		handle, err := NewSourceHandle(value, maximum)
		if err == nil && handle.IsZero() {
			t.Fatal("accepted source handle is zero")
		}
	})
}

func mustScope(t *testing.T, scopeID, principalID string) Scope {
	t.Helper()
	scope, err := NewScope(ScopeID(scopeID), PrincipalID(principalID))
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func fmtHex(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for index, current := range value {
		result[index*2], result[index*2+1] = digits[current>>4], digits[current&15]
	}
	return string(result)
}
