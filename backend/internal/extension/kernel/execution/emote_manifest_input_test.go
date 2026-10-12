package execution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func TestEmoteSourceManifestAcceptsStrictSchemaUIInputs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "plugins", "emote", "amitia-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Modules []struct {
			Contributions []struct {
				ID   string `json:"id"`
				Spec struct {
					ToolID      string          `json:"toolId"`
					InputSchema json.RawMessage `json:"inputSchema"`
				} `json:"spec"`
			} `json:"contributions"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	var tool capability.ToolDefinition
	for _, module := range source.Modules {
		for _, contribution := range module.Contributions {
			if contribution.ID == "emote-command" {
				tool.ID = contribution.Spec.ToolID
				tool.InputSchema = contribution.Spec.InputSchema
			}
		}
	}
	if tool.ID == "" || len(tool.InputSchema) == 0 {
		t.Fatal("source manifest command input schema missing")
	}
	validator := NewInputValidator()
	for _, sample := range []struct {
		name, input string
		valid       bool
	}{
		{"minimal", `{"action":"ui.composer"}`, true},
		{"composer_mount", `{"action":"ui.composer","node_id":"lifecycle-onMount","form_state":{}}`, true},
		{"import_files", `{"action":"ui.import","payload":{"category":"测试"},"node_id":"import","form_state":{"category":"测试","enabled":true},"files":[{"name":"表情.png","mimeType":"image/png","size":8,"dataBase64":"iVBORw0KGgo="}]}`, true},
		{"empty_files", `{"action":"ui.import","files":[]}`, true},
		{"unknown_top_level", `{"action":"ui.composer","runtime":"other"}`, false},
		{"invalid_node", `{"action":"ui.composer","node_id":7}`, false},
		{"invalid_form", `{"action":"ui.composer","form_state":[]}`, false},
		{"invalid_files", `{"action":"ui.import","files":{}}`, false},
		{"missing_file_data", `{"action":"ui.import","files":[{"name":"a.png","mimeType":"image/png","size":1}]}`, false},
		{"unknown_file_property", `{"action":"ui.import","files":[{"name":"a.png","mimeType":"image/png","size":1,"dataBase64":"eA==","path":"/private"}]}`, false},
		{"negative_file_size", `{"action":"ui.import","files":[{"name":"a.png","mimeType":"image/png","size":-1,"dataBase64":"eA=="}]}`, false},
		{"fractional_file_size", `{"action":"ui.import","files":[{"name":"a.png","mimeType":"image/png","size":1.5,"dataBase64":"eA=="}]}`, false},
		{"wrong_file_data", `{"action":"ui.import","files":[{"name":"a.png","mimeType":"image/png","size":1,"dataBase64":true}]}`, false},
	} {
		t.Run(sample.name, func(t *testing.T) {
			err := validator.Validate(t.Context(), tool, json.RawMessage(sample.input))
			if (err == nil) != sample.valid {
				t.Fatalf("valid=%v: %v", sample.valid, err)
			}
		})
	}
}
