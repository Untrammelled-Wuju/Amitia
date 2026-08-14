package browser

import (
	"context"
	"fmt"
)

type DOMBackend interface {
	GetDocument(ctx context.Context, targetID TargetID, depth int) (*domNode, error)
	QuerySelector(ctx context.Context, targetID TargetID, nodeID string, selector string) (string, error)
	DescribeNode(ctx context.Context, targetID TargetID, nodeID string) (*domNode, error)
	ResolveNode(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) (string, error)
	ScrollIntoView(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) error
	GetContentQuads(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) ([][2]float64, error)
	GetNodeForLocation(ctx context.Context, targetID TargetID, x, y int64) (string, error)
	Focus(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) error
	EnableDOM(ctx context.Context, targetID TargetID) error
	EnableRuntime(ctx context.Context, targetID TargetID) error
	CallFunctionOn(ctx context.Context, targetID TargetID, objectID string, functionDeclaration string, args ...map[string]string) error
	SetFileInputFiles(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID, paths []string) error
	GetClient() *cdpClient
	GetSession(targetID TargetID) string
}

type domNode struct {
	NodeID         string
	BackendNodeID  BackendNodeID
	NodeType       int
	NodeName       string
	NodeValue      string
	LocalName      string
	Attributes     map[string]string
	ChildNodeCount int
	Children       []*domNode
	FrameID        string
	DocumentURL    string
}

type chromiumDOMBackend struct {
	engine *chromiumEngine
}

func NewChromiumDOMBackend(engine BrowserEngine) DOMBackend {
	if e, ok := engine.(*chromiumEngine); ok {
		return &chromiumDOMBackend{engine: e}
	}
	return &chromiumDOMBackend{}
}

func (b *chromiumDOMBackend) GetClient() *cdpClient {
	if b.engine == nil {
		return nil
	}
	return b.engine.cdpClient()
}

func (b *chromiumDOMBackend) SetFileInputFiles(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID, paths []string) error {
	client := b.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.GetSession(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}

	params := map[string]interface{}{
		"backendNodeId": int64(backendNodeID),
		"files":         paths,
	}
	if err := client.Call(ctx, "DOM.setFileInputFiles", sessionID, params, nil); err != nil {
		return fmt.Errorf("DOM.setFileInputFiles failed: %w", err)
	}
	return nil
}

func (b *chromiumDOMBackend) GetSession(targetID TargetID) string {
	return b.sessionFor(targetID)
}

func (b *chromiumDOMBackend) sessionFor(targetID TargetID) string {
	if b.engine == nil {
		return ""
	}
	return b.engine.Pages().(*chromiumPageController).getSession(targetID)
}

func (b *chromiumDOMBackend) ensureSession(ctx context.Context, targetID TargetID) string {
	if b.engine == nil {
		return ""
	}
	client := b.GetClient()
	if client == nil {
		return ""
	}
	return b.engine.Pages().(*chromiumPageController).ensureSession(ctx, client, targetID)
}

func (b *chromiumDOMBackend) GetDocument(ctx context.Context, targetID TargetID, depth int) (*domNode, error) {
	client := b.GetClient()
	if client == nil {
		return nil, fmt.Errorf("CDP client not available")
	}
	sessionID := b.ensureSession(ctx, targetID)
	if sessionID == "" {
		return nil, fmt.Errorf("failed to ensure session for target %s", targetID)
	}

	if err := client.Call(ctx, "DOM.enable", sessionID, nil, nil); err != nil {
		return nil, fmt.Errorf("DOM.enable failed: %w", err)
	}

	params := map[string]interface{}{
		"depth":  depth,
		"pierce": false,
	}
	var result struct {
		Root struct {
			NodeID         int64    `json:"nodeId"`
			BackendNodeID  int64    `json:"backendNodeId"`
			NodeType       int      `json:"nodeType"`
			NodeName       string   `json:"nodeName"`
			LocalName      string   `json:"localName"`
			NodeValue      string   `json:"nodeValue"`
			ChildNodeCount int      `json:"childNodeCount"`
			FrameID        string   `json:"frameId"`
			DocumentURL    string   `json:"documentURL"`
			Attributes     []string `json:"attributes"`
			Children       []struct {
				NodeID         int64    `json:"nodeId"`
				BackendNodeID  int64    `json:"backendNodeId"`
				NodeType       int      `json:"nodeType"`
				NodeName       string   `json:"nodeName"`
				LocalName      string   `json:"localName"`
				NodeValue      string   `json:"nodeValue"`
				ChildNodeCount int      `json:"childNodeCount"`
				FrameID        string   `json:"frameId"`
				DocumentURL    string   `json:"documentURL"`
				Attributes     []string `json:"attributes"`
			} `json:"children"`
		} `json:"root"`
	}
	if err := client.Call(ctx, "DOM.getDocument", sessionID, params, &result); err != nil {
		return nil, fmt.Errorf("DOM.getDocument failed: %w", err)
	}

	root := result.Root
	attrs := map[string]string{}
	for i := 0; i < len(root.Attributes)-1; i += 2 {
		attrs[root.Attributes[i]] = root.Attributes[i+1]
	}
	node := &domNode{
		NodeID:         fmt.Sprintf("%d", root.NodeID),
		BackendNodeID:  BackendNodeID(root.BackendNodeID),
		NodeType:       root.NodeType,
		NodeName:       root.NodeName,
		LocalName:      root.LocalName,
		NodeValue:      root.NodeValue,
		ChildNodeCount: root.ChildNodeCount,
		Attributes:     attrs,
		FrameID:        root.FrameID,
		DocumentURL:    root.DocumentURL,
	}
	for _, child := range root.Children {
		childAttrs := map[string]string{}
		for i := 0; i < len(child.Attributes)-1; i += 2 {
			childAttrs[child.Attributes[i]] = child.Attributes[i+1]
		}
		node.Children = append(node.Children, &domNode{
			NodeID:         fmt.Sprintf("%d", child.NodeID),
			BackendNodeID:  BackendNodeID(child.BackendNodeID),
			NodeType:       child.NodeType,
			NodeName:       child.NodeName,
			LocalName:      child.LocalName,
			NodeValue:      child.NodeValue,
			ChildNodeCount: child.ChildNodeCount,
			Attributes:     childAttrs,
			FrameID:        child.FrameID,
			DocumentURL:    child.DocumentURL,
		})
	}
	return node, nil
}

