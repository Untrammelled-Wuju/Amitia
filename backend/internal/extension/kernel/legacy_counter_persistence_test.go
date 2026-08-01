package kernel

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLegacyCountersPersistAcrossReinitializationAndProveZeroWindow(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	container, err := NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()
	firstStore := NewLegacyCounterStore(container.Store.DB())
	firstReads := NewLegacyReadCounterWithStore(firstStore)
	firstWrites := NewLegacyCallCounterWithStore(firstStore)
	firstReads.IncPackageReadCalls()
	firstWrites.IncPackageWriteCalls()
	secondStore := NewLegacyCounterStore(container.Store.DB())
	secondReads := NewLegacyReadCounterWithStore(secondStore)
	secondWrites := NewLegacyCallCounterWithStore(secondStore)
	if secondReads.PackageReadCallsFallbacks() != 1 || secondWrites.PackageWriteCalls() != 1 {
		t.Fatalf("persisted counters reset: reads=%d writes=%d", secondReads.PackageReadCallsFallbacks(), secondWrites.PackageWriteCalls())
	}
	secondReads.IncPackageReadCalls()
	thirdStore := NewLegacyCounterStore(container.Store.DB())
	thirdReads := NewLegacyReadCounterWithStore(thirdStore)
	if thirdReads.PackageReadCallsFallbacks() != 2 || thirdReads.Total() != 2 {
		t.Fatalf("restart increment overwrote persisted value: reads=%d total=%d", thirdReads.PackageReadCallsFallbacks(), thirdReads.Total())
	}
	if err := thirdReads.BeginZeroWindow(ctx); err != nil {
		t.Fatal(err)
	}
	restartedStore := NewLegacyCounterStore(container.Store.DB())
	restartedReads := NewLegacyReadCounterWithStore(restartedStore)
	proof, err := restartedReads.ZeroWindowProof(ctx, 0)
	if err != nil || !proof.Passed || !proof.ZeroRead || !proof.ZeroWrite || proof.ReadBaseline != 2 || proof.ReadCurrent != 2 {
		t.Fatalf("zero window proof did not survive restart: %+v err=%v", proof, err)
	}
	NewLegacyCallCounterWithStore(restartedStore).IncPackageWriteCalls()
	proof, err = restartedReads.ZeroWindowProof(ctx, 0)
	if err != nil || proof.Passed || proof.ZeroWrite || !proof.ZeroRead {
		t.Fatalf("zero window did not detect write drift: %+v err=%v", proof, err)
	}
	if err := container.Store.DB().Close(); err != nil {
		t.Fatal(err)
	}
	if proof, err := restartedReads.ZeroWindowProof(ctx, 0); err == nil || proof.Passed {
		t.Fatalf("database failure must invalidate zero window proof: %+v err=%v", proof, err)
	}
}

func TestDefaultBuildLegacyWriteCounterAlwaysZero(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	container, err := NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Close()
	store := NewLegacyCounterStore(container.Store.DB())
	reads := NewLegacyReadCounterWithStore(store)
	writes := NewLegacyCallCounterWithStore(store)
	if reads.PackageReadCallsFallbacks() != 0 {
		t.Fatalf("default build legacy read counter must be 0, got %d", reads.PackageReadCallsFallbacks())
	}
	if writes.PackageWriteCalls() != 0 {
		t.Fatalf("default build legacy write counter must be 0, got %d", writes.PackageWriteCalls())
	}
	if writes.Total() != 0 {
		t.Fatalf("default build legacy call counter total must be 0, got %d", writes.Total())
	}
	restartedStore := NewLegacyCounterStore(container.Store.DB())
	restartedReads := NewLegacyReadCounterWithStore(restartedStore)
	restartedWrites := NewLegacyCallCounterWithStore(restartedStore)
	if restartedReads.PackageReadCallsFallbacks() != 0 {
		t.Fatalf("restarted build legacy read counter must remain 0, got %d", restartedReads.PackageReadCallsFallbacks())
	}
	if restartedWrites.PackageWriteCalls() != 0 {
		t.Fatalf("restarted build legacy write counter must remain 0, got %d", restartedWrites.PackageWriteCalls())
	}
	if err := restartedReads.BeginZeroWindow(ctx); err != nil {
		t.Fatal(err)
	}
	proof, err := restartedReads.ZeroWindowProof(ctx, 0)
	if err != nil || !proof.Passed || !proof.ZeroRead || !proof.ZeroWrite {
		t.Fatalf("default build zero window proof must pass: %+v err=%v", proof, err)
	}
}
