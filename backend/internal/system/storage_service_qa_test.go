package system

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type storageStatsTestPort struct{ memoryCount int64 }

func (storageStatsTestPort) Read(context.Context, string, string, bool) ([]business.MemoryConsumerEntry, error) {
	return nil, errors.New("storage statistics must not read role-scoped data")
}

func (storageStatsTestPort) Change(context.Context, business.MemoryConsumerEntry, map[string]json.RawMessage, bool, string) (coordination.Acknowledgement, error) {
	return coordination.Acknowledgement{}, nil
}

func (p storageStatsTestPort) OwnedStorageStats(context.Context) (business.OwnedStorageStats, error) {
	return business.OwnedStorageStats{ConversationCount: 3, MessageCount: 5, MemoryCount: p.memoryCount, DatabaseBytes: 4096}, nil
}

func TestStorageInfoUsesOwnedResourceCountsAndCombinedDatabaseSize(t *testing.T) {
	svc := &service{dataDir: t.TempDir(), memoryConsumer: storageStatsTestPort{memoryCount: 1}}
	result := svc.GetStorageInfo()
	if result["conversationCount"] != int64(3) || result["messageCount"] != int64(5) || result["memoryCount"] != int64(1) {
		t.Fatalf("unexpected owned storage counts: %#v", result)
	}
	if result["kernelDbSizeBytes"] != int64(4096) || result["scope"] != "all-roles" {
		t.Fatalf("unexpected storage scope: %#v", result)
	}
}

func TestStorageInfoReportsZeroMemoryWithoutRole(t *testing.T) {
	svc := &service{dataDir: t.TempDir(), memoryConsumer: storageStatsTestPort{}}
	result := svc.GetStorageInfo()
	if result["memoryCount"] != int64(0) || result["memoryError"] != nil {
		t.Fatalf("unexpected empty memory statistics: %#v", result)
	}
}
