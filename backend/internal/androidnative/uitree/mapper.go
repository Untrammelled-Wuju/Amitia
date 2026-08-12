package uitree

import "strings"

var classToRoleMap = map[string]string{
	"android.widget.Button":              "button",
	"android.widget.ImageButton":         "image_button",
	"android.widget.CheckBox":            "checkbox",
	"android.widget.RadioButton":         "radio",
	"android.widget.Switch":              "switch",
	"android.widget.EditText":            "input",
	"android.widget.TextView":            "text",
	"android.widget.ImageView":           "image",
	"android.widget.SeekBar":             "slider",
	"android.widget.ProgressBar":         "progress",
	"android.widget.Spinner":             "menu",
	"android.widget.ListView":            "list",
	"android.widget.GridView":            "list",
	"androidx.recyclerview.widget.RecyclerView": "list",
	"android.widget.ScrollView":          "scroll_view",
	"android.webkit.WebView":             "web_view",
	"androidx.viewpager.widget.ViewPager": "scroll_view",
	"android.app.Dialog":                 "dialog",
	"android.widget.TabWidget":           "tab",
	"android.widget.Toolbar":             "toolbar",
	"androidx.appcompat.widget.Toolbar":  "toolbar",
	"android.widget.PopupMenu":           "menu",
	"android.view.Menu":                  "menu",
}

var actionNameMap = map[string]string{
	"ACTION_CLICK":           "click",
	"ACTION_LONG_CLICK":      "long_click",
	"ACTION_SCROLL_FORWARD":  "scroll_forward",
	"ACTION_SCROLL_BACKWARD": "scroll_backward",
	"ACTION_SET_TEXT":        "set_text",
	"ACTION_FOCUS":           "focus",
	"ACTION_CLEAR_FOCUS":     "clear_focus",
	"ACTION_SELECT":          "select",
}

func MapClassToRole(className string, editable bool, clickable bool, checkable bool) string {
	if role, ok := classToRoleMap[className]; ok {
		return role
	}

	if className != "" {
		lower := strings.ToLower(className)
		if strings.Contains(lower, "button") {
			return "button"
		}
		if strings.Contains(lower, "edittext") || strings.Contains(lower, "editor") {
			return "input"
		}
		if strings.Contains(lower, "textview") || strings.Contains(lower, "text") {
			return "text"
		}
		if strings.Contains(lower, "imageview") || strings.Contains(lower, "image") {
			return "image"
		}
		if strings.Contains(lower, "list") || strings.Contains(lower, "recycler") {
			return "list"
		}
		if strings.Contains(lower, "scroll") {
			return "scroll_view"
		}
		if strings.Contains(lower, "web") {
			return "web_view"
		}
		if strings.Contains(lower, "dialog") {
			return "dialog"
		}
		if strings.Contains(lower, "tab") {
			return "tab"
		}
		if strings.Contains(lower, "toolbar") {
			return "toolbar"
		}
		if strings.Contains(lower, "switch") || strings.Contains(lower, "toggle") {
			return "switch"
		}
		if strings.Contains(lower, "check") {
			return "checkbox"
		}
		if strings.Contains(lower, "radio") {
			return "radio"
		}
		if strings.Contains(lower, "seek") || strings.Contains(lower, "slider") {
			return "slider"
		}
		if strings.Contains(lower, "progress") {
			return "progress"
		}
	}

	if editable {
		return "input"
	}
	if clickable {
		return "button"
	}

	return "unknown"
}

func MapActionName(rawAction string) string {
	if name, ok := actionNameMap[rawAction]; ok {
		return name
	}
	return strings.ToLower(rawAction)
}

func MapActions(rawActions []string) []string {
	if len(rawActions) == 0 {
		return nil
	}
	result := make([]string, 0, len(rawActions))
	seen := make(map[string]bool)
	for _, raw := range rawActions {
		name := MapActionName(raw)
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func RedactSensitiveNode(node *UINode, maxTextRunes, maxDescRunes, maxResourceIDRunes, maxClassNameRunes int) {
	node.Text = TruncateString(node.Text, maxTextRunes)
	node.ContentDescription = TruncateString(node.ContentDescription, maxDescRunes)
	node.ResourceID = TruncateString(node.ResourceID, maxResourceIDRunes)
	node.ClassName = TruncateString(node.ClassName, maxClassNameRunes)

	if node.Password {
		node.Text = ""
	}
}
