package plugincontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

const ContractProtocolVersion = 1

var ErrUnsupportedContractProtocol = errors.New("plugincontract: unsupported contract protocol version")

type ContractDigest struct {
	ProtocolVersion int              `json:"protocolVersion"`
	ServiceID       string           `json:"serviceId"`
	Version         string           `json:"version"`
	Placement       ServicePlacement `json:"placement"`
	Scopes          []ServiceScope   `json:"scopes"`
	InputSchema     json.RawMessage  `json:"inputSchema"`
	OutputSchema    json.RawMessage  `json:"outputSchema"`
}

func CanonicalContractHash(digest ContractDigest) (string, error) {
	payload, err := CanonicalContractBytes(digest)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func CanonicalContractBytes(digest ContractDigest) ([]byte, error) {
	normalized, err := normalizeContractDigest(digest)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: encode canonical contract: %w", err)
	}
	return encoded, nil
}

func ValidateContractProtocolVersion(version int) error {
	if version != ContractProtocolVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedContractProtocol, version, ContractProtocolVersion)
	}
	return nil
}

func ValidateContractHash(raw string) error {
	trimmed := trimContractValue(raw)
	if trimmed == "" {
		return fmt.Errorf("plugincontract: contract hash is empty")
	}
	if !hashPattern.MatchString(trimmed) {
		return fmt.Errorf("plugincontract: contract hash %q must be 64 hex characters", trimmed)
	}
	return nil
}

func ContractHashForOffer(offer ServiceOffer, inputSchema, outputSchema json.RawMessage) (string, error) {
	return CanonicalContractHash(ContractDigest{
		ProtocolVersion: ContractProtocolVersion,
		ServiceID:       offer.ServiceID,
		Version:         offer.Version,
		Placement:       offer.Placement,
		Scopes:          offer.Scopes,
		InputSchema:     inputSchema,
		OutputSchema:    outputSchema,
	})
}

func normalizeContractDigest(digest ContractDigest) (ContractDigest, error) {
	if err := ValidateContractProtocolVersion(digest.ProtocolVersion); err != nil {
		return ContractDigest{}, err
	}
	serviceID := trimContractValue(digest.ServiceID)
	if err := ValidateServiceID(serviceID); err != nil {
		return ContractDigest{}, err
	}
	version := trimContractValue(digest.Version)
	if version == "" {
		return ContractDigest{}, fmt.Errorf("plugincontract: contract version is required for %s", serviceID)
	}
	placement := ServicePlacement(trimContractValue(string(digest.Placement)))
	if !placement.Valid() {
		return ContractDigest{}, fmt.Errorf("plugincontract: unsupported placement %q for %s", placement, serviceID)
	}
	scopes, err := canonicalScopes(digest.Scopes)
	if err != nil {
		return ContractDigest{}, err
	}
	input, err := canonicalJSONOrNull(digest.InputSchema, "inputSchema")
	if err != nil {
		return ContractDigest{}, err
	}
	output, err := canonicalJSONOrNull(digest.OutputSchema, "outputSchema")
	if err != nil {
		return ContractDigest{}, err
	}
	return ContractDigest{
		ProtocolVersion: ContractProtocolVersion,
		ServiceID:       serviceID,
		Version:         version,
		Placement:       placement,
		Scopes:          scopes,
		InputSchema:     input,
		OutputSchema:    output,
	}, nil
}

func canonicalScopes(scopes []ServiceScope) ([]ServiceScope, error) {
	seen := make(map[ServiceScope]bool, len(scopes))
	out := make([]ServiceScope, 0, len(scopes))
	for _, scope := range scopes {
		trimmed := ServiceScope(trimContractValue(string(scope)))
		if trimmed == "" {
			return nil, fmt.Errorf("plugincontract: empty scope in contract")
		}
		if !trimmed.Valid() {
			return nil, fmt.Errorf("plugincontract: unsupported scope %q in contract", trimmed)
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	sort.Slice(out, func(left, right int) bool { return out[left] < out[right] })
	return out, nil
}

func canonicalJSONOrNull(raw json.RawMessage, field string) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage("null"), nil
	}
	canonical, err := CanonicalizeJSONValue(raw)
	if err != nil {
		return nil, fmt.Errorf("plugincontract: canonicalize %s: %w", field, err)
	}
	return json.RawMessage(canonical), nil
}

func CanonicalizeJSONValue(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("plugincontract: unexpected trailing content after json value")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func trimContractValue(raw string) string {
	return string(bytes.TrimSpace([]byte(raw)))
}
