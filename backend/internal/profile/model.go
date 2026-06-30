// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

type UserProfile struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	UserID         string `gorm:"column:user_id;not null" json:"userId"`
	Category       string `gorm:"column:category;not null" json:"category"`
	AttributeName  string `gorm:"column:attribute_name;not null" json:"attributeName"`
	AttributeValue string `gorm:"column:attribute_value;not null" json:"attributeValue"`
	Confidence     int    `gorm:"column:confidence;default:50" json:"confidence"`
	SourceConvID   string `gorm:"column:source_conv_id" json:"sourceConvId"`
	VerifiedAt     string `gorm:"column:verified_at" json:"verifiedAt"`
	CreatedAt      string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      string `gorm:"column:updated_at" json:"updatedAt"`
}

func (UserProfile) TableName() string { return "user_profiles" }

type CreateProfileRequest struct {
	UserID         string `json:"userId"`
	Category       string `json:"category"`
	AttributeName  string `json:"attributeName" binding:"required"`
	AttributeValue string `json:"attributeValue" binding:"required"`
	Confidence     int    `json:"confidence"`
	SourceConvID   string `json:"sourceConvId"`
}

type UpdateProfileRequest struct {
	AttributeValue *string `json:"attributeValue"`
	Confidence     *int    `json:"confidence"`
	Verified       *bool   `json:"verified"`
}

type ProfileListQuery struct {
	UserID   string `form:"userId"`
	Category string `form:"category"`
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
}

type ProfileListResponse struct {
	Items      []UserProfile `json:"items"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}
