package uitree

import (
	"testing"
)

func TestMapClassToRole(t *testing.T) {
	tests := []struct {
		name      string
		className string
		editable  bool
		clickable bool
		checkable bool
		expected  string
	}{
		{"Button", "android.widget.Button", false, false, false, "button"},
		{"EditText", "android.widget.EditText", false, false, false, "input"},
		{"TextView", "android.widget.TextView", false, false, false, "text"},
		{"ImageView", "android.widget.ImageView", false, false, false, "image"},
		{"WebView", "android.webkit.WebView", false, false, false, "web_view"},
		{"ScrollView", "android.widget.ScrollView", false, false, false, "scroll_view"},
		{"Checkbox", "android.widget.CheckBox", false, false, false, "checkbox"},
		{"Switch", "android.widget.Switch", false, false, false, "switch"},
		{"Dialog", "android.app.Dialog", false, false, false, "dialog"},
		{"Toolbar", "android.widget.Toolbar", false, false, false, "toolbar"},
		{"Unknown", "com.example.CustomView", false, false, false, "unknown"},
		{"Editable override", "com.example.CustomView", true, false, false, "input"},
		{"Clickable override", "com.example.CustomView", false, true, false, "button"},
		{"Empty class clickable", "", false, true, false, "button"},
		{"Lowercase contains button", "com.example.MyButtonView", false, false, false, "button"},
		{"Lowercase contains text", "com.example.MyTextView", false, false, false, "text"},
		{"Lowercase contains image", "com.example.ImageContainer", false, false, false, "image"},
		{"Lowercase contains list", "com.example.ListContainer", false, false, false, "list"},
		{"Lowercase contains scroll", "com.example.ScrollContainer", false, false, false, "scroll_view"},
		{"Lowercase contains web", "com.example.WebContainer", false, false, false, "web_view"},
		{"Lowercase contains dialog", "com.example.DialogContainer", false, false, false, "dialog"},
		{"Lowercase contains tab", "com.example.TabContainer", false, false, false, "tab"},
		{"Lowercase contains toolbar", "com.example.ToolbarContainer", false, false, false, "toolbar"},
		{"Lowercase contains switch", "com.example.SwitchContainer", false, false, false, "switch"},
		{"Lowercase contains check", "com.example.CheckContainer", false, false, false, "checkbox"},
		{"Lowercase contains radio", "com.example.RadioContainer", false, false, false, "radio"},
		{"Lowercase contains seek", "com.example.SeekContainer", false, false, false, "slider"},
		{"Lowercase contains progress", "com.example.ProgressContainer", false, false, false, "progress"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapClassToRole(tt.className, tt.editable, tt.clickable, tt.checkable)
			if result != tt.expected {
				t.Fatalf("MapClassToRole(%q, %v, %v, %v) = %q, expected %q",
					tt.className, tt.editable, tt.clickable, tt.checkable, result, tt.expected)
			}
		})
	}
}

func TestMapActionName(t *testing.T) {
	tests := []struct {
		raw      string
		expected string
	}{
		{"ACTION_CLICK", "click"},
		{"ACTION_LONG_CLICK", "long_click"},
		{"ACTION_SCROLL_FORWARD", "scroll_forward"},
		{"ACTION_SCROLL_BACKWARD", "scroll_backward"},
		{"ACTION_SET_TEXT", "set_text"},
		{"ACTION_FOCUS", "focus"},
		{"ACTION_CLEAR_FOCUS", "clear_focus"},
		{"ACTION_SELECT", "select"},
		{"UNKNOWN_ACTION", "unknown_action"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			result := MapActionName(tt.raw)
			if result != tt.expected {
				t.Fatalf("MapActionName(%q) = %q, expected %q", tt.raw, result, tt.expected)
			}
		})
	}
}

func TestMapActions(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"nil", nil, nil},
		{"empty", []string{}, nil},
		{"single", []string{"ACTION_CLICK"}, []string{"click"}},
		{"multiple", []string{"ACTION_CLICK", "ACTION_FOCUS"}, []string{"click", "focus"}},
		{"deduplicate", []string{"ACTION_CLICK", "ACTION_CLICK"}, []string{"click"}},
		{"unknown", []string{"UNKNOWN"}, []string{"unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapActions(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("MapActions(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Fatalf("MapActions(%v)[%d] = %q, expected %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestRedactSensitiveNode(t *testing.T) {
	node := &UINode{
		Text:               "secret_password",
		ContentDescription: "password field",
		ResourceID:         "com.example:id/password",
		ClassName:          "android.widget.EditText",
		Password:           true,
	}

	RedactSensitiveNode(node, 4096, 4096, 1024, 512)

	if node.Text != "" {
		t.Fatalf("expected empty text for password node, got %q", node.Text)
	}
	if node.ContentDescription != "password field" {
		t.Fatalf("expected description preserved, got %q", node.ContentDescription)
	}
}

func TestRedactSensitiveNode_Truncate(t *testing.T) {
	longText := make([]rune, 5000)
	for i := range longText {
		longText[i] = 'a'
	}

	node := &UINode{
		Text:               string(longText),
		ContentDescription: string(longText),
		ResourceID:         string(longText),
		ClassName:          string(longText),
		Password:           false,
	}

	RedactSensitiveNode(node, 4096, 4096, 1024, 512)

	if len([]rune(node.Text)) > 4096 {
		t.Fatalf("text not truncated, got %d runes", len([]rune(node.Text)))
	}
	if len([]rune(node.ContentDescription)) > 4096 {
		t.Fatalf("description not truncated, got %d runes", len([]rune(node.ContentDescription)))
	}
	if len([]rune(node.ResourceID)) > 1024 {
		t.Fatalf("resourceId not truncated, got %d runes", len([]rune(node.ResourceID)))
	}
	if len([]rune(node.ClassName)) > 512 {
		t.Fatalf("className not truncated, got %d runes", len([]rune(node.ClassName)))
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxRunes int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hell"},
		{"hello", 0, "hello"},
		{"", 10, ""},
		{"中文测试", 10, "中文测试"},
		{"中文测试", 2, "中文"},
	}

	for _, tt := range tests {
		result := TruncateString(tt.input, tt.maxRunes)
		if result != tt.expected {
			t.Fatalf("TruncateString(%q, %d) = %q, expected %q", tt.input, tt.maxRunes, result, tt.expected)
		}
	}
}
