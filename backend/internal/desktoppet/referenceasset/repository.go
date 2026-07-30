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
	GetReferenceAssetByID(id string) (*ReferenceAsset, error)
	GetReferenceAssetByTaskID(taskID string) (*ReferenceAsset, error)
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

func (r *repository) DeleteReferenceAsset(id string) error {
	return r.db.Where("id = ?", id).Delete(&ReferenceAsset{}).Error
}