func (b *chromiumDOMBackend) QuerySelector(ctx context.Context, targetID TargetID, nodeID string, selector string) (string, error) {
	client := b.GetClient()
	if client == nil {
		return "", fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return "", fmt.Errorf("no session for target %s", targetID)
	}

	nodeIDInt := int64(0)
	fmt.Sscanf(nodeID, "%d", &nodeIDInt)

	params := map[string]interface{}{
		"nodeId":   nodeIDInt,
		"selector": selector,
	}
	var result struct {
		NodeID int64 `json:"nodeId"`
	}
	if err := client.Call(ctx, "DOM.querySelector", sessionID, params, &result); err != nil {
		return "", fmt.Errorf("DOM.querySelector failed: %w", err)
	}
	if result.NodeID == 0 {
		return "", fmt.Errorf("selector not found: %s", selector)
	}
	return fmt.Sprintf("%d", result.NodeID), nil
}

func (b *chromiumDOMBackend) DescribeNode(ctx context.Context, targetID TargetID, nodeID string) (*domNode, error) {
	client := b.GetClient()
	if client == nil {
		return nil, fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return nil, fmt.Errorf("no session for target %s", targetID)
	}

	nodeIDInt := int64(0)
	fmt.Sscanf(nodeID, "%d", &nodeIDInt)

	params := map[string]interface{}{
		"nodeId": nodeIDInt,
	}
	var result struct {
		Node struct {
			NodeID         int64    `json:"nodeId"`
			BackendNodeID  int64    `json:"backendNodeId"`
			NodeType       int      `json:"nodeType"`
			NodeName       string   `json:"nodeName"`
			LocalName      string   `json:"localName"`
			NodeValue      string   `json:"nodeValue"`
			ChildNodeCount int      `json:"childNodeCount"`
			FrameID        string   `json:"frameId"`
			DocumentURL    string   `json:"documentURL"`
			Attributes     []string `json:"attributes"`
		} `json:"node"`
	}
	if err := client.Call(ctx, "DOM.describeNode", sessionID, params, &result); err != nil {
		return nil, fmt.Errorf("DOM.describeNode failed: %w", err)
	}

	attrs := map[string]string{}
	for i := 0; i < len(result.Node.Attributes)-1; i += 2 {
		attrs[result.Node.Attributes[i]] = result.Node.Attributes[i+1]
	}
	return &domNode{
		NodeID:         fmt.Sprintf("%d", result.Node.NodeID),
		BackendNodeID:  BackendNodeID(result.Node.BackendNodeID),
		NodeType:       result.Node.NodeType,
		NodeName:       result.Node.NodeName,
		LocalName:      result.Node.LocalName,
		NodeValue:      result.Node.NodeValue,
		ChildNodeCount: result.Node.ChildNodeCount,
		Attributes:     attrs,
		FrameID:        result.Node.FrameID,
		DocumentURL:    result.Node.DocumentURL,
	}, nil
}

func (b *chromiumDOMBackend) ResolveNode(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) (string, error) {
	client := b.GetClient()
	if client == nil {
		return "", fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return "", fmt.Errorf("no session for target %s", targetID)
	}

	if err := client.Call(ctx, "Runtime.enable", sessionID, nil, nil); err != nil {
		return "", fmt.Errorf("Runtime.enable failed: %w", err)
	}

	params := map[string]interface{}{
		"backendNodeId": int64(backendNodeID),
	}
	var result struct {
		Object struct {
			ObjectID string `json:"objectId"`
			Type     string `json:"type"`
		} `json:"object"`
	}
	if err := client.Call(ctx, "DOM.resolveNode", sessionID, params, &result); err != nil {
		return "", fmt.Errorf("DOM.resolveNode failed: %w", err)
	}
	if result.Object.ObjectID == "" {
		return "", fmt.Errorf("failed to resolve node: %d", backendNodeID)
	}
	return result.Object.ObjectID, nil
}

