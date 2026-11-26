//go:build rsi_backpressure && rsi_soak

package task_runtime

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func soakSeconds() int {
	if raw := os.Getenv("RSI_SOAK_SECONDS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return value
		}
	}
	return 30
}

func soakOpenFDs() int {
	entries, err := os.ReadDir("/dev/fd")
	if err != nil {
		entries, err = os.ReadDir("/proc/self/fd")
		if err != nil {
			return -1
		}
	}
	return len(entries)
}

func TestSoakCurve(t *testing.T) {
	seconds := soakSeconds()
	var sink *os.File
	if path := os.Getenv("RSI_SOAK_OUT"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatalf("soak sink: %v", err)
		}
		defer file.Close()
		sink = file
	}
	emit := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		fmt.Print(line)
		if sink != nil {
			if _, err := sink.WriteString(line); err != nil {
				t.Fatalf("soak sink write: %v", err)
			}
		}
	}
	emit("m07-s08-pid %d\n", os.Getpid())
	queue := NewBoundedQueue(BoundedQueueConfig{MaxItems: 256, MaxBytes: 8 << 20, MaxItemBytes: 4096})
	var paused atomic.Bool
	var activeTimers atomic.Int64
	var timersMax atomic.Int64
	var producers sync.WaitGroup
	var consumers sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		producers.Add(1)
		go func(worker int) {
			defer producers.Done()
			counter := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				if paused.Load() {
					time.Sleep(time.Millisecond)
					continue
				}
				counter++
				err := queue.Enqueue(context.Background(), QueueItem{
					ID:       fmt.Sprintf("w%d-%d", worker, counter),
					Priority: PriorityExperiment,
					Bytes:    128,
				})
				if err != nil {
					time.Sleep(200 * time.Microsecond)
				}
			}
		}(i)
	}
	for i := 0; i < 2; i++ {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, ok, err := queue.Dequeue(context.Background())
				if err != nil || !ok {
					time.Sleep(200 * time.Microsecond)
				}
			}
		}()
	}
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	baseGoroutines := runtime.NumGoroutine()
	baseFDs := soakOpenFDs()
	fdsCached := baseFDs
	start := time.Now()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	sampleIndex := 0
	maxResidue := 0
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(start)
			if elapsed >= time.Duration(seconds)*time.Second {
				goto done
			}
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			sampleIndex++
			residue := 0
			if sampleIndex%20 == 0 {
				fdsCached = soakOpenFDs()
				paused.Store(true)
				time.Sleep(40 * time.Millisecond)
				for {
					_, ok, err := queue.Dequeue(context.Background())
					if err != nil || !ok {
						break
					}
				}
				residue = queue.Len()
				if residue > maxResidue {
					maxResidue = residue
				}
				activeTimers.Add(1)
				if activeTimers.Load() > timersMax.Load() {
					timersMax.Store(activeTimers.Load())
				}
				time.AfterFunc(5*time.Millisecond, func() { activeTimers.Add(-1) })
				paused.Store(false)
			}
			emit("m07-s08-sample %d %d %d %d %d %d %d\n",
				elapsed.Milliseconds(), stats.HeapAlloc, runtime.NumGoroutine(),
				fdsCached, activeTimers.Load(), queue.Len(), residue)
		}
	}
done:
	close(stop)
	producers.Wait()
	consumers.Wait()
	for {
		_, ok, err := queue.Dequeue(context.Background())
		if err != nil || !ok {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	residue := queue.Len() + int(activeTimers.Load())
	emit("m07-s08-detail window_seconds=%d samples=%d base_goroutines=%d base_fds=%d base_heap=%d residue=%d timers_max=%d max_residue=%d\n",
		seconds, sampleIndex, baseGoroutines, baseFDs, baseline.HeapAlloc, residue, timersMax.Load(), maxResidue)
	if residue != 0 {
		t.Fatalf("soak residue=%d", residue)
	}
}
