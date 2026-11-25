package plugincontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const ColdStartSchemaVersion = 1

type catalogSnapshot struct {
	SchemaVersion int          `json:"schemaVersion"`
	Descriptors   []Descriptor `json:"descriptors"`
}

func (i *DescriptorIndex) MarshalColdStart() ([]byte, error) {
	if i == nil {
		return nil, fmt.Errorf("plugincontract: descriptor index is nil")
	}
	return json.Marshal(catalogSnapshot{SchemaVersion: ColdStartSchemaVersion, Descriptors: i.Snapshot()})
}

func RestoreDescriptorIndex(raw []byte, schemas *SchemaStore) (*DescriptorIndex, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("plugincontract: cold start snapshot is empty")
	}
	var snapshot catalogSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("plugincontract: cold start snapshot is not valid json: %w", err)
	}
	if snapshot.SchemaVersion != ColdStartSchemaVersion {
		return nil, fmt.Errorf("plugincontract: cold start snapshot schemaVersion=%d, want %d", snapshot.SchemaVersion, ColdStartSchemaVersion)
	}
	if snapshot.Descriptors == nil {
		return nil, fmt.Errorf("plugincontract: cold start snapshot has no descriptors field")
	}
	index := NewDescriptorIndex()
	if schemas != nil {
		index.schemas = schemas
	}
	for position, descriptor := range snapshot.Descriptors {
		if _, err := index.Add(descriptor); err != nil {
			return nil, fmt.Errorf("plugincontract: cold start restore failed at descriptor %d (%s): %w", position, strings.TrimSpace(descriptor.ID), err)
		}
	}
	return index, nil
}

func ColdStartDigest(index *DescriptorIndex) (string, error) {
	raw, err := index.MarshalColdStart()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