func (b *chromiumDOMBackend) ScrollIntoView(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) error {
	client := b.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}

	params := map[string]interface{}{
		"backendNodeId": int64(backendNodeID),
	}
	if err := client.Call(ctx, "DOM.scrollIntoViewIfNeeded", sessionID, params, nil); err != nil {
		return fmt.Errorf("DOM.scrollIntoViewIfNeeded failed: %w", err)
	}
	return nil
}

func (b *chromiumDOMBackend) GetContentQuads(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) ([][2]float64, error) {
	client := b.GetClient()
	if client == nil {
		return nil, fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return nil, fmt.Errorf("no session for target %s", targetID)
	}

	params := map[string]interface{}{
		"backendNodeId": int64(backendNodeID),
	}
	var result struct {
		Quads []struct {
			X float64 `json:"0,number"`
			Y float64 `json:"1,number"`
		} `json:"quads"`
	}
	if err := client.Call(ctx, "DOM.getContentQuads", sessionID, params, &result); err != nil {
		return nil, fmt.Errorf("DOM.getContentQuads failed: %w", err)
	}

	quads := make([][2]float64, 0, len(result.Quads))
	for _, q := range result.Quads {
		quads = append(quads, [2]float64{q.X, q.Y})
	}
	return quads, nil
}

func (b *chromiumDOMBackend) GetNodeForLocation(ctx context.Context, targetID TargetID, x, y int64) (string, error) {
	client := b.GetClient()
	if client == nil {
		return "", fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return "", fmt.Errorf("no session for target %s", targetID)
	}

	params := map[string]interface{}{
		"x": x,
		"y": y,
	}
	var result struct {
		BackendNodeID int64 `json:"backendNodeId"`
		FrameID       int64 `json:"frameId"`
	}
	if err := client.Call(ctx, "DOM.getNodeForLocation", sessionID, params, &result); err != nil {
		return "", fmt.Errorf("DOM.getNodeForLocation failed: %w", err)
	}
	if result.BackendNodeID == 0 {
		return "", fmt.Errorf("no node at location (%d, %d)", x, y)
	}
	return fmt.Sprintf("%d", result.BackendNodeID), nil
}

func (b *chromiumDOMBackend) Focus(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) error {
	client := b.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}

	params := map[string]interface{}{
		"backendNodeId": int64(backendNodeID),
	}
	if err := client.Call(ctx, "DOM.focus", sessionID, params, nil); err != nil {
		return fmt.Errorf("DOM.focus failed: %w", err)
	}
	return nil
}

func (b *chromiumDOMBackend) EnableDOM(ctx context.Context, targetID TargetID) error {
	client := b.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.ensureSession(ctx, targetID)
	if sessionID == "" {
		return fmt.Errorf("failed to ensure session for target %s", targetID)
	}
	if err := client.Call(ctx, "DOM.enable", sessionID, nil, nil); err != nil {
		return fmt.Errorf("DOM.enable failed: %w", err)
	}
	return nil
}

func (b *chromiumDOMBackend) EnableRuntime(ctx context.Context, targetID TargetID) error {
	client := b.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}
	if err := client.Call(ctx, "Runtime.enable", sessionID, nil, nil); err != nil {
		return fmt.Errorf("Runtime.enable failed: %w", err)
	}
	return nil
}

func (b *chromiumDOMBackend) CallFunctionOn(ctx context.Context, targetID TargetID, objectID string, functionDeclaration string, args ...map[string]string) error {
	client := b.GetClient()
	if client == nil {
		return fmt.Errorf("CDP client not available")
	}
	sessionID := b.sessionFor(targetID)
	if sessionID == "" {
		return fmt.Errorf("no session for target %s", targetID)
	}

	cdpArgs := []map[string]interface{}{}
	for _, arg := range args {
		for k, v := range arg {
			cdpArgs = append(cdpArgs, map[string]interface{}{
				"value": v,
				"name":  k,
			})
		}
	}

	params := map[string]interface{}{
		"objectId":            objectID,
		"functionDeclaration": functionDeclaration,
		"arguments":           cdpArgs,
		"returnByValue":       true,
	}
	var result struct {
		Result struct {
			Type  string `json:"type"`
			Value any    `json:"value"`
		} `json:"result"`
		ExceptionDetails struct {
			Text       string `json:"text"`
			LineNumber int    `json:"lineNumber"`
		} `json:"exceptionDetails"`
	}
	if err := client.Call(ctx, "Runtime.callFunctionOn", sessionID, params, &result); err != nil {
		return fmt.Errorf("Runtime.callFunctionOn failed: %w", err)
	}
	if result.ExceptionDetails.Text != "" {
		return fmt.Errorf("JS exception: %s", result.ExceptionDetails.Text)
	}
	return nil
}
