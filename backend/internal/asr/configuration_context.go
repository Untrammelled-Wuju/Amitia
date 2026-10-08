package asr

import "context"

func (h *Handler) configurationService(ctx context.Context) Service {
	if svc, ok := h.service.(*service); ok {
		if repo, ok := svc.repo.(*repository); ok {
			return NewService(NewRepository(repo.db.WithContext(ctx)))
		}
	}
	return h.service
}
