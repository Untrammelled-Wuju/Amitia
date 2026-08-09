package binary

import (
	"context"
	"testing"
)

func newTestResolver(t *testing.T) (*Resolver, ObjectRegistry) {
	t.Helper()
	reg := NewObjectRegistry(Options{})
	providers := NewProviderRegistry()
	prov, err := NewFileProvider(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create file provider: %v", err)
	}
	providers.Register(prov)
	resolver := NewResolver(reg, providers)
	return resolver, reg
}

func TestResolver_CreateAndResolve(t *testing.T) {
	resolver, _ := newTestResolver(t)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}

	handle, err := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{
		ExpectedSize: 5,
		MediaType:    "image/png",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, err := handle.Writer.Write([]byte("hello")); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	ref, err := handle.Seal(5, nil)
	if err != nil {
		t.Fatalf("seal failed: %v", err)
	}

	resolved, err := resolver.Resolve(context.Background(), owner, ref)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if resolved.Reader != nil {
		resolved.Reader.Close()
	}
}

func TestResolver_OwnerMismatch_Rejected(t *testing.T) {
	resolver, _ := newTestResolver(t)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	wrongOwner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "other", ChannelID: "c"}
	_, err := resolver.Resolve(context.Background(), wrongOwner, ref)
	if err == nil {
		t.Fatal("wrong owner should be rejected")
	}
}

func TestResolver_KindMismatch_Rejected(t *testing.T) {
	resolver, _ := newTestResolver(t)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	ref.Kind = BinaryStorageSharedMemory
	_, err := resolver.Resolve(context.Background(), owner, ref)
	if err == nil {
		t.Fatal("kind mismatch should be rejected")
	}
}

func TestResolver_SizeMismatch_Rejected(t *testing.T) {
	resolver, _ := newTestResolver(t)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	ref.Size = 999
	_, err := resolver.Resolve(context.Background(), owner, ref)
	if err == nil {
		t.Fatal("size mismatch should be rejected")
	}
}

func TestResolver_LifetimeMismatch_Rejected(t *testing.T) {
	resolver, _ := newTestResolver(t)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	ref, _ := handle.Seal(5, nil)

	ref.Lifetime = BinaryLifetimeRuntime
	_, err := resolver.Resolve(context.Background(), owner, ref)
	if err == nil {
		t.Fatal("lifetime mismatch should be rejected")
	}
}

func TestResolver_ReleaseByRuntime(t *testing.T) {
	resolver, reg := newTestResolver(t)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	handle.Seal(5, nil)

	if err := resolver.ReleaseByRuntime(context.Background(), "r"); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if reg.CountActive() != 0 {
		t.Fatal("expected 0 active")
	}
}

func TestResolver_Shutdown(t *testing.T) {
	resolver, _ := newTestResolver(t)

	if err := resolver.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestProviderRegistry_RegisterAndResolve(t *testing.T) {
	reg := NewProviderRegistry()
	prov, _ := NewFileProvider(t.TempDir())
	reg.Register(prov)

	found, err := reg.Resolve(BinaryStorageFile)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if found.Kind() != BinaryStorageFile {
		t.Fatal("wrong kind")
	}
}

func TestProviderRegistry_UnknownKind(t *testing.T) {
	reg := NewProviderRegistry()
	_, err := reg.Resolve(BinaryStorageKind("unknown"))
	if err == nil {
		t.Fatal("unknown kind should fail")
	}
}

func TestProviderRegistry_Shutdown(t *testing.T) {
	reg := NewProviderRegistry()
	prov, _ := NewFileProvider(t.TempDir())
	reg.Register(prov)

	if err := reg.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestCleanupManager_ReleaseByRuntime(t *testing.T) {
	resolver, _ := newTestResolver(t)
	cm := NewCleanupManager(resolver)

	owner := BinaryOwner{PluginID: "p", RuntimeID: "r", ServiceID: "s", ChannelID: "c"}
	handle, _ := resolver.Create(context.Background(), owner, BinaryStorageFile, CreateRequest{ExpectedSize: 5})
	handle.Writer.Write([]byte("hello"))
	handle.Seal(5, nil)

	if err := cm.ReleaseByRuntime(context.Background(), "r"); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
}

func TestCleanupManager_Shutdown(t *testing.T) {
	resolver, _ := newTestResolver(t)
	cm := NewCleanupManager(resolver)

	if err := cm.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestSharedMemoryProvider_NilResolverRejected(t *testing.T) {
	_, err := NewSharedMemoryProvider(nil)
	if err == nil {
		t.Fatal("nil resolver should be rejected")
	}
}

func TestDefaultSharedMemoryProvider_Unsupported(t *testing.T) {
	_, err := DefaultSharedMemoryProvider()
	if err == nil {
		t.Fatal("default should be unsupported")
	}
}
