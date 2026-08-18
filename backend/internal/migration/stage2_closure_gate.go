package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type EvidenceStatus string

const (
	EvidencePASS        EvidenceStatus = "PASS"
	EvidenceNOTVERIFIED EvidenceStatus = "NOT_VERIFIED"
	EvidenceFAIL        EvidenceStatus = "FAIL"
	EvidenceBLOCKED     EvidenceStatus = "BLOCKED"
)

type EvidenceItem struct {
	Status   EvidenceStatus `json:"status"`
	Evidence string         `json:"evidence"`
}

type EvidenceManifest struct {
	Version         string                  `json:"version"`
	ManifestVersion string                  `json:"manifestVersion"`
	GeneratedAt     string                  `json:"generatedAt"`
	Evidence        map[string]EvidenceItem `json:"evidence"`
	Summary         EvidenceSummary         `json:"summary"`
	Fixtures        map[string]int          `json:"fixtures"`
}

type EvidenceSummary struct {
	Total       int `json:"total"`
	Pass        int `json:"pass"`
	NotVerified int `json:"not_verified"`
	Fail        int `json:"fail"`
	Blocked     int `json:"blocked"`
}

type EvidenceLoader struct {
	mu       sync.RWMutex
	manifest *EvidenceManifest
	loadedAt time.Time
	path     string
}

var (
	defaultLoader     *EvidenceLoader
	defaultLoaderOnce sync.Once
)

func GetEvidenceLoader(path string) *EvidenceLoader {
	if path == "" {
		path = "contracts/native_bridge/stage2-closure-evidence.json"
	}
	defaultLoaderOnce.Do(func() {
		defaultLoader = &EvidenceLoader{path: path}
	})
	if defaultLoader.path != path {
		defaultLoader = &EvidenceLoader{path: path}
	}
	return defaultLoader
}

func NewEvidenceLoader(path string) *EvidenceLoader {
	return &EvidenceLoader{path: path}
}

func (l *EvidenceLoader) Load() (*EvidenceManifest, error) {
	l.mu.RLock()
	if l.manifest != nil && time.Since(l.loadedAt) < 5*time.Minute {
		defer l.mu.RUnlock()
		return l.manifest, nil
	}
	l.mu.RUnlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("evidence loader: read file %s: %w", l.path, err)
	}

	var manifest EvidenceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("evidence loader: parse json: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.manifest = &manifest
	l.loadedAt = time.Now()
	return l.manifest, nil
}

func (l *EvidenceLoader) LoadFromBytes(data []byte) (*EvidenceManifest, error) {
	var manifest EvidenceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("evidence loader: parse json: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.manifest = &manifest
	l.loadedAt = time.Now()
	return l.manifest, nil
}

func (l *EvidenceLoader) Cached() *EvidenceManifest {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.manifest
}

type RuntimeArchitectureGate struct {
	container       CanonicalAuthorityProvider
	currentPlatform string
}

func NewRuntimeArchitectureGate(container CanonicalAuthorityProvider, platform string) *RuntimeArchitectureGate {
	return &RuntimeArchitectureGate{
		container:       container,
		currentPlatform: platform,
	}
}

func (g *RuntimeArchitectureGate) Check(ctx context.Context) (bool, []string) {
	var failures []string

	if g.container == nil {
		return false, []string{"KernelContainer: nil"}
	}
	if g.container.ToolFacade() == nil {
		failures = append(failures, "ToolFacade: nil")
	}
	if g.container.PermissionBroker() == nil {
		failures = append(failures, "PermissionBroker: nil")
	}
	if g.container.EventService() == nil {
		failures = append(failures, "EventService: nil")
	}
	if g.container.ScheduleService() == nil {
		failures = append(failures, "ScheduleService: nil")
	}
	if g.container.TaskRuntimeService() == nil {
		failures = append(failures, "TaskRuntimeService: nil")
	}
	if g.container.HookService() == nil {
		failures = append(failures, "HookService: nil")
	}
	if g.container.NativeBridgeRelay() == nil {
		failures = append(failures, "NativeBridgeRelay: nil")
	}
	if g.container.PlatformBridge() == nil {
		failures = append(failures, "PlatformBridge: nil")
	}

	return len(failures) == 0, failures
}

func (g *RuntimeArchitectureGate) ArchitectureReady() bool {
	ready, _ := g.Check(context.Background())
	return ready
}

type Stage2ClosureGate struct {
	loader          *EvidenceLoader
	runtimeGate     *RuntimeArchitectureGate
	manifestVersion string
}

var (
	defaultClosureGate     *Stage2ClosureGate
	defaultClosureGateOnce sync.Once
)

func GetStage2ClosureGate(loader *EvidenceLoader, runtimeGate *RuntimeArchitectureGate) *Stage2ClosureGate {
	defaultClosureGateOnce.Do(func() {
		defaultClosureGate = &Stage2ClosureGate{
			loader:      loader,
			runtimeGate: runtimeGate,
		}
		if loader != nil {
			if m, err := loader.Load(); err == nil {
				defaultClosureGate.manifestVersion = m.ManifestVersion
			}
		}
	})
	return defaultClosureGate
}

func NewStage2ClosureGate(loader *EvidenceLoader, runtimeGate *RuntimeArchitectureGate) *Stage2ClosureGate {
	g := &Stage2ClosureGate{
		loader:      loader,
		runtimeGate: runtimeGate,
	}
	if loader != nil {
		if m, err := loader.Load(); err == nil {
			g.manifestVersion = m.ManifestVersion
		}
	}
	return g
}

func (g *Stage2ClosureGate) ValidateG0(ctx context.Context) (bool, []string, error) {
	var failures []string

	if g.runtimeGate == nil {
		return false, []string{"RuntimeArchitectureGate: nil"}, nil
	}

	runtimeOK, runtimeFailures := g.runtimeGate.Check(ctx)
	if !runtimeOK {
		failures = append(failures, runtimeFailures...)
	}

	if g.loader != nil {
		manifest, err := g.loader.Load()
		if err != nil {
			failures = append(failures, fmt.Sprintf("EvidenceLoader: %v", err))
		} else {
			for key, item := range manifest.Evidence {
				switch item.Status {
				case EvidenceFAIL, EvidenceBLOCKED:
					failures = append(failures, fmt.Sprintf("Evidence[%s]: %s", key, item.Status))
				}
			}
		}
	}

	return len(failures) == 0, failures, nil
}

func (g *Stage2ClosureGate) ManifestVersionConsistent(expected string) bool {
	if g.manifestVersion == "" {
		return false
	}
	return g.manifestVersion == expected
}

func (g *Stage2ClosureGate) CurrentManifestVersion() string {
	return g.manifestVersion
}

func (g *Stage2ClosureGate) ArchitectureReady() bool {
	if g.runtimeGate == nil {
		return false
	}
	return g.runtimeGate.ArchitectureReady()
}

