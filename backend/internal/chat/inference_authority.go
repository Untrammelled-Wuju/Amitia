package chat

import "context"

type InferenceAuthority func(context.Context) (context.Context, func(), error)

func (s *service) SetInferenceAuthority(authority InferenceAuthority) {
	s.inferenceAuthority = authority
}

func (s *service) beginInference(ctx context.Context) (context.Context, func(), error) {
	if err := context.Cause(ctx); err != nil {
		return nil, nil, err
	}
	if s.inferenceAuthority != nil {
		return s.inferenceAuthority(ctx)
	}
	return ctx, func() {}, nil
}

func (s *service) requireInferenceAuthority(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if s.inferenceAuthority == nil {
		return nil
	}
	_, finish, err := s.inferenceAuthority(ctx)
	if finish != nil {
		finish()
	}
	return err
}
