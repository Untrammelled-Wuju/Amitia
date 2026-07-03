package affect

import "time"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) LoadOrDefault(characterID string, now time.Time) *AffectState {
	state, err := s.repo.LoadState(characterID)
	if err != nil || state == nil {
		def := DefaultState(now)
		return &def
	}
	return state
}

func (s *Service) ProcessEvent(characterID string, input EngineInput) (*EngineOutput, error) {
	current := s.LoadOrDefault(characterID, input.Now)
	if input.Current.Version == "" && current != nil {
		input.Current = *current
	}
	output := ComputeNextState(input)
	if err := s.repo.SaveState(characterID, output.State); err != nil {
		return nil, err
	}
	return &output, nil
}
