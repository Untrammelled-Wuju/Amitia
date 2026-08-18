package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

type EvidenceStatus string

const (
	EvidencePASS            EvidenceStatus = "PASS"
	EvidenceNOTVERIFIED     EvidenceStatus = "NOT_VERIFIED"
	EvidenceFAIL            EvidenceStatus = "FAIL"
	EvidenceBLOCKED         EvidenceStatus = "BLOCKED"
	EvidenceBLOCKEDEXTERNAL EvidenceStatus = "BLOCKED_EXTERNAL"

	CanonicalStage2ManifestVersion = "20260816002"
)

var CanonicalStage2RequiredGates = []string{
	"G0-A01-relay-ws-url",
	"G0-A02-relay-credential",
	"G0-A03-relay-local-token-header",
	"G0-A04-relay-reject-invalid-token",
	"G0-A05-single-pending-authority",
	"G0-A06-generation-handling",
	"G0-A07-android-harmless-rpc",
	"G0-A08-ios-harmless-rpc",
	"G0-A09-relay-fixtures",
	"G0-B01-file-mount-relativepath",
	"G0-B02-file-move-new-path",
	"G0-B03-file-no-sourcepath-destpath",
	"G0-B04-file-contentbase64",
	"G0-B05-file-all-operations-relativepath",
	"G0-B06-file-path-resolver",
	"G0-B07-file-bounded-chunk",
	"G0-B08-file-security-scoped",
	"G0-B09-file-go-swift-fixtures",
	"G0-B10-file-crud-smoke",
	"G0-C01-bg-submit-fields",
	"G0-C02-bg-taskrun-id-authority",
	"G0-C03-bg-launch-event",
	"G0-C04-bg-backend-taskruntime",
	"G0-C05-bg-expiration",
	"G0-C06-bg-no-second-checkpoint",
	"G0-C07-bg-pending-semantics",
	"G0-C08-bg-restart-recovery",
	"G0-D01-resource-no-file-uri",
	"G0-D02-resource-staging-contract",
	"G0-D03-resource-importer",
	"G0-D04-resource-canonical-only",
	"G0-D05-resource-bounded-streaming",
	"G0-D06-share-send-materialize",
	"G0-D07-share-receive-removed",
	"G0-E01-alarmkit-sdk-compile",
	"G0-E02-alarmkit-truthful-unavailable",
	"G0-E03-alarm-no-fake-success",
	"G0-E04-shortcuts-typed-intent",
	"G0-E05-shortcuts-no-string-action-id",
	"G0-E06-shortcuts-no-noop-handler",
	"G0-E07-shortcuts-backend-authority",
	"G0-E08-shortcuts-no-ininteraction",
	"G0-E09-shortcuts-no-fixed-return",
	"G0-F01-homekit-homes-event",
	"G0-F02-homekit-accessory-event",
	"G0-F03-bluetooth-discover-connect-failed",
	"G0-F04-bluetooth-connected-value-event",
	"G0-F05-event-generation-bound",
	"G0-F06-event-backpressure",
	"G0-F07-event-no-sensitive-leak",
	"G0-F08-event-dedup-fingerprint",
	"G0-F09-event-dequeue",
	"G0-G01-architecture-ready-false",
	"G0-G02-cutover-machine-validate",
	"G0-G03-validate-g0-blocked",
	"G0-G04-g0-not-dead-code",
	"G0-G05-platform-runtime-gate-separation",
	"G0-G06-step68-after-g0-pass",
	"G0-G07-fresh-existing-db-gate",
	"G0-G08-no-leak",
	"G0-H01-go-build",
	"G0-H02-flutter-analyze",
	"G0-H03-android-compile",
	"G0-H04-ios-xcodebuild",
	"G0-H05-shizuku-smoke",
	"G0-H06-ios-native-smoke",
	"G0-H07-file-crud-smoke",
	"G0-H08-bg-smoke",
	"G0-H09-blocked-not-pass",
}

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
	return &RuntimeArchitectureGate{container: container, currentPlatform: strings.ToLower(strings.TrimSpace(platform))}
}

