// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/surrealdb/surrealdb.go"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
)

type Client struct {
	db *surrealdb.DB
	mu sync.Mutex
}

func NewClient(cfg config.SurrealConfig) (*Client, error) {
	url := fmt.Sprintf("ws://%s:%d/rpc", cfg.Host, cfg.Port)
	db, err := surrealdb.New(url)
	if err != nil {
		return nil, fmt.Errorf("surrealdb connect: %w", err)
	}

	ctx := context.Background()

	if _, err := db.SignIn(ctx, map[string]string{
		"user": cfg.Username,
		"pass": cfg.Password,
	}); err != nil {
		log.Warn("SurrealDB登录失败，尝试root/root:", err)
		if _, err2 := db.SignIn(ctx, map[string]string{
			"user": "root",
			"pass": "root",
		}); err2 != nil {
			return nil, fmt.Errorf("surrealdb signin: %w", err2)
		}
	}

	if err := db.Use(ctx, cfg.Namespace, cfg.Database); err != nil {
		return nil, fmt.Errorf("surrealdb use: %w", err)
	}

	c := &Client{db: db}
	if err := c.initSchema(); err != nil {
		log.Warn("SurrealDB schema init warning:", err)
	}

	return c, nil
}

func (c *Client) initSchema() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ctx := context.Background()

	queries := []string{
		"DEFINE TABLE entity_node SCHEMALESS",
		"DEFINE FIELD entity_type ON entity_node TYPE string",
		"DEFINE FIELD label ON entity_node TYPE string",
		"DEFINE FIELD properties ON entity_node TYPE object",
		"DEFINE TABLE entity_edge TYPE RELATION IN entity_node TO entity_node SCHEMALESS",
		"DEFINE FIELD relation_type ON entity_edge TYPE string",
		"DEFINE FIELD weight ON entity_edge TYPE float",
		"DEFINE INDEX idx_entity_type ON entity_node FIELDS entity_type",
		"DEFINE INDEX idx_relation_type ON entity_edge FIELDS relation_type",
	}

	for _, q := range queries {
		_, err := surrealdb.Query[any](ctx, c.db, q, nil)
		if err != nil {
			return fmt.Errorf("schema query: %s: %w", q, err)
		}
	}
	return nil
}

func (c *Client) Close() {
	if c.db != nil {
		c.db.Close(context.Background())
	}
}

func (c *Client) DB() *surrealdb.DB {
	return c.db
}
