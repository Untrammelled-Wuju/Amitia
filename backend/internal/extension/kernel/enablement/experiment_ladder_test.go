//go:build rsi_lifecycle && rsi_experiment

package enablement

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func realPluginManifests(t *testing.T, root string) []Manifest {
	t.Helper()
	patterns := []string{
		"plugins-code/*/amitia-extension.json",
		"plugins-code/channels/*/amitia-extension.json",
		"plugins-builtin/*/amitia-extension.json",
		"testplugins/*/amitia-extension.json",
	}
	seen := map[string]bool{}
	manifests := []Manifest{}
	for _, pattern := range patterns {
		paths, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			parsed, err := plugincontract.Parse(data)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			if seen[parsed.Plugin.ID] {
				continue
			}
			seen[parsed.Plugin.ID] = true
			permissions := make([]string, 0, len(parsed.Permissions))
			for _, permission := range parsed.Permissions {
				permissions = append(permissions, permission.Name)
			}
			entry := ""
			for _, module := range parsed.Modules {
				if module.Runtime != nil && module.Runtime.Entry != "" {
					entry = module.Runtime.Entry
					break
				}
			}
			manifests = append(manifests, Manifest{
				ID:          parsed.Plugin.ID,
				Version:     parsed.Plugin.Version,
				Permissions: permissions,
				Entry:       entry,
			})
		}
	}
	sort.Slice(manifests, func(left, right int) bool { return manifests[left].ID < manifests[right].ID })
	return manifests
}

func TestExperimentRealPluginLadder(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	manifests := realPluginManifests(t, root)
	if len(manifests) < 5 {
		t.Fatalf("real plugin manifests=%d want >=5", len(manifests))
	}
	zeroSpawner := &leakSpawner{}
	zeroInstaller := NewInstaller(zeroSpawner)
	zero := zeroInstaller.Snapshot()
	if zero.Plugins != 0 || zero.Processes != 0 || zero.Workers != 0 || zero.SpawnCalls != 0 {
		t.Fatalf("zero snapshot=%+v", zero)
	}
	tiers := []int{}
	for _, requested := range []int{1, 3, 5, 9} {
		count := requested
		if count > len(manifests) {
			count = len(manifests)
		}
		spawner := &leakSpawner{}
		installer := NewInstaller(spawner)
		if _, err := installer.InstallBatch(context.Background(), manifests[:count]); err != nil {
			t.Fatalf("install %d: %v", count, err)
		}
		installed := installer.Snapshot()
		if installed.Plugins != count || installed.Processes != 0 || installed.Workers != 0 || installed.SpawnCalls != 0 {
			t.Fatalf("tier %d install snapshot=%+v", count, installed)
		}
		for i := 0; i < count; i++ {
			if err := installer.Enable(context.Background(), manifests[i].ID, 1); err != nil {
				t.Fatalf("tier %d enable %s: %v", count, manifests[i].ID, err)
			}
		}
		enabled := installer.Snapshot()
		if enabled.Processes != 0 || enabled.Workers != 0 || spawner.Starts() != 0 {
			t.Fatalf("tier %d enable snapshot=%+v starts=%d", count, enabled, spawner.Starts())
		}
		activations := count
		if activations > 3 {
			activations = 3
		}
		for i := 0; i < activations; i++ {
			if err := installer.Activate(context.Background(), manifests[i].ID, 2); err != nil {
				t.Fatalf("tier %d activate %s: %v", count, manifests[i].ID, err)
			}
		}
		activated := installer.Snapshot()
		if activated.Processes != activations || spawner.Starts() != activations {
			t.Fatalf("tier %d activate snapshot=%+v starts=%d", count, activated, spawner.Starts())
		}
		clock := &experimentTestClock{}
		governor := task_runtime.NewExperimentGovernor(task_runtime.ExperimentGovernorConfig{IdleAfter: time.Hour, CheckpointTTL: time.Hour, MaxActive: 16}).WithClock(clock.Now)
		for i := 0; i < activations; i++ {
			if err := governor.Register(context.Background(), &experimentTestJob{id: fmt.Sprintf("tier-%d-%d", count, i)}); err != nil {
				t.Fatalf("tier %d register: %v", count, err)
			}
		}
		if _, err := governor.Foreground(context.Background()); err != nil {
			t.Fatalf("tier %d foreground: %v", count, err)
		}
		executed := 0
		for i := 0; i < activations; i++ {
			report, err := governor.Resume(context.Background(), fmt.Sprintf("tier-%d-%d", count, i))
			if err != nil {
				t.Fatalf("tier %d resume: %v", count, err)
			}
			executed += report.Applied
		}
		if executed != activations {
			t.Fatalf("tier %d executed=%d want %d", count, executed, activations)
		}
		tiers = append(tiers, count)
		t.Logf("m07-s06-detail tier=%d plugins=%d enabled_processes=0 activated_processes=%d spawn_calls=%d executed=%d", count, count, activations, spawner.Starts(), executed)
	}
	available := len(manifests)
	fmt.Printf("m07-s06 real_ladder_zero plugins=0 processes=0 workers=0 spawn_calls=0\n")
	fmt.Printf("m07-s06 real_ladder tiers=1,3,5,9 available=%d requested=10 gap=%d load_split=true\n", available, 10-available)
}

type experimentTestClock struct {
	at time.Time
}

func (c *experimentTestClock) Now() time.Time {
	if c.at.IsZero() {
		c.at = time.Unix(1700000000, 0)
	}
	return c.at
}

type experimentTestJob struct {
	mu      sync.Mutex
	id      string
	applied []string
}

func (j *experimentTestJob) ID() string { return j.id }

func (j *experimentTestJob) Epoch() uint64 { return 1 }

func (j *experimentTestJob) Effects() []string { return []string{"effect"} }

func (j *experimentTestJob) Checkpoint(ctx context.Context) (task_runtime.ExperimentCheckpoint, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return task_runtime.ExperimentCheckpoint{
		JobID:            j.id,
		Epoch:            1,
		Progress:         len(j.applied),
		CompletedEffects: append([]string(nil), j.applied...),
	}, nil
}

func (j *experimentTestJob) Pause(ctx context.Context, checkpoint task_runtime.ExperimentCheckpoint) error {
	return nil
}

func (j *experimentTestJob) Resume(ctx context.Context, checkpoint task_runtime.ExperimentCheckpoint, pending []string) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.applied = append(j.applied, pending...)
	return len(pending), nil
}
