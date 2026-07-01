package psyche_testdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Case struct {
	ID             string                 `json:"id"`
	Category       string                 `json:"category"`
	Priority       string                 `json:"priority"`
	Clock          string                 `json:"clock"`
	InputEvent     map[string]interface{} `json:"inputEvent"`
	PreState       map[string]interface{} `json:"preState"`
	TaskPriority   string                 `json:"taskPriority"`
	AllowedDelta   map[string]interface{} `json:"allowedDelta"`
	Forbidden      []string               `json:"forbidden"`
	ExpectedState  string                 `json:"expectedState"`
	OutputFeatures []string               `json:"outputFeatures"`
	FakeLLM        string                 `json:"fakeLLM"`
	FakeChannel    string                 `json:"fakeChannel"`
	Faults         []string               `json:"faults"`
}

func LoadCases(path string) ([]Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, err
	}
	for i, c := range cases {
		if c.ID == "" {
			return nil, fmt.Errorf("case %d missing id", i)
		}
		if c.Category == "" {
			return nil, fmt.Errorf("%s missing category", c.ID)
		}
		if c.Clock == "" {
			return nil, fmt.Errorf("%s missing clock", c.ID)
		}
		if c.FakeLLM == "" {
			return nil, fmt.Errorf("%s missing fakeLLM", c.ID)
		}
		if c.FakeChannel == "" {
			return nil, fmt.Errorf("%s missing fakeChannel", c.ID)
		}
	}
	return cases, nil
}

func DefaultCasesPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "cases.json"
	}
	return filepath.Join(filepath.Dir(file), "cases.json")
}
