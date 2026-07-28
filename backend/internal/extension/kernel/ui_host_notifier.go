package kernel

import (
	"context"
	"log"
)

type DefaultUIHostNotifier struct{}

func NewDefaultUIHostNotifier() *DefaultUIHostNotifier {
	return &DefaultUIHostNotifier{}
}

func (n *DefaultUIHostNotifier) Notify(ctx context.Context, extensionID string, title string, body string, severity string) error {
	log.Printf("[UI Notify] ext=%s title=%s severity=%s body=%s", extensionID, title, severity, body)
	return nil
}

func (n *DefaultUIHostNotifier) Dialog(ctx context.Context, extensionID string, dialogID string, message string, buttons []string) (string, error) {
	log.Printf("[UI Dialog] ext=%s dialog=%s message=%s buttons=%v", extensionID, dialogID, message, buttons)
	return "ok", nil
}

func (n *DefaultUIHostNotifier) Navigate(ctx context.Context, extensionID string, target string) error {
	log.Printf("[UI Navigate] ext=%s target=%s", extensionID, target)
	return nil
}
