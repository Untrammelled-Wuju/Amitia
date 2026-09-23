package webresearch

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/browser"
)

type cleanupContextKey string

func TestBrowserCleanupContextDetachesCancellationButKeepsDeadlineAndValues(t *testing.T) {
	parent := context.WithValue(context.Background(), cleanupContextKey("trace"), "trace-1")
	parent, cancelParent := context.WithCancel(parent)
	cancelParent()
	ctx, cancel := browserCleanupContext(parent)
	defer cancel()
	if err := ctx.Err(); err != nil {
		t.Fatalf("cleanup context inherited cancellation: %v", err)
	}
	if got := ctx.Value(cleanupContextKey("trace")); got != "trace-1" {
		t.Fatalf("cleanup context lost request value: %v", got)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("cleanup context must have a deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > browserCleanupTimeout+time.Second {
		t.Fatalf("unexpected cleanup deadline: %v", remaining)
	}
}

type dynamicBrowserProvider struct {
	browser.BrowserProvider
	observer   browser.BrowserObserver
	interactor browser.BrowserInteractor
	caps       browser.BrowserCapabilities
}

func (p *dynamicBrowserProvider) BrowserCapabilities() browser.BrowserCapabilities { return p.caps }
func (p *dynamicBrowserProvider) Observe() browser.BrowserObserver                 { return p.observer }
func (p *dynamicBrowserProvider) Interact() browser.BrowserInteractor              { return p.interactor }

type dynamicObserver struct {
	browser.BrowserObserver
	contents []string
	calls    int
}

func (o *dynamicObserver) GetDOMSnapshot(_ context.Context, sessionID browser.BrowserSessionID, tabID browser.BrowserTabID, maxDepth int) (*browser.BrowserDOMSnapshot, *browser.BrowserError) {
	if len(o.contents) == 0 {
		return &browser.BrowserDOMSnapshot{SessionID: sessionID, TabID: tabID, Content: ""}, nil
	}
	idx := o.calls
	if idx >= len(o.contents) {
		idx = len(o.contents) - 1
	}
	o.calls++
	return &browser.BrowserDOMSnapshot{SessionID: sessionID, TabID: tabID, Content: o.contents[idx], MaxDepth: maxDepth}, nil
}

type dynamicInteractor struct {
	browser.BrowserInteractor
	calls int
	err   *browser.BrowserError
}

func (i *dynamicInteractor) Scroll(_ context.Context, _ browser.BrowserSessionID, _ browser.BrowserTabID, direction string) (*browser.BrowserInteractionResult, *browser.BrowserError) {
	i.calls++
	if i.err != nil {
		return nil, i.err
	}
	return &browser.BrowserInteractionResult{Success: true, Action: "scroll", Strategy: direction}, nil
}

func newDynamicReader(contents []string, maxScrolls int) (*AmitiaBrowserReader, *dynamicObserver, *dynamicInteractor) {
	base := browser.NewDisabledProvider()
	observer := &dynamicObserver{BrowserObserver: base.Observe(), contents: contents}
	interactor := &dynamicInteractor{BrowserInteractor: base.Interact()}
	provider := &dynamicBrowserProvider{
		BrowserProvider: base,
		observer:        observer,
		interactor:      interactor,
		caps:            browser.BrowserCapabilities{SupportsInteraction: true, SupportsDOM: true},
	}
	reader := NewAmitiaBrowserReader(provider).WithMaxScrolls(maxScrolls)
	reader.scrollSettleDelay = 0
	return reader, observer, interactor
}

func TestExpandDynamicContentUsesBoundedScrolls(t *testing.T) {
	reader, observer, interactor := newDynamicReader([]string{"expanded once", "expanded twice"}, 2)
	initial := &browser.BrowserDOMSnapshot{Content: "initial"}
	got, err := reader.expandDynamicContent(context.Background(), "session", "tab", initial)
	if err != nil {
		t.Fatalf("expand dynamic content: %v", err)
	}
	if got.Content != "expanded twice" {
		t.Fatalf("unexpected final content: %q", got.Content)
	}
	if interactor.calls != 2 || observer.calls != 2 {
		t.Fatalf("expected exactly 2 bounded scroll/read cycles, scroll=%d snapshot=%d", interactor.calls, observer.calls)
	}
}

func TestExpandDynamicContentStopsWhenSnapshotDoesNotChange(t *testing.T) {
	reader, observer, interactor := newDynamicReader([]string{"same", "should not be read"}, 4)
	initial := &browser.BrowserDOMSnapshot{Content: "same"}
	got, err := reader.expandDynamicContent(context.Background(), "session", "tab", initial)
	if err != nil {
		t.Fatalf("expand dynamic content: %v", err)
	}
	if got.Content != "same" {
		t.Fatalf("unexpected final content: %q", got.Content)
	}
	if interactor.calls != 1 || observer.calls != 1 {
		t.Fatalf("unchanged DOM should stop after one cycle, scroll=%d snapshot=%d", interactor.calls, observer.calls)
	}
}

func TestExpandDynamicContentKeepsCapturedDOMWhenScrollFails(t *testing.T) {
	reader, observer, interactor := newDynamicReader([]string{"unused"}, 2)
	interactor.err = &browser.BrowserError{Code: browser.ErrCodeUnsupportedAction, Message: "scroll unsupported"}
	initial := &browser.BrowserDOMSnapshot{Content: "captured"}
	got, err := reader.expandDynamicContent(context.Background(), "session", "tab", initial)
	if err != nil {
		t.Fatalf("optional scroll failure should not fail browser read: %v", err)
	}
	if got != initial {
		t.Fatal("expected previously captured DOM to be preserved")
	}
	if observer.calls != 0 || interactor.calls != 1 {
		t.Fatalf("unexpected calls after failed scroll: scroll=%d snapshot=%d", interactor.calls, observer.calls)
	}
}