func (g *RuntimeArchitectureGate) Check(ctx context.Context) (bool, []string) {
	_ = ctx
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
	if (g.currentPlatform == "ios" || g.currentPlatform == "android") && g.container.PlatformBridge() == nil {
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
	requiredGates   []string
}

func NewStage2ClosureGate(loader *EvidenceLoader, runtimeGate *RuntimeArchitectureGate) *Stage2ClosureGate {
	return newStage2ClosureGate(loader, runtimeGate, CanonicalStage2ManifestVersion, CanonicalStage2RequiredGates)
}

func NewStage2ClosureGateWithManifest(loader *EvidenceLoader, runtimeGate *RuntimeArchitectureGate, version string, required []string) *Stage2ClosureGate {
	return newStage2ClosureGate(loader, runtimeGate, version, required)
}

func newStage2ClosureGate(loader *EvidenceLoader, runtimeGate *RuntimeArchitectureGate, version string, required []string) *Stage2ClosureGate {
	g := &Stage2ClosureGate{
		loader:          loader,
		runtimeGate:     runtimeGate,
		manifestVersion: version,
		requiredGates:   append([]string(nil), required...),
	}
	return g
}

func (g *Stage2ClosureGate) ValidateG0(ctx context.Context) (bool, []string, error) {
	var failures []string

	if g.runtimeGate == nil {
		failures = append(failures, "RuntimeArchitectureGate: nil")
	} else if runtimeOK, runtimeFailures := g.runtimeGate.Check(ctx); !runtimeOK {
		failures = append(failures, runtimeFailures...)
	}

	if g.loader == nil {
		failures = append(failures, "EvidenceLoader: nil")
		return false, failures, nil
	}

	manifest, err := g.loader.Load()
	if err != nil {
		failures = append(failures, fmt.Sprintf("EvidenceLoader: %v", err))
		return false, failures, nil
	}
	if manifest.ManifestVersion != g.manifestVersion {
		failures = append(failures, fmt.Sprintf("ManifestVersion: got %q want %q", manifest.ManifestVersion, g.manifestVersion))
	}
	if summaryFailures := validateEvidenceSummary(manifest); len(summaryFailures) > 0 {
		failures = append(failures, summaryFailures...)
	}

	requiredSet := make(map[string]struct{}, len(g.requiredGates))
	for _, gateID := range g.requiredGates {
		requiredSet[gateID] = struct{}{}
		item, ok := manifest.Evidence[gateID]
		if !ok {
			failures = append(failures, fmt.Sprintf("Evidence[%s]: missing (NOT_VERIFIED)", gateID))
			continue
		}
		if item.Status != EvidencePASS {
			failures = append(failures, fmt.Sprintf("Evidence[%s]: %s", gateID, item.Status))
			continue
		}
		if strings.TrimSpace(item.Evidence) == "" {
			failures = append(failures, fmt.Sprintf("Evidence[%s]: PASS without evidence", gateID))
		}
	}
	for gateID := range manifest.Evidence {
		if _, ok := requiredSet[gateID]; !ok {
			failures = append(failures, fmt.Sprintf("Evidence[%s]: not declared by canonical manifest", gateID))
		}
	}

	return len(failures) == 0, failures, nil
}

func validateEvidenceSummary(manifest *EvidenceManifest) []string {
	if manifest == nil {
		return []string{"EvidenceManifest: nil"}
	}
	actual := EvidenceSummary{Total: len(manifest.Evidence)}
	for _, item := range manifest.Evidence {
		switch item.Status {
		case EvidencePASS:
			actual.Pass++
		case EvidenceNOTVERIFIED:
			actual.NotVerified++
		case EvidenceFAIL:
			actual.Fail++
		case EvidenceBLOCKED, EvidenceBLOCKEDEXTERNAL:
			actual.Blocked++
		default:
			actual.Blocked++
		}
	}
	if actual != manifest.Summary {
		return []string{fmt.Sprintf("EvidenceSummary mismatch: got %+v want %+v", manifest.Summary, actual)}
	}
	return nil
}

func (g *Stage2ClosureGate) ManifestVersionConsistent(expected string) bool {
	return g.manifestVersion != "" && g.manifestVersion == expected
}

func (g *Stage2ClosureGate) CurrentManifestVersion() string {
	return g.manifestVersion
}

func (g *Stage2ClosureGate) ArchitectureReady() bool {
	return g.runtimeGate != nil && g.runtimeGate.ArchitectureReady()
}
