package system

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type storageStatsTestPort struct{}

func (storageStatsTestPort) Read(context.Context, string, string, bool) ([]business.MemoryConsumerEntry, error) {
	return []business.MemoryConsumerEntry{{Resource: coordination.Resource{ID: "memory-one", Kind: "memory", Body: json.RawMessage(`{}`)}}}, nil
}

func (storageStatsTestPort) Change(context.Context, business.MemoryConsumerEntry, map[string]json.RawMessage, bool, string) (coordination.Acknowledgement, error) {
	return coordination.Acknowledgement{}, nil
}

func (storageStatsTestPort) OwnedStorageStats(context.Context) (business.OwnedStorageStats, error) {
	return business.OwnedStorageStats{ConversationCount: 3, MessageCount: 5, DatabaseBytes: 4096}, nil
}

func TestStorageInfoUsesOwnedResourceCountsAndCombinedDatabaseSize(t *testing.T) {
	svc := &service{dataDir: t.TempDir(), memoryConsumer: storageStatsTestPort{}}
	result := svc.GetStorageInfo()
	if result["conversationCount"] != int64(3) || result["messageCount"] != int64(5) || result["memoryCount"] != 1 {
		t.Fatalf("unexpected owned storage counts: %#v", result)
	}
	if result["kernelDbSizeBytes"] != int64(4096) || result["scope"] != "all-roles" {
		t.Fatalf("unexpected storage scope: %#v", result)
	}
}
