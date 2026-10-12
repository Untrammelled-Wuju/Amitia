package ui_handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
)

func TestSchemaHandlerUsesRendererAndPreservesOwnership(t *testing.T) {
	for _, tc := range []struct {
		name       string
		kind       ui_contribution.UIContributionKind
		entry      ui_contribution.UISandboxType
		sandbox    ui_contribution.UISandboxType
		path       string
		extension  string
		registered bool
		want       int
		wantLookup bool
	}{
		{"page", ui_contribution.UIContributionSchemaPage, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/page.json", "com.amitia%2Femote", true, 200, true},
		{"composer", ui_contribution.UIContributionComposerAction, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/composer.json", "com.amitia%2Femote", true, 200, true},
		{"message_action", ui_contribution.UIContributionMessageAction, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/message.json", "com.amitia%2Femote", true, 200, true},
		{"message_renderer", ui_contribution.UIContributionMessageRenderer, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/message.json", "com.amitia%2Femote", true, 200, true},
		{"panel", ui_contribution.UIContributionPanel, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/panel.json", "com.amitia%2Femote", true, 200, true},
		{"wrong_extension", ui_contribution.UIContributionComposerAction, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/composer.json", "com.other%2Fplugin", true, 404, false},
		{"web_entry", ui_contribution.UIContributionComposerAction, ui_contribution.SandboxWebRestricted, ui_contribution.SandboxSchemaRenderer, "ui/composer.json", "com.amitia%2Femote", true, 404, false},
		{"web_sandbox", ui_contribution.UIContributionComposerAction, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxWebRestricted, "ui/composer.json", "com.amitia%2Femote", true, 404, false},
		{"missing_schema_path", ui_contribution.UIContributionSchemaPage, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "", "com.amitia%2Femote", true, 404, false},
		{"schema_not_loaded", ui_contribution.UIContributionComposerAction, ui_contribution.SandboxSchemaRenderer, ui_contribution.SandboxSchemaRenderer, "ui/composer.json", "com.amitia%2Femote", false, 404, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := ui_contribution.NewUIHost()
			def := &ui_contribution.UIContributionDefinition{
				ContributionID:  "emote-composer",
				ExtensionID:     "com.amitia/emote",
				ModuleID:        "emote-ui",
				Kind:            tc.kind,
				Slot:            ui_contribution.UISlotReference{SlotID: "test.schema", ContractVersion: 1},
				ContractVersion: 1,
				Entry:           ui_contribution.UIEntryDefinition{Type: tc.entry, Path: "ui/entry", SchemaPath: tc.path, ContentHash: "test-content"},
				Sandbox:         ui_contribution.UISandboxPolicy{Type: tc.sandbox},
				Integrity:       ui_contribution.ContributionIntegrity{DefinitionHash: "test-definition", Generation: 1},
			}
			if err := host.RegisterContribution(def); err != nil {
				t.Fatal(err)
			}
			lookupCalled := false
			handler := &HTTPHandler{uiHost: host}
			handler.SetSchemaLookup(func(extension, contribution string) (json.RawMessage, bool) {
				lookupCalled = true
				if extension != "com.amitia/emote" || contribution != "emote-composer" {
					t.Fatalf("unexpected schema identity %q %q", extension, contribution)
				}
				return json.RawMessage(`{"schemaVersion":"schema-ui/1"}`), tc.registered
			})
			request := httptest.NewRequest(http.MethodGet, "/api/extensions/ui/schema/"+tc.extension+"/emote-composer", nil)
			recorder := httptest.NewRecorder()
			mux := http.NewServeMux()
			handler.Register(mux)
			mux.ServeHTTP(recorder, request)
			if recorder.Code != tc.want || lookupCalled != tc.wantLookup {
				t.Fatalf("status=%d lookup=%t, want status=%d lookup=%t: %s", recorder.Code, lookupCalled, tc.want, tc.wantLookup, recorder.Body.String())
			}
			if recorder.Code == http.StatusOK {
				var response struct {
					ExtensionID    string          `json:"extensionId"`
					ContributionID string          `json:"contributionId"`
					SchemaPath     string          `json:"schemaPath"`
					Document       json.RawMessage `json:"document"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.ExtensionID != "com.amitia/emote" || response.ContributionID != "emote-composer" || response.SchemaPath != tc.path || string(response.Document) != `{"schemaVersion":"schema-ui/1"}` {
					t.Fatalf("unexpected schema response: %s", recorder.Body.String())
				}
			}
		})
	}
}
