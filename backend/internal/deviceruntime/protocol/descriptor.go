package protocol

import "fmt"

type Descriptor struct {
	Name            string
	EnvelopeVersion int
	SchemaVersion   string
}

func (d Descriptor) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("protocol name is required")
	}
	if d.EnvelopeVersion < 1 {
		return fmt.Errorf("envelope version must be >= 1")
	}
	if d.SchemaVersion == "" {
		return fmt.Errorf("schema version is required")
	}
	return nil
}

type RuntimeContractVersion string

func (v RuntimeContractVersion) String() string {
	return string(v)
}

func (v RuntimeContractVersion) IsEmpty() bool {
	return v == ""
}
