package coordination

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

func continuityExecutionBindings(body json.RawMessage, id string) (map[string]bool, error) {
	var document struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		Bindings []struct {
			Type     string `json:"bindingType"`
			ID       string `json:"bindingId"`
			ThreadID string `json:"threadId"`
			Role     string `json:"role"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, binding := range document.Bindings {
		if binding.Type != "task_run" || binding.Role != "execution" {
			continue
		}
		if binding.ID == "" || len(binding.ID) > 160 || strings.TrimSpace(binding.ID) != binding.ID || binding.ThreadID != id || document.Thread.ID != id || result[binding.ID] {
			return nil, errors.New("持续事项任务执行绑定身份无效")
		}
		result[binding.ID] = true
	}
	return result, nil
}

func (s *OwnershipStore) newContinuityExecutionBindings(ctx context.Context, tx *sql.Tx, mutations []Mutation) (map[string]bool, error) {
	added := map[string]bool{}
	for _, mutation := range mutations {
		if mutation.Kind != "continuity" || mutation.Deleted {
			continue
		}
		next, err := continuityExecutionBindings(mutation.Body, mutation.ID)
		if err != nil {
			return nil, err
		}
		if len(next) == 0 {
			continue
		}
		var previous json.RawMessage
		err = tx.QueryRowContext(ctx, `SELECT body FROM kernel_device_owned_resources WHERE owner_id=? AND kind='continuity' AND resource_id=? AND deleted=0`, s.ownerID, mutation.ID).Scan(&previous)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		old := map[string]bool{}
		if err == nil {
			old, err = continuityExecutionBindings(previous, mutation.ID)
			if err != nil {
				return nil, err
			}
		}
		for task := range next {
			if !old[task] {
				added[task] = true
			}
		}
	}
	return added, nil
}

func (s *OwnershipStore) validateContinuityExecutionBindings(ctx context.Context, tx *sql.Tx, tasks map[string]bool) error {
	for task := range tasks {
		var count int
		err := tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT resource.resource_id) FROM kernel_device_owned_resources resource, json_each(CAST(resource.body AS TEXT),'$.bindings') binding WHERE resource.owner_id=? AND resource.kind='continuity' AND resource.deleted=0 AND json_extract(binding.value,'$.bindingType')='task_run' AND json_extract(binding.value,'$.role')='execution' AND json_extract(binding.value,'$.bindingId')=?`, s.ownerID, task).Scan(&count)
		if err != nil {
			return err
		}
		if count > 1 {
			return ErrRequestConflict
		}
	}
	return nil
}
