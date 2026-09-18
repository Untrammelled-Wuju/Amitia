package storage

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func TestConcurrentResolveAndEnsure(t *testing.T) {
	dm, _ := newTestDirManager(t)
	const goroutines = 50

	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := domain.PluginID("com.example.concurrent." + strconv.Itoa(idx))
			paths, err := dm.ResolvePluginPaths(id)
			if err != nil {
				errs[idx] = err
				return
			}
			if paths.Root == "" {
				errs[idx] = errEmptyResult
			}
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: unexpected error: %v", i, err)
		}
	}
}

func TestConcurrentEnsurePluginPaths(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()
	pluginID := domain.PluginID("com.example.concurrent-ensure")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := dm.EnsurePluginPaths(ctx, pluginID)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	paths, err := dm.ResolvePluginPaths(pluginID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(paths.Data); err != nil {
		t.Fatalf("data dir should exist: %v", err)
	}
}

func TestConcurrentMultipleRuntimes(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()
	const runtimes = 30

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]string)

	for i := 0; i < runtimes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := domain.RuntimeInstanceID("runtime-" + strconv.Itoa(idx))
			paths, err := dm.EnsureRuntimePaths(ctx, id)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			mu.Lock()
			results[string(id)] = paths.Root
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	seen := make(map[string]struct{})
	for id, root := range results {
		if _, exists := seen[root]; exists {
			t.Fatalf("duplicate runtime root %s for id %s", root, id)
		}
		seen[root] = struct{}{}
	}
}

func TestConcurrentEnsureRuntimeAndService(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()
	rtID := domain.RuntimeInstanceID("runtime-svc-concurrent")
	svcID := domain.ServiceID("svc-a")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := dm.EnsureRuntimePaths(ctx, rtID)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			_, err := dm.EnsureServicePaths(ctx, rtID, svcID)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	rtPaths, err := dm.ResolveRuntimePaths(rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(rtPaths.Root); err != nil {
		t.Fatalf("runtime root should exist: %v", err)
	}

	svcPaths, err := dm.ResolveServicePaths(rtID, svcID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(svcPaths.Root); err != nil {
		t.Fatalf("service root should exist: %v", err)
	}
}

func TestConcurrentContextBuild(t *testing.T) {
	dm, _ := newTestDirManager(t)

	pluginID := domain.PluginID("com.example.ctx-concurrent")
	rtID := domain.RuntimeInstanceID("runtime-ctx-concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := dm.BuildRuntimeContext(pluginID, rtID)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
}

var errEmptyResult = &concurrentTestError{"empty result"}

type concurrentTestError struct {
	msg string
}

func (e *concurrentTestError) Error() string {
	return e.msg
}

func TestRemoveRuntime_ConcurrentAccess(t *testing.T) {
	dm, _ := newTestDirManager(t)
	ctx := context.Background()

	rtID := domain.RuntimeInstanceID("runtime-remove-concurrent")
	_, err := dm.EnsureRuntimePaths(ctx, rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = dm.RemoveRuntime(ctx, rtID)
		}()
	}
	wg.Wait()

	paths, err := dm.ResolveRuntimePaths(rtID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(paths.Root); !os.IsNotExist(err) {
		t.Fatal("runtime root should be removed")
	}
}

func TestConcurrentResolveSameRuntime(t *testing.T) {
	dm, _ := newTestDirManager(t)

	rtID := domain.RuntimeInstanceID("runtime-resolve-same")
	var wg sync.WaitGroup
	results := make([]string, 50)
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			paths, err := dm.ResolveRuntimePaths(rtID)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			mu.Lock()
			results[idx] = paths.Root
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Fatalf("inconsistent runtime paths: %s vs %s", results[i], results[0])
		}
	}
}

