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

type cdpDOMNode struct {
	NodeID         int64        `json:"nodeId"`
	BackendNodeID  int64        `json:"backendNodeId"`
	NodeType       int          `json:"nodeType"`
	NodeName       string       `json:"nodeName"`
	LocalName      string       `json:"localName"`
	NodeValue      string       `json:"nodeValue"`
	ChildNodeCount int          `json:"childNodeCount"`
	FrameID        string       `json:"frameId"`
	DocumentURL    string       `json:"documentURL"`
	Attributes     []string     `json:"attributes"`
	Children       []cdpDOMNode `json:"children"`
}

func convertCDPDOMNode(source cdpDOMNode) *domNode {
	attrs := map[string]string{}
	for i := 0; i < len(source.Attributes)-1; i += 2 {
		attrs[source.Attributes[i]] = source.Attributes[i+1]
	}
	node := &domNode{
		NodeID:         fmt.Sprintf("%d", source.NodeID),
		BackendNodeID:  BackendNodeID(source.BackendNodeID),
		NodeType:       source.NodeType,
		NodeName:       source.NodeName,
		LocalName:      source.LocalName,
		NodeValue:      source.NodeValue,
		ChildNodeCount: source.ChildNodeCount,
		Attributes:     attrs,
		FrameID:        source.FrameID,
		DocumentURL:    source.DocumentURL,
	}
	node.Children = make([]*domNode, 0, len(source.Children))
	for _, child := range source.Children {
		node.Children = append(node.Children, convertCDPDOMNode(child))
	}
	return node
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

func (b *chromiumDOMBackend) ensureSession(ctx context.Context, targetID TargetID) (string, error) {
	if b.engine == nil {
		return "", fmt.Errorf("browser engine is unavailable")
	}
	client := b.GetClient()
	if client == nil {
		return "", fmt.Errorf("CDP client not available")
	}
	return b.engine.Pages().(*chromiumPageController).ensureSessionWithError(ctx, client, targetID)
}

func (b *chromiumDOMBackend) GetDocument(ctx context.Context, targetID TargetID, depth int) (*domNode, error) {
	client := b.GetClient()
	if client == nil {
		return nil, fmt.Errorf("CDP client not available")
	}
	sessionID, sessionErr := b.ensureSession(ctx, targetID)
	if sessionErr != nil {
		return nil, fmt.Errorf("failed to ensure session for target %s: %w", targetID, sessionErr)
	}

	if err := client.Call(ctx, "DOM.enable", sessionID, nil, nil); err != nil {
		return nil, fmt.Errorf("DOM.enable failed: %w", err)
	}

	params := map[string]interface{}{
		"depth":  depth,
		"pierce": false,
	}
	var result struct {
		Root cdpDOMNode `json:"root"`
	}
	if err := client.Call(ctx, "DOM.getDocument", sessionID, params, &result); err != nil {
		return nil, fmt.Errorf("DOM.getDocument failed: %w", err)
	}

	return convertCDPDOMNode(result.Root), nil
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
		Node cdpDOMNode `json:"node"`
	}
	if err := client.Call(ctx, "DOM.describeNode", sessionID, params, &result); err != nil {
		return nil, fmt.Errorf("DOM.describeNode failed: %w", err)
	}

	return convertCDPDOMNode(result.Node), nil
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
	sessionID, sessionErr := b.ensureSession(ctx, targetID)
	if sessionErr != nil {
		return fmt.Errorf("failed to ensure session for target %s: %w", targetID, sessionErr)
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
