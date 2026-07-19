package rie

import (
	"errors"
	"testing"
)

type testArtifact struct {
	name    string
	version string
}

func (artifact testArtifact) ArtifactName() string    { return artifact.name }
func (artifact testArtifact) ArtifactVersion() string { return artifact.version }

func TestArtifactStorePublishesTypedVersionedArtifact(t *testing.T) {
	t.Parallel()

	store := NewArtifactStore()
	want := testArtifact{name: "test", version: "1.0.0"}
	if err := store.Put(want); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, exists := ArtifactAs[testArtifact](store, "test")
	if !exists || got != want {
		t.Errorf("ArtifactAs() = %#v, %v", got, exists)
	}
	if err := store.Put(want); !errors.Is(err, ErrArtifactAlreadyExists) {
		t.Errorf("duplicate Put() error = %v", err)
	}
}

func TestArtifactStoreReset(t *testing.T) {
	t.Parallel()

	store := NewArtifactStore()
	if err := store.Put(testArtifact{name: "test", version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	store.Reset()
	if _, exists := store.Get("test"); exists {
		t.Error("artifact still exists after Reset()")
	}
}
