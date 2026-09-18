package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"encoding/json"
)

type TestCategory string

const (
	CatProcessCrash   TestCategory = "process_crash"
	CatHandshake      TestCategory = "handshake"
	CatHeartbeat      TestCategory = "heartbeat"
	CatRPCTimeout     TestCategory = "rpc_timeout"
	CatQueue          TestCategory = "queue"
	CatQuota          TestCategory = "quota"
	CatSecretRevoke   TestCategory = "secret_revoke"
	CatEmergency      TestCategory = "emergency"
	CatUpgrade        TestCategory = "upgrade"
	CatRollback       TestCategory = "rollback"
	CatDisable        TestCategory = "disable_uninstall"
	CatBackendRestart TestCategory = "backend_restart"
	CatOrphan         TestCategory = "orphan_recovery"
	CatCrossRuntime   TestCategory = "cross_runtime"
	CatCrossService   TestCategory = "cross_service"
	CatCombined       TestCategory = "combined"
	CatCheckpoint     TestCategory = "checkpoint"
	CatResidue        TestCategory = "residue"
)

type TestResult struct {
	Category string
	ID       string
	Name     string
	Passed   bool
	Error    string
	Duration time.Duration
}

type TestFunc func(ctx context.Context, host *PseudoHost) (TestResult, error)

type TestCase struct {
	ID       string
	Category TestCategory
	Name     string
	Run      TestFunc
}

type MatrixResult struct {
	Total   int
	Passed  int
	Failed  int
	Results []TestResult
}

func FindPluginBinary() string {
	candidates := []string{
		"../../../go/cmd/mock-game-plugin/mock-game-plugin.exe",
		"../../../go/cmd/mock-game-plugin/mock-game-plugin",
		"../../go/cmd/mock-game-plugin/mock-game-plugin.exe",
		"../../go/cmd/mock-game-plugin/mock-game-plugin",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func RunMatrix(t *testing.T, category TestCategory, cases []TestCase) {
	binary := FindPluginBinary()
	if binary == "" {
		t.Fatalf("Category %s: mock plugin binary not found. Build it first: cd ../../../go/cmd/mock-game-plugin && go build -o mock-game-plugin.exe .", category)
	}

	result := MatrixResult{
		Total: len(cases),
	}

	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tr, err := tc.Run(ctx, nil)
			tr.Duration = time.Since(startTime)
			tr.ID = tc.ID
			tr.Category = string(tc.Category)
			tr.Name = tc.Name

			if err != nil {
				tr.Passed = false
				tr.Error = err.Error()
				result.Failed++
				t.Errorf("[%s] %s FAIL: %v", tc.ID, tc.Name, err)
			} else if !tr.Passed {
				result.Failed++
				t.Errorf("[%s] %s FAIL: %s", tc.ID, tc.Name, tr.Error)
			} else {
				result.Passed++
				t.Logf("[%s] %s PASS (%.2fs)", tc.ID, tc.Name, tr.Duration.Seconds())
			}
			result.Results = append(result.Results, tr)
		})
	}

	t.Logf("Matrix %s: %d/%d passed", category, result.Passed, result.Total)
}

func RunWithHost(ctx context.Context, binary string, fn func(ctx context.Context, host *PseudoHost) error) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	host := NewPseudoHost(launchConfig{pluginPath: binary})
	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("start host: %w", err)
	}
	defer host.Kill()

	return fn(ctx, host)
}

func MustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func NewPseudoHostForBinary(pluginPath string) *PseudoHost {
	return NewPseudoHost(launchConfig{pluginPath: pluginPath})
}
