package permission

import "encoding/json"

type PermissionCondition struct {
	Field    string          `json:"field"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
}

type NetworkCondition struct {
	Domains []string `json:"domains,omitempty"`
	Methods []string `json:"methods,omitempty"`
	Ports   []int    `json:"ports,omitempty"`
}

type FileCondition struct {
	Paths      []string `json:"paths,omitempty"`
	Operations []string `json:"operations,omitempty"`
}

type UICondition struct {
	Slots []string `json:"slots,omitempty"`
}

type MessageCondition struct {
	Channels   []string `json:"channels,omitempty"`
	MaxPerHour int      `json:"maxPerHour,omitempty"`
}

func ParseNetworkCondition(data json.RawMessage) (NetworkCondition, error) {
	var c NetworkCondition
	err := json.Unmarshal(data, &c)
	return c, err
}

func ParseFileCondition(data json.RawMessage) (FileCondition, error) {
	var c FileCondition
	err := json.Unmarshal(data, &c)
	return c, err
}

func ParseUICondition(data json.RawMessage) (UICondition, error) {
	var c UICondition
	err := json.Unmarshal(data, &c)
	return c, err
}
