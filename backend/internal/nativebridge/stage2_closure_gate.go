package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type GateStatus string

const (
	GateStatusPass           GateStatus = "PASS"
	GateStatusFail           GateStatus = "FAIL"
	GateStatusNotVerified    GateStatus = "NOT_VERIFIED"
	GateStatusBlockedExternal GateStatus = "BLOCKED_EXTERNAL"
)

type GateRecord struct {
	ID       string     `json:"id"`
	Status   GateStatus `json:"status"`
	Evidence string     `json:"evidence,omitempty"`
}

type ClosureEvidence struct {
	Version        string                  `json:"version"`
	ManifestVersion string                 `json:"manifestVersion"`
	GeneratedAt    string                  `json:"generatedAt"`
	Evidence       map[string]*GateRecord  `json:"evidence"`
	Summary        struct {
		Total        int `json:"total"`
		Pass         int `json:"pass"`
		NotVerified  int `json:"not_verified"`
		Fail         int `json:"fail"`
		Blocked      int `json:"blocked"`
	} `json:"summary"`
}

type Stage2ClosureGate struct {
	mu         sync.RWMutex
	records    map[string]*GateRecord
	requiredGates []string
	loaded     bool
}

func NewStage2ClosureGate(requiredGates []string) *Stage2ClosureGate {
	return &Stage2ClosureGate{
		records:       make(map[string]*GateRecord),
		requiredGates: requiredGates,
	}
}

func (g *Stage2ClosureGate) LoadFromEvidence(path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read evidence file: %w", err)
	}

	var evidence ClosureEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return fmt.Errorf("parse evidence file: %w", err)
	}

	for id, record := range evidence.Evidence {
		g.records[id] = record
	}

	for _, reqID := range g.requiredGates {
		if _, ok := g.records[reqID]; !ok {
			g.records[reqID] = &GateRecord{
				ID:     reqID,
				Status: GateStatusNotVerified,
			}
		}
	}

	g.loaded = true
	return nil
}

func (g *Stage2ClosureGate) SetGate(ctx context.Context, id string, status GateStatus, evidence string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.loaded {
		return fmt.Errorf("gate not loaded from evidence")
	}

	if _, ok := g.records[id]; !ok {
		return fmt.Errorf("unknown gate id: %s", id)
	}

	g.records[id].Status = status
	g.records[id].Evidence = evidence
	return nil
}

func (g *Stage2ClosureGate) GetGate(id string) (*GateRecord, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	record, ok := g.records[id]
	return record, ok
}

func (g *Stage2ClosureGate) GetAllGates() map[string]*GateRecord {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]*GateRecord, len(g.records))
	for k, v := range g.records {
		result[k] = v
	}
	return result
}

func (g *Stage2ClosureGate) CanRunCutover() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.loaded {
		return false
	}

	for _, reqID := range g.requiredGates {
		record, ok := g.records[reqID]
		if !ok || record.Status != GateStatusPass {
			return false
		}
	}
	return true
}

func (g *Stage2ClosureGate) GetRequiredGates() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]string, len(g.requiredGates))
	copy(result, g.requiredGates)
	return result
}

func (g *Stage2ClosureGate) ValidateConsistency() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if !g.loaded {
		return fmt.Errorf("gate not loaded from evidence")
	}

	for _, reqID := range g.requiredGates {
		if _, ok := g.records[reqID]; !ok {
			return fmt.Errorf("required gate missing: %s", reqID)
		}
	}

	return nil
}

func (g *Stage2ClosureGate) GetSummary() (total, pass, notVerified, fail, blocked int) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	total = len(g.records)
	for _, record := range g.records {
		switch record.Status {
		case GateStatusPass:
			pass++
		case GateStatusNotVerified:
			notVerified++
		case GateStatusFail:
			fail++
		case GateStatusBlockedExternal:
			blocked++
		}
	}
	return
}

func DefaultRequiredGates() []string {
	return []string{
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
		"G0-D07-share-receive-extension-or-capability-removed",
		"G0-D08-share-receive-no-pseudouri",
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
}
