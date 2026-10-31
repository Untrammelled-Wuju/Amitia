package workbenchterminal

import (
	"context"
	"errors"
	amitialog "log"
)

func (h *Handler) validateTask(ctx context.Context, owner, id, conversationID string) error {
	s, err := h.manager.get(owner, id)
	if err != nil {
		return err
	}
	if s.taskID == "" {
		return nil
	}
	if h.taskVerifier == nil || !taskIDPattern.MatchString(s.taskID) ||
		!conversationPattern.MatchString(conversationID) || s.conversationID != conversationID {
		if err := h.manager.RevokeSession(owner, id); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return ErrTaskBinding
	}
	if err = h.taskVerifier(ctx, s.workspaceURI, s.taskID, conversationID); err != nil {
		if errors.Is(err, ErrTaskBinding) {
			if err := h.manager.RevokeSession(owner, id); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
		return err
	}
	return nil
}
