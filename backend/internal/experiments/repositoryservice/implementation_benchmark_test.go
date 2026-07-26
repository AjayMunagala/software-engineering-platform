package repositoryservice

import (
	"context"
	"io"
	"sync"
	"testing"
)

func BenchmarkArtifactIdentity(b *testing.B) {
	input := identityFixture()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := ArtifactID(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTwentyArtifactManifestIdentity(b *testing.B) {
	input := identityFixture()
	b.ReportAllocs()
	for b.Loop() {
		for index := range 20 {
			input.ArtifactName = "artifact-" + string(rune('a'+index))
			if _, err := ArtifactID(input); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkMaterializeSixteenMiB(b *testing.B) {
	materializer, err := NewMaterializer(Config{SpoolDirectory: b.TempDir(), MaxArtifactBytes: 32 << 20})
	if err != nil {
		b.Fatal(err)
	}
	block := make([]byte, 64*1024)
	b.SetBytes(16 << 20)
	b.ReportAllocs()
	for b.Loop() {
		artifact, materializeErr := materializer.Materialize(context.Background(), identityFixture(), func(_ context.Context, writer io.Writer) error {
			for range (16 << 20) / len(block) {
				if _, writeErr := writer.Write(block); writeErr != nil {
					return writeErr
				}
			}
			return nil
		})
		if materializeErr != nil {
			b.Fatal(materializeErr)
		}
		if _, verifyErr := VerifyAndCopy(context.Background(), artifact, io.Discard); verifyErr != nil {
			b.Fatal(verifyErr)
		}
		if closeErr := artifact.Close(context.Background()); closeErr != nil {
			b.Fatal(closeErr)
		}
	}
}

func BenchmarkSingleFlightOneHundredCallers(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var group FlightGroup[string]
		start := make(chan struct{})
		var wait sync.WaitGroup
		for range 100 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				_, _, _ = group.Do(context.Background(), "scan", func(context.Context) (string, error) {
					return "done", nil
				})
			}()
		}
		close(start)
		wait.Wait()
	}
}
