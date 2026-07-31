// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package referenceasset

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrReferenceAssetNotFound = errors.New("reference asset not found")
)

type Repository interface {
	CreateReferenceAsset(asset *ReferenceAsset) error
	CreateReferenceAssetTx(tx *gorm.DB, asset *ReferenceAsset) error
	GetReferenceAssetByID(id string) (*ReferenceAsset, error)
	GetReferenceAssetByIDTx(tx *gorm.DB, id string) (*ReferenceAsset, error)
	GetReferenceAssetByTaskID(taskID string) (*ReferenceAsset, error)
	GetReferenceAssetByTaskIDTx(tx *gorm.DB, taskID string) (*ReferenceAsset, error)
	UpdateReferenceAssetStatus(tx *gorm.DB, id string, status string) error
	DeleteReferenceAsset(id string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateReferenceAsset(asset *ReferenceAsset) error {
	return r.db.Create(asset).Error
}

func (r *repository) CreateReferenceAssetTx(tx *gorm.DB, asset *ReferenceAsset) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Create(asset).Error
}

func (r *repository) GetReferenceAssetByID(id string) (*ReferenceAsset, error) {
	var asset ReferenceAsset
	err := r.db.Where("id = ?", id).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReferenceAssetNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (r *repository) GetReferenceAssetByIDTx(tx *gorm.DB, id string) (*ReferenceAsset, error) {
	if tx == nil {
		tx = r.db
	}
	var asset ReferenceAsset
	err := tx.Where("id = ?", id).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReferenceAssetNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (r *repository) GetReferenceAssetByTaskID(taskID string) (*ReferenceAsset, error) {
	var asset ReferenceAsset
	err := r.db.Where("task_id = ?", taskID).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReferenceAssetNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (r *repository) GetReferenceAssetByTaskIDTx(tx *gorm.DB, taskID string) (*ReferenceAsset, error) {
	if tx == nil {
		tx = r.db
	}
	var asset ReferenceAsset
	err := tx.Where("task_id = ?", taskID).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReferenceAssetNotFound
		}
		return nil, err
	}
	return &asset, nil
}

func (r *repository) UpdateReferenceAssetStatus(tx *gorm.DB, id string, status string) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Model(&ReferenceAsset{}).Where("id = ?", id).Update("status", status).Error
}

func (r *repository) DeleteReferenceAsset(id string) error {
	return r.db.Where("id = ?", id).Delete(&ReferenceAsset{}).Error
}
