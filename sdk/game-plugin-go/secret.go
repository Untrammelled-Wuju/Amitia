package sdk

import (
	"context"
	"encoding/json"
)

const (
	MethodSecretAcquire = "secret.acquire"
	MethodSecretRelease = "secret.release"
	MethodSecretQuery   = "secret.query"
)

const (
	SecretPurposeStartup = "startup"
	SecretPurposeRuntime = "runtime"
)

type SecretRef string

type SecretAcquireInput struct {
	Ref      SecretRef `json:"ref"`
	Purpose  string    `json:"purpose"`
	Required bool      `json:"required"`
}

type SecretAcquireResult struct {
	LeaseID   string    `json:"leaseId"`
	Ref       SecretRef `json:"ref"`
	Purpose   string    `json:"purpose"`
	Granted   bool      `json:"granted"`
	ExpiresAt int64     `json:"expiresAt,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

type SecretReleaseInput struct {
	LeaseID string `json:"leaseId"`
	Reason  string `json:"reason,omitempty"`
}

type SecretReleaseResult struct {
	Released bool   `json:"released"`
	Reason   string `json:"reason,omitempty"`
}

type SecretQueryInput struct {
	LeaseID string `json:"leaseId"`
}

type SecretQueryResult struct {
	LeaseID   string    `json:"leaseId"`
	Ref       SecretRef `json:"ref,omitempty"`
	Granted   bool      `json:"granted"`
	ExpiresAt int64     `json:"expiresAt,omitempty"`
	Valid     bool      `json:"valid"`
}

func (c *Client) AcquireSecret(ctx context.Context, input SecretAcquireInput, opts ...MessageOption) (SecretAcquireResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodSecretAcquire, input, opts...)
	if err != nil {
		return SecretAcquireResult{}, err
	}
	var out SecretAcquireResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return SecretAcquireResult{}, NewEncodeError("unmarshal secret acquire response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) ReleaseSecret(ctx context.Context, input SecretReleaseInput, opts ...MessageOption) (SecretReleaseResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodSecretRelease, input, opts...)
	if err != nil {
		return SecretReleaseResult{}, err
	}
	var out SecretReleaseResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return SecretReleaseResult{}, NewEncodeError("unmarshal secret release response: %v", err)
		}
	}
	return out, nil
}

func (c *Client) QuerySecretLease(ctx context.Context, input SecretQueryInput, opts ...MessageOption) (SecretQueryResult, error) {
	envelope, err := c.SendReservedRequest(ctx, MethodSecretQuery, input, opts...)
	if err != nil {
		return SecretQueryResult{}, err
	}
	var out SecretQueryResult
	if len(envelope.Payload) > 0 {
		if err := json.Unmarshal(envelope.Payload, &out); err != nil {
			return SecretQueryResult{}, NewEncodeError("unmarshal secret query response: %v", err)
		}
	}
	return out, nil
}
