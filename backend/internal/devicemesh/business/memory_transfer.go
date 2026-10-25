package business

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type MemoryTransferInput struct {
	Kind             string `json:"kind"`
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expectedRevision"`
	TargetScope      string `json:"targetScope"`
	TargetRoleID     string `json:"targetRoleId,omitempty"`
}

func (e *Engine) TransferMemory(ctx context.Context, request Request, input MemoryTransferInput) (coordination.Acknowledgement, error) {
	ctx, scope, finish, err := e.memoryManagementScope(ctx, request, true)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	defer finish()
	if input.ID == "" || !coordination.MemoryKind(input.Kind) {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	target := coordination.GlobalMemoryRole
	targetRevision := int64(0)
	if input.TargetScope == "character" {
		if input.TargetRoleID == "" {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
		roles, err := e.data.Roles(ctx, scope)
		if err != nil {
			return coordination.Acknowledgement{}, err
		}
		role, err := coordination.ResolveRole(input.TargetRoleID, roles)
		if err != nil {
			return coordination.Acknowledgement{}, err
		}
		target, targetRevision = role.ID, role.Revision
	} else if input.TargetScope != "global" {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	if input.ExpectedRevision < 1 {
		return coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	port, ok := e.data.(coordination.ResourcePort)
	if !ok {
		return coordination.Acknowledgement{}, coordination.ErrWrongOwner
	}
	selected, err := port.Resource(ctx, scope, input.Kind, input.ID)
	if err != nil {
		return coordination.Acknowledgement{}, err
	}
	if selected == nil || selected.Deleted || selected.Revision != input.ExpectedRevision {
		return coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	root := selected
	if selected.Kind != "memory" && selected.Kind != "continuity" {
		if selected.SourceID == "" {
			return coordination.Acknowledgement{}, errors.New("此记录缺少源记忆，无法迁移")
		}
		root, err = port.Resource(ctx, scope, "memory", selected.SourceID)
		if err != nil {
			return coordination.Acknowledgement{}, err
		}
	}
	if root == nil || root.Deleted || root.RoleID == target {
		return coordination.Acknowledgement{}, coordination.ErrResourceVersion
	}
	records := []coordination.Resource{*root}
	if root.Kind == "memory" {
		for _, kind := range []string{"fact", "episodic", "vector", "graph"} {
			snapshot, err := e.memoryManagementSnapshot(ctx, scope, kind, "")
			if err != nil {
				return coordination.Acknowledgement{}, err
			}
			for _, row := range snapshot.Resources {
				if row.SourceID == root.ID {
					records = append(records, row)
				}
			}
		}
	}
	mutations := make([]coordination.Mutation, 0, len(records))
	for _, row := range records {
		var document map[string]json.RawMessage
		if json.Unmarshal(row.Body, &document) != nil {
			return coordination.Acknowledgement{}, coordination.ErrWrongOwner
		}
		document["memoryPartition"] = body(target)
		document["previousMemoryPartition"] = body(row.RoleID)
		if row.Kind == "continuity" {
			var thread map[string]json.RawMessage
			if json.Unmarshal(document["thread"], &thread) != nil {
				return coordination.Acknowledgement{}, coordination.ErrWrongOwner
			}
			thread["revision"] = body(row.Revision + 1)
			document["thread"] = body(thread)
		}
		mutations = append(mutations, coordination.Mutation{Kind: row.Kind, ID: row.ID, RoleID: target, SourceID: row.SourceID, ExpectedRevision: row.Revision, Body: body(document)})
	}
	ack, err := e.commit(ctx, coordination.Commit{Scope: scope, MemoryTransfer: &coordination.MemoryTransfer{Kind: root.Kind, ID: root.ID, ToRoleID: target, TargetRoleRevision: targetRevision}, Mutations: mutations})
	if err == nil && root.Kind == "continuity" {
		var document OwnedContinuity
		if json.Unmarshal(root.Body, &document) == nil {
			managed := e.PresentManagedContinuity(ctx, document, scope, target)
			if managed.ManagementScope != nil && !managed.ReadOnly {
				err = e.coordination.TrackContinuity(ctx, *managed.ManagementScope, root.ID, ack.Versions["continuity/"+root.ID])
			}
		}
	}
	return ack, err
}
