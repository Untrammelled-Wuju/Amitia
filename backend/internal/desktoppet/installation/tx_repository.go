// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type TxRepository struct {
	repoV2   RepositoryV2
	tx       *gorm.DB
	finished bool
}

func NewTxRepository(db *gorm.DB) *TxRepository {
	return &TxRepository{tx: db}
}

func (t *TxRepository) SetBaseRepo(baseRepo RepositoryV2) {
	t.repoV2 = baseRepo
}

func (t *TxRepository) DB() *gorm.DB {
	return t.tx
}

func (t *TxRepository) Commit() error {
	if t.finished {
		return errors.New("tx repository: already finished")
	}
	t.finished = true
	return t.tx.Commit().Error
}

func (t *TxRepository) Rollback() error {
	if t.finished {
		return errors.New("tx repository: already finished")
	}
	t.finished = true
	return t.tx.Rollback().Error
}

func WithTx(db *gorm.DB, repo RepositoryV2, fn func(txRepo *TxRepository) error) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	txRepo := &TxRepository{
		repoV2: repo,
		tx:     tx,
	}
	defer func() {
		if r := recover(); r != nil {
			txRepo.Rollback()
			panic(r)
		}
	}()
	if err := fn(txRepo); err != nil {
		txRepo.Rollback()
		return err
	}
	return txRepo.Commit()
}

func BeginTxContext(ctx context.Context, repo RepositoryV2) (*TxRepository, error) {
	db := repo.DB()
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	txRepo := &TxRepository{
		repoV2: repo,
		tx:     tx,
	}
	return txRepo, nil
}
