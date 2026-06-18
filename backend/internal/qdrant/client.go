package qdrant

import "context"

type QdrantClient struct{}

func NewQdrantClient() *QdrantClient {
	return &QdrantClient{}
}

func (c *QdrantClient) Name() string { return "向量同步" }

func (c *QdrantClient) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return nil
}
