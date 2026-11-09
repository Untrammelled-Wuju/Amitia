package main

import (
	"errors"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type failingPetxReleaseRepository struct {
	release.ReleaseRepository
	err error
}

func (r failingPetxReleaseRepository) GetRelease(string) (*release.ReleaseData, error) {
	return nil, r.err
}

type unusedPetxReleaseStorage struct {
	release.ReleaseStoragePort
}

func TestPetxAssetReleaseBridgeConcurrentCorrelationAttach(t *testing.T) {
	bridge := newPetxAssetReleaseBridge(nil, nil, nil, nil, nil, "space-a", "device-a")
	const workers = 64

	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	errorsChannel := make(chan error, workers)
	for index := 0; index < workers; index++ {
		go func(index int) {
			defer waitGroup.Done()
			_, err := bridge.AttachPluginResource(
				t.Context(),
				"com.example.pet",
				"resource",
				index,
				map[string]any{"displayName": "Test"},
			)
			errorsChannel <- err
		}(index)
	}
	waitGroup.Wait()
	close(errorsChannel)

	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent correlation attach failed: %v", err)
		}
	}
	bindings, err := bridge.ListAttachedResources(t.Context(), "com.example.pet")
	if err != nil {
		t.Fatalf("list attached resources: %v", err)
	}
	if len(bindings) != workers {
		t.Fatalf("expected %d bindings, got %d", workers, len(bindings))
	}
}

func TestPetxAssetReleaseBridgeDoesNotHideReleaseReadFailure(t *testing.T) {
	readErr := errors.New("release database unavailable")
	bridge := newPetxAssetReleaseBridge(
		nil,
		nil,
		failingPetxReleaseRepository{err: readErr},
		unusedPetxReleaseStorage{},
		nil,
		"space-a",
		"device-a",
	)
	_, err := bridge.ensureRelease(t.Context(), "com.example.pet", t.TempDir(), &packageformat.Manifest{
		ReleaseID:     "release-a",
		PetID:         "pet-a",
		DefaultAction: "idle",
	})
	if !errors.Is(err, readErr) {
		t.Fatalf("expected release read failure, got %v", err)
	}
}
