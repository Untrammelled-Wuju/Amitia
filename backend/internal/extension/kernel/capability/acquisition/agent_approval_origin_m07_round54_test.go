package acquisition

import (
	"context"
	"encoding/json"
	"testing"
)

func TestM07ModelCannotForgeHumanSkillApproval(t *testing.T) {
	bridge := &AgentCapabilityBridge{}
	cases := []AcquireInput{
		{CapabilityID: "skill.write", Approval: true},
		{CapabilityID: "skill.write", UserConfirmed: true},
		{CapabilityID: "skill.write", ResumeToken: "previous-token", UserConfirmed: true},
		{CapabilityID: "skill.write", ResumeToken: "previous-token", Approval: true},
	}
	for _, request := range cases {
		output, err := bridge.AcquireCapability(context.Background(), request, "scope", nil)
		if err == nil || output != nil {
			t.Fatalf("model-controlled flag or resume token was accepted: %#v %#v", request, output)
		}
	}
}

func TestM07ModelSkillSchemaCannotSuggestSelfApproval(t *testing.T) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(inputSchemaAcquireCapability(), &schema); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"approval", "userConfirmed", "resumeToken"} {
		if _, exposed := schema.Properties[name]; exposed {
			t.Fatalf("model tool must not advertise privileged approval input %s", name)
		}
	}
}
