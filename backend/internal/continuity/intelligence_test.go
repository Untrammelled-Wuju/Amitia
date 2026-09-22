package continuity

import (
	"context"
	"testing"
)

type staticJSONGenerator struct {
	value string
	err   error
}

func (g staticJSONGenerator) GenerateWorkshopJSON(context.Context, string, string) (string, string, string, error) {
	return g.value, "", "", g.err
}

func TestIntelligenceExtractsStructuredThreadPatch(t *testing.T) {
	intel := NewIntelligence(staticJSONGenerator{value: `{"stateChanged":true,"status":"waiting","currentState":"后端已经部署","nextAction":"等待 DNS 后切流量","confidence":0.93,"eventType":"deployment.backend_completed","eventSummary":"后端部署完成","waits":[{"action":"create","type":"external","description":"等待 DNS","condition":{"dnsRecord":"api.example.com"},"autoResume":true}]}`})
	patch, err := intel.Extract(context.Background(), &Thread{ID: "t1", Title: "服务器迁移", Status: ThreadStatusActive}, nil, ObservePayload{UserMessage: "后端好了，等 DNS"})
	if err != nil {
		t.Fatal(err)
	}
	if !patch.StateChanged || patch.Status != ThreadStatusWaiting || patch.NextAction == "" || len(patch.Waits) != 1 {
		t.Fatalf("unexpected patch: %#v", patch)
	}
	if patch.Waits[0].Type != WaitTypeExternal {
		t.Fatalf("wait type=%s", patch.Waits[0].Type)
	}
}

func TestIntelligenceDropsInvalidWaitAndStatus(t *testing.T) {
	intel := NewIntelligence(staticJSONGenerator{value: `{"stateChanged":true,"status":"invented","waits":[{"action":"create","type":"magic","description":"bad"}]}`})
	patch, err := intel.Extract(context.Background(), &Thread{ID: "t1", Title: "x", Status: ThreadStatusActive}, nil, ObservePayload{})
	if err != nil {
		t.Fatal(err)
	}
	if patch.Status != "" || len(patch.Waits) != 0 {
		t.Fatalf("invalid model output was not normalized: %#v", patch)
	}
}
