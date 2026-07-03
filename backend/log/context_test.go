// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package log

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestCtxWithTrace_RoundTrip(t *testing.T) {
	tf := TraceFields{
		RequestID:     uuid.New().String(),
		CorrelationID: uuid.New().String(),
		CausationID:   uuid.New().String(),
		User:          "user-1",
		Character:     "char-1",
		Conversation:  "conv-1",
		Channel:       "web",
		StateVersion:  "v1",
		Path:          "GET /api/test",
		Stage:         "handler",
	}

	ctx := context.Background()
	ctx = CtxWithTrace(ctx, tf)

	result := FromContext(ctx)

	if result.RequestID != tf.RequestID {
		t.Errorf("request_id mismatch: got %s, want %s", result.RequestID, tf.RequestID)
	}
	if result.CorrelationID != tf.CorrelationID {
		t.Errorf("correlation_id mismatch: got %s, want %s", result.CorrelationID, tf.CorrelationID)
	}
	if result.CausationID != tf.CausationID {
		t.Errorf("causation_id mismatch: got %s, want %s", result.CausationID, tf.CausationID)
	}
	if result.User != tf.User {
		t.Errorf("user mismatch: got %s, want %s", result.User, tf.User)
	}
	if result.Character != tf.Character {
		t.Errorf("character mismatch: got %s, want %s", result.Character, tf.Character)
	}
	if result.Conversation != tf.Conversation {
		t.Errorf("conversation mismatch: got %s, want %s", result.Conversation, tf.Conversation)
	}
	if result.Channel != tf.Channel {
		t.Errorf("channel mismatch: got %s, want %s", result.Channel, tf.Channel)
	}
	if result.StateVersion != tf.StateVersion {
		t.Errorf("state_version mismatch: got %s, want %s", result.StateVersion, tf.StateVersion)
	}
	if result.Path != tf.Path {
		t.Errorf("path mismatch: got %s, want %s", result.Path, tf.Path)
	}
	if result.Stage != tf.Stage {
		t.Errorf("stage mismatch: got %s, want %s", result.Stage, tf.Stage)
	}
}

func TestFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	result := FromContext(ctx)
	if result.RequestID != "" {
		t.Error("expected empty TraceFields for empty context")
	}
}

func TestFromGin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("trace_request_id", "req-1")
	c.Set("trace_correlation_id", "corr-1")
	c.Set("trace_causation_id", "caus-1")
	c.Set("trace_path", "GET /test")

	result := FromGin(c)

	if result.RequestID != "req-1" {
		t.Errorf("request_id mismatch: got %s", result.RequestID)
	}
	if result.CorrelationID != "corr-1" {
		t.Errorf("correlation_id mismatch: got %s", result.CorrelationID)
	}
	if result.CausationID != "caus-1" {
		t.Errorf("causation_id mismatch: got %s", result.CausationID)
	}
	if result.Path != "GET /test" {
		t.Errorf("path mismatch: got %s", result.Path)
	}
}

func TestFromGin_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)

	result := FromGin(c)
	if result.RequestID != "" {
		t.Error("expected empty TraceFields for empty gin context")
	}
}

func TestMergeTrace(t *testing.T) {
	original := TraceFields{
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		User:          "user-1",
		Stage:         "start",
	}

	updates := TraceFields{
		Character:    "char-2",
		Conversation: "conv-2",
		Stage:        "process",
	}

	result := MergeTrace(original, updates)

	if result.RequestID != "req-1" {
		t.Errorf("request_id should be preserved")
	}
	if result.Character != "char-2" {
		t.Errorf("character should be updated")
	}
	if result.Conversation != "conv-2" {
		t.Errorf("conversation should be updated")
	}
	if result.Stage != "process" {
		t.Errorf("stage should be updated")
	}
	if result.User != "user-1" {
		t.Errorf("user should be preserved")
	}
}

func TestMergeTrace_EmptyUpdates(t *testing.T) {
	original := TraceFields{
		RequestID: "req-1",
		Path:      "GET /test",
	}

	result := MergeTrace(original, TraceFields{})

	if result.RequestID != "req-1" {
		t.Errorf("request_id should be preserved")
	}
	if result.Path != "GET /test" {
		t.Errorf("path should be preserved")
	}
}

func TestClone(t *testing.T) {
	tf := TraceFields{
		RequestID: "req-1",
		Character: "char-1",
		Channel:   "web",
		Stage:     "start",
	}

	clone := tf.Clone()
	clone.Stage = "modified"

	if tf.Stage != "start" {
		t.Error("original should not be affected by clone modification")
	}
	if clone.Stage != "modified" {
		t.Error("clone should have new value")
	}
}
