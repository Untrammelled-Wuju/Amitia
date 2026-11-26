//go:build rsi_quota

package task_runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/u-ai/backend/internal/platform/process"
)

func TestQuotaRejectsNonFinite(t *testing.T) {
	ceiling := DefaultQuotaCeiling()
	nanQuota := ResourceQuota{CPUPercent: math.NaN()}
	if violations := ValidateQuota(nanQuota); len(violations) != 1 || violations[0].Reason != "not_finite" {
		t.Fatalf("nan violations=%+v", violations)
	}
	if _, _, err := NormalizeQuota(nanQuota, ceiling); !errors.Is(err, ErrQuotaRejected) {
		t.Fatalf("nan err=%v", err)
	}
	infQuota := ResourceQuota{CPUPercent: math.Inf(1)}
	if _, _, err := NormalizeQuota(infQuota, ceiling); !errors.Is(err, ErrQuotaRejected) {
		t.Fatalf("inf err=%v", err)
	}
	fmt.Printf("m07-s04 reject nan=true inf=true\n")
}

func TestQuotaRejectsNegative(t *testing.T) {
	quota := ResourceQuota{MemoryBytes: -1, CPUPercent: -2, Processes: -3, Concurrency: -4, DiskBytes: -5}
	violations := ValidateQuota(quota)
	if len(violations) != 5 {
		t.Fatalf("violations=%+v", violations)
	}
	for _, violation := range violations {
		if violation.Reason != "negative" {
			t.Fatalf("reason=%s", violation.Reason)
		}
	}
	_, _, err := NormalizeQuota(quota, DefaultQuotaCeiling())
	if !errors.Is(err, ErrQuotaRejected) {
		t.Fatalf("err=%v", err)
	}
	if !IsTaskErrorCode(err, ErrTaskDefinitionInvalid) || HTTPStatusForErrorCode(ErrTaskDefinitionInvalid) != 400 {
		t.Fatalf("task error code not wired: %v", err)
	}
	fmt.Printf("m07-s04 reject negative=5 task_code=400\n")
}

func TestQuotaNormalizesUnset(t *testing.T) {
	ceiling := DefaultQuotaCeiling()
	normalized, notes, err := NormalizeQuota(ResourceQuota{}, ceiling)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized != (ResourceQuota{MemoryBytes: ceiling.MemoryBytes, CPUPercent: ceiling.CPUPercent, Processes: ceiling.Processes, Concurrency: ceiling.Concurrency, DiskBytes: ceiling.DiskBytes}) {
		t.Fatalf("normalized=%+v", normalized)
	}
	if len(notes) != 5 {
		t.Fatalf("notes=%v", notes)
	}
	fmt.Printf("m07-s04 default dimensions=5\n")
}

func TestQuotaClampsOverCeiling(t *testing.T) {
	ceiling := DefaultQuotaCeiling()
	quota := ResourceQuota{MemoryBytes: 2 << 30, CPUPercent: 400, Processes: 100, Concurrency: 64, DiskBytes: 1 << 30}
	normalized, notes, err := NormalizeQuota(quota, ceiling)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized != (ResourceQuota{MemoryBytes: ceiling.MemoryBytes, CPUPercent: ceiling.CPUPercent, Processes: ceiling.Processes, Concurrency: ceiling.Concurrency, DiskBytes: ceiling.DiskBytes}) {
		t.Fatalf("normalized=%+v", normalized)
	}
	if len(notes) != 5 {
		t.Fatalf("notes=%v", notes)
	}
	fmt.Printf("m07-s04 clamp dimensions=5\n")
}

func TestQuotaFloorsSubPercentCPU(t *testing.T) {
	normalized, notes, err := NormalizeQuota(ResourceQuota{CPUPercent: 0.2}, DefaultQuotaCeiling())
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	floor := false
	for _, note := range notes {
		if note == "cpuPercent=floor" {
			floor = true
		}
	}
	if normalized.CPUPercent != MinQuotaCPUPercent || !floor {
		t.Fatalf("cpu=%v notes=%v", normalized.CPUPercent, notes)
	}
	fmt.Printf("m07-s04 floor cpu=1\n")
}

func TestQuotaKeepsValuesWithinBounds(t *testing.T) {
	ceiling := DefaultQuotaCeiling()
	quota := ResourceQuota{MemoryBytes: 128 << 20, CPUPercent: 25, Processes: ceiling.Processes, Concurrency: 2, DiskBytes: 64 << 20}
	normalized, notes, err := NormalizeQuota(quota, ceiling)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized != quota || len(notes) != 0 {
		t.Fatalf("normalized=%+v notes=%v", normalized, notes)
	}
	fmt.Printf("m07-s04 keep unchanged=true\n")
}

func TestQuotaRejectsInvalidCeiling(t *testing.T) {
	_, _, err := NormalizeQuota(ResourceQuota{}, QuotaCeiling{MemoryBytes: 0, CPUPercent: math.NaN(), Processes: 1, Concurrency: 1, DiskBytes: 1})
	if !errors.Is(err, ErrQuotaRejected) {
		t.Fatalf("ceiling err=%v", err)
	}
	var rejection *QuotaRejection
	if !errors.As(err, &rejection) || len(rejection.Violations) != 2 {
		t.Fatalf("rejection=%+v", rejection)
	}
	fmt.Printf("m07-s04 ceiling invalid=true\n")
}

