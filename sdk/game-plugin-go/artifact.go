package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodArtifactList           = "artifact.list"
	MethodArtifactDeployRequired = "artifact.deploy_required"
	MethodArtifactDeploy         = "artifact.deploy"
	MethodArtifactVerify         = "artifact.verify"
	MethodArtifactRemove         = "artifact.remove"
)

type ArtifactRequest struct {
	ArtifactID           string `json:"artifactId,omitempty"`
	TargetRoot           string `json:"targetRoot"`
	CompatibilityVersion string `json:"compatibilityVersion,omitempty"`
}

type ArtifactStatus struct {
	Artifact      PluginArtifact `json:"artifact"`
	Installed     bool           `json:"installed"`
	Healthy       bool           `json:"healthy"`
	TargetPath    string         `json:"targetPath,omitempty"`
	InstalledHash string         `json:"installedHash,omitempty"`
}

type ArtifactListResult struct {
	Items []ArtifactStatus `json:"items"`
}

type ArtifactRemoveResult struct {
	Removed bool `json:"removed"`
}

func (c *Client) ListArtifacts(ctx context.Context, input ArtifactRequest, opts ...MessageOption) (ArtifactListResult, error) {
	return c.artifactListRequest(ctx, MethodArtifactList, input, opts...)
}

func (c *Client) DeployRequiredArtifacts(ctx context.Context, input ArtifactRequest, opts ...MessageOption) (ArtifactListResult, error) {
	return c.artifactListRequest(ctx, MethodArtifactDeployRequired, input, opts...)
}

func (c *Client) DeployArtifact(ctx context.Context, input ArtifactRequest, opts ...MessageOption) (ArtifactStatus, error) {
	return c.artifactStatusRequest(ctx, MethodArtifactDeploy, input, opts...)
}

func (c *Client) VerifyArtifact(ctx context.Context, input ArtifactRequest, opts ...MessageOption) (ArtifactStatus, error) {
	return c.artifactStatusRequest(ctx, MethodArtifactVerify, input, opts...)
}

func (c *Client) RemoveArtifact(ctx context.Context, input ArtifactRequest, opts ...MessageOption) (ArtifactRemoveResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodArtifactRemove, input, opts...)
	if err != nil {
		return ArtifactRemoveResult{}, err
	}
	var out ArtifactRemoveResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ArtifactRemoveResult{}, NewEncodeError("unmarshal artifact remove response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) artifactListRequest(ctx context.Context, method string, input ArtifactRequest, opts ...MessageOption) (ArtifactListResult, error) {
	envelope, err := c.SendReservedRequest(ctx, method, input, opts...)
	if err != nil {
		return ArtifactListResult{}, err
	}
	var out ArtifactListResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ArtifactListResult{}, NewEncodeError("unmarshal artifact list response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) artifactStatusRequest(ctx context.Context, method string, input ArtifactRequest, opts ...MessageOption) (ArtifactStatus, error) {
	envelope, err := c.SendReservedRequest(ctx, method, input, opts...)
	if err != nil {
		return ArtifactStatus{}, err
	}
	var out ArtifactStatus
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return ArtifactStatus{}, NewEncodeError("unmarshal artifact response: %v", err)
		}
	}
	return out, nil
}
