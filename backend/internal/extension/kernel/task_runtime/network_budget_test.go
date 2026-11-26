//go:build rsi_netbudget

package task_runtime

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type netClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *netClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() {
		c.at = time.Unix(1700000000, 0)
	}
	return c.at
}

func (c *netClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.at.IsZero() {
		c.at = time.Unix(1700000000, 0)
	}
	c.at = c.at.Add(delta)
}

func newNetworkBucket(t *testing.T, budget NetworkBudget, clock *netClock) *NetworkBucket {
	t.Helper()
	bucket, err := NewNetworkBucket(budget)
	if err != nil {
		t.Fatalf("bucket: %v", err)
	}
	return bucket.WithClock(clock.Now)
}

func TestNetworkBudgetAdmitsBurst(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 1, BytesPerSecond: 1024, Burst: 3}, clock)
	for i := 0; i < 3; i++ {
		if err := bucket.Admit(16); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
	}
	if err := bucket.Admit(16); !errors.Is(err, ErrNetworkBudgetExceeded) {
		t.Fatalf("fourth err=%v", err)
	}
	fmt.Printf("m07-s13 burst admitted=3 rejected=1\n")
}

func TestNetworkBudgetRetryable(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 10, BytesPerSecond: 1024, Burst: 1}, clock)
	if err := bucket.Admit(0); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := bucket.Admit(0)
	var budget *NetworkBudgetError
	if !errors.As(err, &budget) || !budget.Retryable() || budget.RetryAfter <= 0 {
		t.Fatalf("err=%v", err)
	}
	if !IsTaskErrorCode(err, ErrTaskQueueFull) || HTTPStatusForErrorCode(ErrTaskQueueFull) != 429 {
		t.Fatalf("task code err=%v", err)
	}
	fmt.Printf("m07-s13 retry after_ms>0 http=429\n")
}

func TestNetworkBudgetByteExhaustion(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 100, BytesPerSecond: 100, Burst: 10}, clock)
	if err := bucket.Admit(60); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := bucket.Admit(60)
	var budget *NetworkBudgetError
	if !errors.As(err, &budget) || budget.Dimension != "bytes" {
		t.Fatalf("err=%v", err)
	}
	fmt.Printf("m07-s13 bytes exceeded=true\n")
}

func TestNetworkBudgetAllOrNothing(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 100, BytesPerSecond: 50, Burst: 2}, clock)
	if err := bucket.Admit(60); !errors.Is(err, ErrNetworkBudgetExceeded) {
		t.Fatalf("oversize bytes err=%v", err)
	}
	requests, _ := bucket.Tokens()
	if requests != 2 {
		t.Fatalf("requests=%v", requests)
	}
	if err := bucket.Admit(10); err != nil {
		t.Fatalf("small: %v", err)
	}
	requests, _ = bucket.Tokens()
	if requests != 1 {
		t.Fatalf("requests=%v", requests)
	}
	fmt.Printf("m07-s13 atomic request_tokens=1\n")
}

func TestNetworkBudgetRefill(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 10, BytesPerSecond: 1024, Burst: 1}, clock)
	if err := bucket.Admit(0); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := bucket.Admit(0); !errors.Is(err, ErrNetworkBudgetExceeded) {
		t.Fatalf("second err=%v", err)
	}
	clock.Advance(200 * time.Millisecond)
	if err := bucket.Admit(0); err != nil {
		t.Fatalf("refilled: %v", err)
	}
	fmt.Printf("m07-s13 refill ok=true\n")
}

func TestNetworkBudgetInvalidBytes(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 1, BytesPerSecond: 1024, Burst: 1}, clock)
	if err := bucket.Admit(-1); !errors.Is(err, ErrNetworkBudgetInvalid) {
		t.Fatalf("negative err=%v", err)
	}
	if err := bucket.Admit(MaxNetworkRequestBytes + 1); !errors.Is(err, ErrNetworkBudgetInvalid) {
		t.Fatalf("oversize err=%v", err)
	}
	fmt.Printf("m07-s13 invalid bytes=2\n")
}

func TestNetworkBudgetInvalidConfig(t *testing.T) {
	cases := []NetworkBudget{
		{RequestsPerSecond: 0, BytesPerSecond: 1, Burst: 1},
		{RequestsPerSecond: math.NaN(), BytesPerSecond: 1, Burst: 1},
		{RequestsPerSecond: 1, BytesPerSecond: 1, Burst: 0},
	}
	invalid := 0
	for _, budget := range cases {
		if _, err := NewNetworkBucket(budget); errors.Is(err, ErrNetworkBudgetInvalid) {
			invalid++
		}
	}
	if invalid != 3 {
		t.Fatalf("invalid=%d", invalid)
	}
	fmt.Printf("m07-s13 config invalid=3\n")
}

func TestNetworkBudgetPressure(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 1, BytesPerSecond: 1024, Burst: 2}, clock)
	if bucket.Pressure() <= 0 {
		t.Fatalf("pressure=%v", bucket.Pressure())
	}
	if err := bucket.Admit(0); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := bucket.Admit(0); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if bucket.Pressure() != 0 {
		t.Fatalf("pressure=%v", bucket.Pressure())
	}
	fmt.Printf("m07-s13 pressure exhausted=0\n")
}

func TestNetworkBudgetConcurrent(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 1, BytesPerSecond: 1 << 20, Burst: 10}, clock)
	var admitted atomic.Int64
	var wait sync.WaitGroup
	for i := 0; i < 32; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := bucket.Admit(0); err == nil {
				admitted.Add(1)
			}
		}()
	}
	wait.Wait()
	if admitted.Load() != 10 {
		t.Fatalf("admitted=%d", admitted.Load())
	}
	fmt.Printf("m07-s13 concurrent admitted=10\n")
}

func TestNetworkBudgetZeroByteRequest(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 1, BytesPerSecond: 1024, Burst: 2}, clock)
	if err := bucket.Admit(0); err != nil {
		t.Fatalf("zero byte: %v", err)
	}
	requests, bytes := bucket.Tokens()
	if requests != 1 || bytes != 1024 {
		t.Fatalf("requests=%v bytes=%v", requests, bytes)
	}
	fmt.Printf("m07-s13 zero_byte bytes_untouched=true\n")
}

func TestNetworkBudgetByteRefill(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 100, BytesPerSecond: 100, Burst: 10}, clock)
	if err := bucket.Admit(100); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := bucket.Admit(50); !errors.Is(err, ErrNetworkBudgetExceeded) {
		t.Fatalf("byte err=%v", err)
	}
	clock.Advance(500 * time.Millisecond)
	if err := bucket.Admit(50); err != nil {
		t.Fatalf("refill: %v", err)
	}
	fmt.Printf("m07-s13 byte_refill ok=true\n")
}

func TestNetworkBudgetBytesRetryAfter(t *testing.T) {
	clock := &netClock{}
	bucket := newNetworkBucket(t, NetworkBudget{RequestsPerSecond: 100, BytesPerSecond: 100, Burst: 10}, clock)
	if err := bucket.Admit(100); err != nil {
		t.Fatalf("first: %v", err)
	}
	err := bucket.Admit(100)
	var budget *NetworkBudgetError
	if !errors.As(err, &budget) || budget.Dimension != "bytes" || budget.RetryAfter <= 0 {
		t.Fatalf("err=%v", err)
	}
	fmt.Printf("m07-s13 bytes retry_after_ms>0\n")
}
