package embedding

import (
	"context"
	"sync"

	"gorm.io/gorm"
)

type InferenceAuthority func(context.Context) (context.Context, func(), error)

var inferenceAuthorities sync.Map

func SetInferenceAuthority(db *gorm.DB, authority InferenceAuthority) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if authority == nil {
		inferenceAuthorities.Delete(sqlDB)
	} else {
		inferenceAuthorities.Store(sqlDB, authority)
	}
	return nil
}

func (s *Service) beginInference(ctx context.Context) (context.Context, func(), error) {
	if err := context.Cause(ctx); err != nil {
		return nil, nil, err
	}
	db, err := s.db.DB()
	if err != nil {
		return nil, nil, err
	}
	if value, ok := inferenceAuthorities.Load(db); ok {
		return value.(InferenceAuthority)(ctx)
	}
	return ctx, func() {}, nil
}
