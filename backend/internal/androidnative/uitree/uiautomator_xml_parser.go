package uitree

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// UiAutomatorXmlParser converts the XML emitted by `uiautomator dump` into the
// same raw node/window shape used by the Accessibility source. This keeps ADB
// and Root fallback transparent to the rest of the UI Tree stack.
type UiAutomatorXmlParser struct{}

type uiAutomatorHierarchy struct {
	Rotation string            `xml:"rotation,attr"`
	Nodes    []uiAutomatorNode `xml:"node"`
}

type uiAutomatorNode struct {
	Index       string            `xml:"index,attr"`
	Text        string            `xml:"text,attr"`
	ResourceID  string            `xml:"resource-id,attr"`
	Class       string            `xml:"class,attr"`
	Package     string            `xml:"package,attr"`
	Description string            `xml:"content-desc,attr"`
	Checkable   string            `xml:"checkable,attr"`
	Checked     string            `xml:"checked,attr"`
	Clickable   string            `xml:"clickable,attr"`
	Enabled     string            `xml:"enabled,attr"`
	Focusable   string            `xml:"focusable,attr"`
	Focused     string            `xml:"focused,attr"`
	Scrollable  string            `xml:"scrollable,attr"`
	LongClick   string            `xml:"long-clickable,attr"`
	Password    string            `xml:"password,attr"`
	Selected    string            `xml:"selected,attr"`
	Bounds      string            `xml:"bounds,attr"`
	Children    []uiAutomatorNode `xml:"node"`
}

func (UiAutomatorXmlParser) Parse(data string, source SourceType) ([]map[string]any, []map[string]any, error) {
	data = strings.TrimSpace(data)
	if data == "" {
		return nil, nil, &Error{Code: UI_TREE_INVALID_RESPONSE, Message: "uiautomator XML is empty"}
	}

	var hierarchy uiAutomatorHierarchy
	if err := xml.Unmarshal([]byte(data), &hierarchy); err != nil {
		return nil, nil, &Error{Code: UI_TREE_INVALID_RESPONSE, Message: "parse uiautomator XML: " + err.Error()}
	}

	windowID := "uia_" + string(source) + "_window_0"
	nodes := make([]map[string]any, 0, 64)
	var rootID string
	var rootBounds Rect
	var rootPackage string

	for i := range hierarchy.Nodes {
		id, bounds, pkg := appendUiAutomatorNode(&nodes, hierarchy.Nodes[i], source, windowID, "", fmt.Sprintf("%d", i), 0)
		if rootID == "" {
			rootID, rootBounds, rootPackage = id, bounds, pkg
		}
	}

	windows := []map[string]any{{
		"windowId":    windowID,
		"type":        string(WindowTypeApplication),
		"packageName": rootPackage,
		"active":      true,
		"focused":     true,
		"displayId":   float64(0),
		"rootNodeId":  rootID,
		"left":        float64(rootBounds.Left),
		"top":         float64(rootBounds.Top),
		"right":       float64(rootBounds.Right),
		"bottom":      float64(rootBounds.Bottom),
	}}
	return windows, nodes, nil
}

func appendUiAutomatorNode(
	out *[]map[string]any,
	n uiAutomatorNode,
	source SourceType,
	windowID string,
	parentID string,
	path string,
	depth int,
) (string, Rect, string) {
	bounds := parseUiAutomatorBounds(n.Bounds)
	nodeID := uiAutomatorStableID(source, path, n.ResourceID, n.Class, bounds)
	actions := make([]any, 0, 5)
	if parseUIBool(n.Clickable) {
		actions = append(actions, "ACTION_CLICK")
	}
	if parseUIBool(n.LongClick) {
		actions = append(actions, "ACTION_LONG_CLICK")
	}
	if parseUIBool(n.Scrollable) {
		actions = append(actions, "ACTION_SCROLL_FORWARD", "ACTION_SCROLL_BACKWARD")
	}
	editable := strings.Contains(strings.ToLower(n.Class), "edittext")
	if editable {
		actions = append(actions, "ACTION_SET_TEXT")
	}

	raw := map[string]any{
		"nodeId":             nodeID,
		"parentId":           parentID,
		"windowId":           windowID,
		"className":          n.Class,
		"packageName":        n.Package,
		"text":               n.Text,
		"contentDescription": n.Description,
		"resourceId":         n.ResourceID,
		"sourceRef":          path,
		"left":               float64(bounds.Left),
		"top":                float64(bounds.Top),
		"right":              float64(bounds.Right),
		"bottom":             float64(bounds.Bottom),
		"visibleToUser":      true,
		"enabled":            parseUIBool(n.Enabled),
		"focusable":          parseUIBool(n.Focusable),
		"focused":            parseUIBool(n.Focused),
		"selected":           parseUIBool(n.Selected),
		"checked":            parseUIBool(n.Checked),
		"checkable":          parseUIBool(n.Checkable),
		"clickable":          parseUIBool(n.Clickable),
		"longClickable":      parseUIBool(n.LongClick),
		"scrollable":         parseUIBool(n.Scrollable),
		"editable":           editable,
		"password":           parseUIBool(n.Password),
		"depth":              float64(depth),
		"actions":            actions,
	}
	*out = append(*out, raw)

	for i := range n.Children {
		childPath := path + "/" + strconv.Itoa(i)
		appendUiAutomatorNode(out, n.Children[i], source, windowID, nodeID, childPath, depth+1)
	}
	return nodeID, bounds, n.Package
}

func parseUIBool(v string) bool {
	return strings.EqualFold(strings.TrimSpace(v), "true")
}

func parseUiAutomatorBounds(v string) Rect {
	var r Rect
	_, _ = fmt.Sscanf(v, "[%d,%d][%d,%d]", &r.Left, &r.Top, &r.Right, &r.Bottom)
	return r
}

func uiAutomatorStableID(source SourceType, path, resourceID, className string, bounds Rect) string {
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "%s|%s|%s|%s|%d,%d,%d,%d", source, path, resourceID, className, bounds.Left, bounds.Top, bounds.Right, bounds.Bottom)
	sum := h.Sum(nil)
	return "uia_" + hex.EncodeToString(sum[:8])
}
