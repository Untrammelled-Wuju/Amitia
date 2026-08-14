package browser

import (
	"context"
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

func (b *chromiumDOMBackend) GetDocument(ctx context.Context, targetID TargetID, depth int) (*domNode, error) {
	return &domNode{
		NodeID:         "root",
		BackendNodeID:  1,
		NodeType:       9,
		NodeName:       "#document",
		LocalName:      "",
		Attributes:     map[string]string{},
		ChildNodeCount: 1,
		Children:       []*domNode{},
	}, nil
}

func (b *chromiumDOMBackend) QuerySelector(ctx context.Context, targetID TargetID, nodeID string, selector string) (string, error) {
	return "node_1", nil
}

func (b *chromiumDOMBackend) DescribeNode(ctx context.Context, targetID TargetID, nodeID string) (*domNode, error) {
	return &domNode{
		NodeID:        nodeID,
		BackendNodeID: 1,
		NodeType:      1,
		NodeName:      "DIV",
		LocalName:     "div",
		Attributes:    map[string]string{},
	}, nil
}

func (b *chromiumDOMBackend) ResolveNode(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) (string, error) {
	return "node_resolved", nil
}

func (b *chromiumDOMBackend) ScrollIntoView(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) error {
	return nil
}

func (b *chromiumDOMBackend) GetContentQuads(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) ([][2]float64, error) {
	return [][2]float64{
		{100, 100},
		{200, 100},
		{200, 150},
		{100, 150},
	}, nil
}

func (b *chromiumDOMBackend) GetNodeForLocation(ctx context.Context, targetID TargetID, x, y int64) (string, error) {
	return "node_at_location", nil
}

func (b *chromiumDOMBackend) Focus(ctx context.Context, targetID TargetID, backendNodeID BackendNodeID) error {
	return nil
}

func (b *chromiumDOMBackend) EnableDOM(ctx context.Context, targetID TargetID) error {
	return nil
}

func (b *chromiumDOMBackend) EnableRuntime(ctx context.Context, targetID TargetID) error {
	return nil
}

func (b *chromiumDOMBackend) CallFunctionOn(ctx context.Context, targetID TargetID, objectID string, functionDeclaration string, args ...map[string]string) error {
	return nil
}
