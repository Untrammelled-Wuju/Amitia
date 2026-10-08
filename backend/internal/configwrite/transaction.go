package configwrite

import (
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"gorm.io/gorm"
)

func Transaction(db *gorm.DB, write func(*gorm.DB) error) error {
	return TransactionAndApply(db, write, nil)
}

func TransactionAndApply(db *gorm.DB, write func(*gorm.DB) error, apply func()) error {
	ctx := db.Statement.Context
	return coordination.CommitCurrent(ctx, func() error {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		if err := db.WithContext(ctx).Transaction(write); err != nil {
			return err
		}
		if apply != nil {
			apply()
		}
		return nil
	})
}
