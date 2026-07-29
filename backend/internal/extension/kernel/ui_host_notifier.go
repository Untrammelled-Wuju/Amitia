package kernel

import (
	"context"
	"errors"
)

var (
	ErrUIHostUnavailable         = errors.New("ui_host: notification host unavailable")
	ErrDialogHostUnavailable     = errors.New("ui_host: dialog host unavailable")
	ErrNavigationHostUnavailable = errors.New("ui_host: navigation host unavailable")
)

type NotificationHost interface {
	Notify(ctx context.Context, extensionID string, title string, body string, severity string) error
}

type DialogHost interface {
	Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error)
}

type NavigationHost interface {
	Navigate(ctx context.Context, extensionID string, target string) error
}

type UIHostNotifier interface {
	NotificationHost
	DialogHost
	NavigationHost
}

type DefaultUIHostNotifier struct{}

func NewDefaultUIHostNotifier() *DefaultUIHostNotifier {
	return &DefaultUIHostNotifier{}
}

func (n *DefaultUIHostNotifier) Notify(ctx context.Context, extensionID string, title string, body string, severity string) error {
	return ErrUIHostUnavailable
}

func (n *DefaultUIHostNotifier) Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error) {
	return "", ErrDialogHostUnavailable
}

func (n *DefaultUIHostNotifier) Navigate(ctx context.Context, extensionID string, target string) error {
	return ErrNavigationHostUnavailable
}
