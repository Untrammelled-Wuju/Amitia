package mindruntime

import "testing"

func TestRuntimeMetricsCacheTokenWeighting(t *testing.T) {
	c := NewRuntimeMetricsCollector()
	if c.Snapshot().CacheHitRate != nil {
		t.Fatal("empty samples must be unknown")
	}
	c.RecordModelUsage(100, 0, true)
	c.RecordModelUsage(900, 900, true)
	c.RecordModelUsage(500, 0, false)
	c.RecordModelUsage(100, 101, true)
	s := c.Snapshot()
	if s.CacheHitRate == nil || *s.CacheHitRate != 0.9 || s.ModelInputTokens != 1600 || s.CacheReportedInputTokens != 1000 || s.CachedInputTokens != 900 || s.UncachedInputTokens != 100 || s.CacheUsageSamples != 2 || s.UnknownCacheUsageCalls != 2 || s.TotalModelCalls != 4 {
		t.Fatalf("wrong weighted metrics: %+v", s)
	}
	c.Reset()
	s = c.Snapshot()
	if s.CacheHitRate != nil || s.ModelInputTokens != 0 || s.TotalModelCalls != 0 || s.CacheUsageSamples != 0 || s.UnknownCacheUsageCalls != 0 {
		t.Fatalf("reset retained samples: %+v", s)
	}
}

func TestRuntimeMetricsExplicitZeroCacheUsage(t *testing.T) {
	c := NewRuntimeMetricsCollector()
	c.RecordModelUsage(100, 0, true)
	s := c.Snapshot()
	if s.CacheHitRate == nil || *s.CacheHitRate != 0 {
		t.Fatalf("reported zero must remain zero: %+v", s)
	}
}
