package deepsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

const (
	DeepSearchTaskID      = "system.search.deep"
	DeepSearchExtensionID = "amitia.system.search"
	DeepSearchModuleID    = "deep_search"
	DeepSearchEntry       = "tasks/deep-search/index.js"
)

func BuildTaskDefinition(entry string) task_runtime.TaskDefinition {
	if entry == "" {
		entry = DeepSearchEntry
	}

	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "minLength": 1, "maxLength": 2048},
			"focusAreas": {
				"type": "array",
				"maxItems": 8,
				"items": {"type": "string", "maxLength": 256}
			},
			"maxRounds": {"type": "integer", "minimum": 1, "maximum": 5},
			"maxQueriesPerRound": {"type": "integer", "minimum": 1, "maximum": 6},
			"resultsPerQuery": {"type": "integer", "minimum": 1, "maximum": 20},
			"maxSources": {"type": "integer", "minimum": 1, "maximum": 100},
			"language": {"type": "string"},
			"country": {"type": "string"},
			"safeSearch": {"enum": ["off", "moderate", "strict"]}
		}
	}`)

	checkpointSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"version": {"type": "integer"},
			"round": {"type": "integer"},
			"queryIndex": {"type": "integer"},
			"searchCalls": {"type": "integer"},
			"sourcePool": {"type": "array"},
			"focusAreas": {"type": "array"},
			"executedQueries": {"type": "array"},
			"completedQueries": {"type": "array"}
		}
	}`)

	return task_runtime.TaskDefinition{
		TaskID:         DeepSearchTaskID,
		ExtensionID:    DeepSearchExtensionID,
		ModuleID:       DeepSearchModuleID,
		RuntimeType:    string(task_runtime.RuntimeTaskJavaScript),
		Entry:          entry,
		InputSchema:    inputSchema,
		CheckpointSchema: checkpointSchema,
		Checkpoint:     true,
		Idempotency:    task_runtime.ConditionallyIdempotent,
		Recoverability: task_runtime.CheckpointRecoverable,
		RetryPolicy: task_runtime.TaskRetryPolicy{
			MaxAttempts:    1,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     5 * time.Second,
			Multiplier:     1.0,
		},
		TimeoutPolicy: task_runtime.TaskTimeoutPolicy{
			DefaultTimeout: 5 * time.Minute,
			MaxTimeout:     15 * time.Minute,
			HardKillAfter:  30 * time.Second,
		},
		ResultPolicy: task_runtime.ResultAuto,
		CleanupPolicy: task_runtime.CleanupAlways,
		ResourceLimits: task_runtime.TaskResourceLimits{
			MaxMemoryMB:        256,
			MaxCPUPercent:      50,
			MaxDiskMB:          128,
			MaxOutputSizeMB:    1,
			MaxLogSizeMB:       4,
			MaxConcurrentTasks: 2,
		},
		MaxDuration:   15 * time.Minute,
	}
}

func DefinitionHash(def task_runtime.TaskDefinition) string {
	h := sha256.New()
	h.Write([]byte(def.TaskID))
	h.Write([]byte(def.ExtensionID))
	h.Write([]byte(def.RuntimeType))
	h.Write([]byte(def.Entry))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
