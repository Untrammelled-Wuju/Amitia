package memory

import "context"

import "testing"

type captureGraphService struct {
	nodes   []map[string]interface{}
	deleted []string
}

func (s *captureGraphService) SyncNode(entityType, entityID, label string, properties map[string]interface{}) error {
	s.nodes = append(s.nodes, map[string]interface{}{
		"type":       entityType,
		"id":         entityID,
		"label":      label,
		"properties": properties,
	})
	return nil
}

func (s *captureGraphService) SyncEdge(sourceID, targetID, relationType string, weight float64) error {
	return nil
}

func (s *captureGraphService) DeleteNode(entityID string) error {
	s.deleted = append(s.deleted, entityID)
	return nil
}

func (s *captureGraphService) DeleteNodeIfOrphan(entityID string) error {
	return nil
}

func (s *captureGraphService) DeleteNodesByProperty(entityType, propertyKey, propertyValue string) error {
	return nil
}

func (s *captureGraphService) QueryNeighbors(entityID string, depth int, userID string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *captureGraphService) FindPaths(sourceID, targetID string, maxDepth int) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *captureGraphService) DeleteOrphanNodes() error {
	return nil
}

func (s *captureGraphService) GetStats(userID string) (map[string]interface{}, error) {
	return nil, nil
}

func (s *captureGraphService) GetAllNodes(userID string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *captureGraphService) GetAllEdges(userID string) ([]map[string]interface{}, error) {
	return nil, nil
}

func (s *captureGraphService) Name() string {
	return "capture"
}

func (s *captureGraphService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}

func TestSyncGraphMemorySkipsDefaultUserScope(t *testing.T) {
	graphSvc := &captureGraphService{}
	svc := &service{graphSvc: graphSvc}

	svc.syncGraph(&Memory{
		ID:          "mem-a",
		CharacterID: "default",
		Scope:       "user",
		Key:         "称呼",
		Value:       "姐姐",
	})

	if len(graphSvc.nodes) != 0 {
		t.Fatalf("default scoped memory should not sync graph nodes: %+v", graphSvc.nodes)
	}
}

func TestSyncGraphMemoryUsesCharacterScopeAsUserScope(t *testing.T) {
	graphSvc := &captureGraphService{}
	svc := &service{graphSvc: graphSvc}

	svc.syncGraph(&Memory{
		ID:          "mem-a",
		CharacterID: "char-a",
		Scope:       "character",
		Key:         "称呼",
		Value:       "姐姐",
	})

	if len(graphSvc.nodes) == 0 {
		t.Fatalf("character scoped memory should sync graph nodes")
	}
	for _, node := range graphSvc.nodes {
		properties, _ := node["properties"].(map[string]interface{})
		if properties["user_id"] == "default" {
			t.Fatalf("graph node used default user scope: %+v", node)
		}
	}
}

func TestSyncGraphMemorySkipsBlockedStatusAndDeletesDerivedNode(t *testing.T) {
	graphSvc := &captureGraphService{}
	svc := &service{graphSvc: graphSvc}

	svc.syncGraph(&Memory{
		ID:             "mem-blocked",
		CharacterID:    "char-a",
		Scope:          "character",
		Key:            "关系",
		Value:          "旧关系叙事",
		VerifiedStatus: "invalidated",
	})

	if len(graphSvc.nodes) != 0 {
		t.Fatalf("blocked memory should not sync graph nodes: %+v", graphSvc.nodes)
	}
	if len(graphSvc.deleted) == 0 || graphSvc.deleted[0] != "memory:mem-blocked" {
		t.Fatalf("blocked memory should delete derived graph node: %+v", graphSvc.deleted)
	}
}

func TestSyncGraphMemorySkipsExpiredMemoryAndDeletesDerivedNode(t *testing.T) {
	graphSvc := &captureGraphService{}
	svc := &service{graphSvc: graphSvc}
	expired := "2000-01-01 00:00:00"

	svc.syncGraph(&Memory{
		ID:          "mem-expired",
		CharacterID: "char-a",
		Scope:       "character",
		Key:         "摘要",
		Value:       "旧摘要",
		ExpiresAt:   &expired,
	})

	if len(graphSvc.nodes) != 0 {
		t.Fatalf("expired memory should not sync graph nodes: %+v", graphSvc.nodes)
	}
	if len(graphSvc.deleted) == 0 || graphSvc.deleted[0] != "memory:mem-expired" {
		t.Fatalf("expired memory should delete derived graph node: %+v", graphSvc.deleted)
	}
}
