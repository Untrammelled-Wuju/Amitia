package task_runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestTaskProcessBudgetAtomicallyBoundsConcurrentReservationsAndReleasesOnce(t *testing.T) {
	config := DefaultTaskRuntimeConfig()
	config.PerExtensionMaxConcurrent, config.PerDefinitionMaxConcurrent = 4, 4
	service := NewTaskRuntimeService(nil, config)
	definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension"}
	results := make(chan func(), 32)
	var workers sync.WaitGroup
	for index := 0; index < 32; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			release, err := service.reserveTaskProcess(t.Context(), definition)
			if err != nil && !errors.Is(err, ErrTaskProcessCapacity) {
				t.Error(err)
			}
			if err == nil {
				results <- release
			}
		}()
	}
	workers.Wait()
	close(results)
	var releases []func()
	for release := range results {
		releases = append(releases, release)
	}
	t.Cleanup(func() {
		for _, release := range releases {
			release()
		}
	})
	if len(releases) != 4 {
		t.Fatalf("physical process budget allowed %d processes", len(releases))
	}
	for _, release := range releases {
		for index := 0; index < 2; index++ {
			workers.Add(1)
			go func(release func()) { defer workers.Done(); release() }(release)
		}
	}
	workers.Wait()
	if service.processBudget.total != 0 || len(service.processBudget.extensions) != 0 || len(service.processBudget.definitions) != 0 {
		t.Fatal("physical execution reservations leaked or released twice")
	}
}

func TestTaskProcessBudgetUsesExtensionDefinitionDeclarationCancellationAndShutdown(t *testing.T) {
	config := DefaultTaskRuntimeConfig()
	config.GlobalMaxConcurrent, config.PerDefinitionMaxConcurrent = 8, 2
	service := NewTaskRuntimeService(nil, config)
	definition := &TaskDefinition{TaskID: "task", ExtensionID: "extension", ResourceLimits: TaskResourceLimits{MaxConcurrentTasks: 1}}
	release, err := service.reserveTaskProcess(t.Context(), definition)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if _, err := service.reserveTaskProcess(t.Context(), definition); !errors.Is(err, ErrTaskProcessCapacity) {
		t.Fatalf("task declaration ignored: %v", err)
	}
	second := &TaskDefinition{TaskID: "other", ExtensionID: "extension"}
	releaseSecond, err := service.reserveTaskProcess(t.Context(), second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(releaseSecond)
	if _, err := service.reserveTaskProcess(t.Context(), &TaskDefinition{TaskID: "third", ExtensionID: "extension"}); !errors.Is(err, ErrTaskProcessCapacity) {
		t.Fatalf("extension capacity ignored: %v", err)
	}
	foreign := &TaskDefinition{TaskID: "task", ExtensionID: "other-extension"}
	releaseForeign, err := service.reserveTaskProcess(t.Context(), foreign)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(releaseForeign)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.reserveTaskProcess(cancelled, foreign); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled task reserved capacity: %v", err)
	}
	service.mu.Lock()
	service.closed = true
	service.mu.Unlock()
	if _, err := service.reserveTaskProcess(t.Context(), foreign); err == nil {
		t.Fatal("shutdown accepted a process")
	}
}
