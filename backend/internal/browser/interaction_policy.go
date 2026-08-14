package browser

import (
	"time"
)

const (
	DefaultDOMMaxDepth          = 12
	MaxDOMMaxDepth              = 64
	DefaultMaxDOMNodes          = 5000
	DefaultMaxSnapshotBytes     = 1024 * 1024
	DefaultMaxTextBytesPerNode  = 4 * 1024
	DefaultMaxSelectorBytes     = 4 * 1024
	DefaultMaxElementRefsPerTab = 1024
	DefaultMaxInputRunes        = 10000
	DefaultDOMTimeout           = 5 * time.Second
	DefaultMaxDOMTimeout        = 15 * time.Second
	DefaultInteractionTimeout   = 10 * time.Second
	DefaultScrollDeltaRatio     = 0.7
	DefaultMaxClickRetries      = 0
)

type InteractionPolicy struct {
	DOMMaxDepth          int
	MaxDOMMaxDepth       int
	MaxDOMNodes          int
	MaxSnapshotBytes     int
	MaxTextBytesPerNode  int
	MaxSelectorBytes     int
	MaxElementRefsPerTab int
	MaxInputRunes        int
	DOMTimeout           time.Duration
	MaxDOMTimeout        time.Duration
	InteractionTimeout   time.Duration
	ScrollDeltaRatio     float64
	MaxClickRetries      int
}

func NewInteractionPolicy() *InteractionPolicy {
	return &InteractionPolicy{
		DOMMaxDepth:          DefaultDOMMaxDepth,
		MaxDOMMaxDepth:       MaxDOMMaxDepth,
		MaxDOMNodes:          DefaultMaxDOMNodes,
		MaxSnapshotBytes:     DefaultMaxSnapshotBytes,
		MaxTextBytesPerNode:  DefaultMaxTextBytesPerNode,
		MaxSelectorBytes:     DefaultMaxSelectorBytes,
		MaxElementRefsPerTab: DefaultMaxElementRefsPerTab,
		MaxInputRunes:        DefaultMaxInputRunes,
		DOMTimeout:           DefaultDOMTimeout,
		MaxDOMTimeout:        DefaultMaxDOMTimeout,
		InteractionTimeout:   DefaultInteractionTimeout,
		ScrollDeltaRatio:     DefaultScrollDeltaRatio,
		MaxClickRetries:      DefaultMaxClickRetries,
	}
}

func (p *InteractionPolicy) ResolveDOMTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return p.DOMTimeout
	}
	d := time.Duration(timeoutMS) * time.Millisecond
	if d > p.MaxDOMTimeout {
		return p.MaxDOMTimeout
	}
	return d
}

func (p *InteractionPolicy) ResolveInteractionTimeout(timeoutMS int) time.Duration {
	if timeoutMS <= 0 {
		return p.InteractionTimeout
	}
	d := time.Duration(timeoutMS) * time.Millisecond
	if d > p.MaxDOMTimeout {
		return p.MaxDOMTimeout
	}
	return d
}

func (p *InteractionPolicy) NormalizeMaxDepth(maxDepth int) int {
	if maxDepth <= 0 {
		return p.DOMMaxDepth
	}
	if maxDepth > p.MaxDOMMaxDepth {
		return p.MaxDOMMaxDepth
	}
	return maxDepth
}

func (p *InteractionPolicy) ValidateSelector(selector string) *BrowserError {
	if selector == "" {
		return &BrowserError{
			Code:    ErrCodeInvalidSelector,
			Message: "selector is required",
		}
	}
	if len(selector) > p.MaxSelectorBytes {
		return &BrowserError{
			Code:    ErrCodeInvalidSelector,
			Message: "selector exceeds maximum length",
		}
	}
	return nil
}

func (p *InteractionPolicy) ValidateInputText(text string) *BrowserError {
	if len([]rune(text)) > p.MaxInputRunes {
		return &BrowserError{
			Code:    ErrCodeInvalidRequest,
			Message: "input text exceeds maximum length",
		}
	}
	return nil
}

func (p *InteractionPolicy) IsScrollDirectionAllowed(direction string) bool {
	switch direction {
	case "up", "down", "left", "right":
		return true
	default:
		return false
	}
}