func TestQuotaSchemaRoundTrip(t *testing.T) {
	quota := ResourceQuota{MemoryBytes: 128 << 20, CPUPercent: 25, Processes: 4, Concurrency: 2, DiskBytes: 64 << 20}
	if quota.SchemaVersion() != QuotaSchemaVersion || QuotaSchemaVersion != 1 {
		t.Fatalf("schema=%d", quota.SchemaVersion())
	}
	body, err := json.Marshal(quota)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ResourceQuota
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded != quota {
		t.Fatalf("decoded=%+v", decoded)
	}
	fmt.Printf("m07-s04 schema=1 roundtrip=true\n")
}

func TestQuotaSupportZeroesUnenforcedDimensions(t *testing.T) {
	quota := ResourceQuota{MemoryBytes: 128 << 20, CPUPercent: 25, Processes: 4, Concurrency: 2, DiskBytes: 64 << 20}
	applied, notes := ApplyQuotaSupport(quota, process.ResourceLimitSupport{Processes: true})
	if applied.MemoryBytes != 0 || applied.CPUPercent != 0 || applied.Processes != 4 {
		t.Fatalf("applied=%+v", applied)
	}
	if len(notes) != 2 {
		t.Fatalf("notes=%v", notes)
	}
	fmt.Printf("m07-s04 support unenforced=2 enforced=processes\n")
}

func TestQuotaLedgerRejectsNonFinite(t *testing.T) {
	ledger, err := NewQuotaLedger(DefaultQuotaCeiling())
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if err := ledger.Admit(ResourceQuota{CPUPercent: math.NaN()}); !errors.Is(err, ErrQuotaRejected) {
		t.Fatalf("nan admit err=%v", err)
	}
	if used := ledger.Snapshot(); !used.IsZero() {
		t.Fatalf("nan polluted used=%+v", used)
	}
	fmt.Printf("m07-s04 ledger reject nonfinite=true used=0\n")
}

func TestQuotaLedgerAdmitsWithinBudget(t *testing.T) {
	ledger, err := NewQuotaLedger(DefaultQuotaCeiling())
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := ledger.Admit(ResourceQuota{MemoryBytes: 100 << 20, CPUPercent: 10}); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
	}
	used := ledger.Snapshot()
	if used.MemoryBytes != 200<<20 || used.CPUPercent != 20 {
		t.Fatalf("used=%+v", used)
	}
	fmt.Printf("m07-s04 ledger admitted=2 used_mb=200\n")
}

func TestQuotaLedgerRejectsOverBudget(t *testing.T) {
	ledger, err := NewQuotaLedger(DefaultQuotaCeiling())
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if err := ledger.Admit(ResourceQuota{MemoryBytes: 500 << 20}); err != nil {
		t.Fatalf("first admit: %v", err)
	}
	err = ledger.Admit(ResourceQuota{MemoryBytes: 100 << 20})
	if !errors.Is(err, ErrQuotaBudgetExceeded) {
		t.Fatalf("budget err=%v", err)
	}
	var budget *QuotaBudgetError
	if !errors.As(err, &budget) || budget.Dimension != "memoryBytes" {
		t.Fatalf("budget error=%+v", budget)
	}
	if used := ledger.Snapshot(); used.MemoryBytes != 500<<20 {
		t.Fatalf("used=%+v", used)
	}
	fmt.Printf("m07-s04 budget rejected=true dimension=memory\n")
}

func TestQuotaLedgerIsAllOrNothing(t *testing.T) {
	ledger, err := NewQuotaLedger(DefaultQuotaCeiling())
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	if err := ledger.Admit(ResourceQuota{CPUPercent: 40}); err != nil {
		t.Fatalf("first admit: %v", err)
	}
	err = ledger.Admit(ResourceQuota{MemoryBytes: 100 << 20, CPUPercent: 40})
	if !errors.Is(err, ErrQuotaBudgetExceeded) {
		t.Fatalf("atomic err=%v", err)
	}
	if used := ledger.Snapshot(); used.MemoryBytes != 0 || used.CPUPercent != 40 {
		t.Fatalf("partial admit used=%+v", used)
	}
	fmt.Printf("m07-s04 atomic=true dimension=cpu\n")
}

func TestQuotaLedgerReleaseIsIdempotent(t *testing.T) {
	ledger, err := NewQuotaLedger(DefaultQuotaCeiling())
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	quota := ResourceQuota{MemoryBytes: 100 << 20, CPUPercent: 10}
	if err := ledger.Admit(quota); err != nil {
		t.Fatalf("admit: %v", err)
	}
	ledger.Release(quota)
	ledger.Release(quota)
	if used := ledger.Snapshot(); !used.IsZero() {
		t.Fatalf("used=%+v", used)
	}
	fmt.Printf("m07-s04 release zero=true\n")
}

func TestQuotaLedgerConcurrentAdmission(t *testing.T) {
	budget := QuotaCeiling{MemoryBytes: 64 << 20, CPUPercent: 50, Processes: 32, Concurrency: 100, DiskBytes: 256 << 20}
	ledger, err := NewQuotaLedger(budget)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	var admitted atomic.Int64
	var wait sync.WaitGroup
	wait.Add(32)
	for i := 0; i < 32; i++ {
		go func() {
			defer wait.Done()
			if err := ledger.Admit(ResourceQuota{MemoryBytes: 8 << 20}); err == nil {
				admitted.Add(1)
			}
		}()
	}
	wait.Wait()
	if admitted.Load() != 8 || ledger.Snapshot().MemoryBytes != 64<<20 {
		t.Fatalf("admitted=%d used=%d", admitted.Load(), ledger.Snapshot().MemoryBytes)
	}
	fmt.Printf("m07-s04 concurrent attempts=32 admitted=8 used_mb=64\n")
}
