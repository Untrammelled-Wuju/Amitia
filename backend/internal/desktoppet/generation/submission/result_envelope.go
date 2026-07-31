package submission

import "encoding/json"

type ResultEnvelope struct {
	ProviderStatus  string                 `json:"providerStatus"`
	ImageURLs       []string               `json:"imageUrls,omitempty"`
	ImageBase64List []string               `json:"imageBase64List,omitempty"`
	RawResponseJSON string                 `json:"rawResponseJson,omitempty"`
	ResponseHash    string                 `json:"responseHash,omitempty"`
	CostEstimate    *CostEstimate          `json:"costEstimate,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CompletedAt     string                 `json:"completedAt,omitempty"`
	ErrorCode       string                 `json:"errorCode,omitempty"`
	ErrorMessage    string                 `json:"errorMessage,omitempty"`
}

type CostEstimate struct {
	InputUnits    int     `json:"inputUnits"`
	OutputUnits   int     `json:"outputUnits"`
	EstimatedCost float64 `json:"estimatedCost"`
	Currency      string  `json:"currency"`
}

func (e *ResultEnvelope) ToJSON() string {
	data, _ := json.Marshal(e)
	return string(data)
}

func ParseResultEnvelope(jsonStr string) (*ResultEnvelope, error) {
	var env ResultEnvelope
	err := json.Unmarshal([]byte(jsonStr), &env)
	if err != nil {
		return nil, err
	}
	return &env, nil
}
