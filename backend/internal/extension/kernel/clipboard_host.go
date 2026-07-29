package kernel

import (
	"context"
	"errors"
)

var ErrClipboardHostUnavailable = errors.New("clipboard_host: host unavailable")

type ClipboardHost interface {
	WriteText(ctx context.Context, text string) error
	ReadText(ctx context.Context) (string, error)
}

type DefaultClipboardHost struct{}

func NewDefaultClipboardHost() *DefaultClipboardHost {
	return &DefaultClipboardHost{}
}

func (h *DefaultClipboardHost) WriteText(ctx context.Context, text string) error {
	return ErrClipboardHostUnavailable
}

func (h *DefaultClipboardHost) ReadText(ctx context.Context) (string, error) {
	return "", ErrClipboardHostUnavailable
}
