package memory

import "context"

type WorkingMemoryService struct{}

func NewWorkingMemoryService() *WorkingMemoryService {
	return &WorkingMemoryService{}
}

func (s *WorkingMemoryService) Name() string { return "工作记忆" }

func (s *WorkingMemoryService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return ErrSkip
}
