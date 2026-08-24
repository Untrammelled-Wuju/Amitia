package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodPermissionCheck    = "permission.check"
	MethodPermissionSnapshot = "permission.snapshot"
	MethodPermissionRequest  = "permission.request"
)

const (
	PermGameHostControl        = "gamehost.control"
	PermGameHostChannelUse     = "gamehost.channel.use"
	PermGameHostHostAPIInvoke  = "gamehost.host_api.invoke"
	PermGameHostArtifactDeploy = "gamehost.artifact.deploy"

	DecisionAllowed          = "allowed"
	DecisionDenied           = "denied"
	DecisionApprovalRequired = "approval_required"

	DenyReasonNotDeclared    = "not_declared"
	DenyReasonNotGranted     = "not_granted"
	DenyReasonScopeDenied    = "scope_denied"
	DenyReasonPolicyDenied   = "host_policy_denied"
	DenyReasonUnknownPerm    = "unknown_permission"
	DenyReasonInvalidSubject = "invalid_subject"
)

type PermissionCheckInput struct {
	PermissionID string `json:"permissionId"`
	ServiceID    string `json:"serviceId,omitempty"`
	RuntimeID    string `json:"runtimeId,omitempty"`
}

type PermissionCheckResult struct {
	PermissionID string `json:"permissionId"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type PermissionSnapshotInput struct {
	RuntimeID string `json:"runtimeId,omitempty"`
	ServiceID string `json:"serviceId,omitempty"`
}

type PermissionSnapshotResult struct {
	SnapshotID    string   `json:"snapshotId"`
	Revision      string   `json:"revision"`
	GrantedPerms  []string `json:"grantedPerms"`
	GrantedScopes []string `json:"grantedScopes"`
	ExpiresAt     int64    `json:"expiresAt,omitempty"`
	IsValid       bool     `json:"isValid"`
}

type PermissionRequestInput struct {
	PermissionID string `json:"permissionId"`
	ServiceID    string `json:"serviceId,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type PermissionRequestResult struct {
	PermissionID string `json:"permissionId"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason,omitempty"`
}

func (c *Client) CheckPermission(ctx context.Context, input PermissionCheckInput, opts ...MessageOption) (PermissionCheckResult, error) {
	envelope, err := c.SendRequest(ctx, MethodPermissionCheck, input, opts...)
	if err != nil {
		return PermissionCheckResult{}, err
	}
	var out PermissionCheckResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return PermissionCheckResult{}, NewEncodeError("unmarshal permission check response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) GetPermissionSnapshot(ctx context.Context, input PermissionSnapshotInput, opts ...MessageOption) (PermissionSnapshotResult, error) {
	envelope, err := c.SendRequest(ctx, MethodPermissionSnapshot, input, opts...)
	if err != nil {
		return PermissionSnapshotResult{}, err
	}
	var out PermissionSnapshotResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return PermissionSnapshotResult{}, NewEncodeError("unmarshal permission snapshot response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) RequestPermission(ctx context.Context, input PermissionRequestInput, opts ...MessageOption) (PermissionRequestResult, error) {
	envelope, err := c.SendRequest(ctx, MethodPermissionRequest, input, opts...)
	if err != nil {
		return PermissionRequestResult{}, err
	}
	var out PermissionRequestResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return PermissionRequestResult{}, NewEncodeError("unmarshal permission request response: %v", err)
		}
	}
	return out, nil
}
