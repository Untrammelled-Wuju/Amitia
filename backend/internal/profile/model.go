// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package profile

type UserProfile struct {
	ID               string `gorm:"column:id;primaryKey" json:"id"`
	UserID           string `gorm:"column:user_id;not null" json:"userId"`
	CharacterID      string `gorm:"column:character_id;not null;default:''" json:"characterId"`
	Category         string `gorm:"column:category;not null" json:"category"`
	AttributeName    string `gorm:"column:attribute_name;not null" json:"attributeName"`
	AttributeValue   string `gorm:"column:attribute_value;not null" json:"attributeValue"`
	Confidence       int    `gorm:"column:confidence;default:50" json:"confidence"`
	Source           string `gorm:"column:source;not null;default:''" json:"source"`
	SourceConvID     string `gorm:"column:source_conv_id" json:"sourceConvId"`
	VerifiedAt       string `gorm:"column:verified_at" json:"verifiedAt"`
	CreatedAt        string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        string `gorm:"column:updated_at" json:"updatedAt"`
	SourceMemoryID   string `gorm:"column:source_memory_id;default:''" json:"sourceMemoryId"`
	ProjectionStatus string `gorm:"column:projection_status;default:active" json:"projectionStatus"`
}

func (UserProfile) TableName() string { return "user_profiles" }

func clampProfileConfidence(confidence int) int {
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return confidence
}

func clampProfileConfidenceValue(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return clampProfileConfidence(v)
	case int8:
		return clampProfileConfidence(int(v))
	case int16:
		return clampProfileConfidence(int(v))
	case int32:
		return clampProfileConfidence(int(v))
	case int64:
		return clampProfileConfidence(int(v))
	case uint:
		return clampProfileConfidence(int(v))
	case uint8:
		return clampProfileConfidence(int(v))
	case uint16:
		return clampProfileConfidence(int(v))
	case uint32:
		return clampProfileConfidence(int(v))
	case uint64:
		if v > 100 {
			return 100
		}
		return int(v)
	default:
		return value
	}
}

type CreateProfileRequest struct {
	UserID         string `json:"userId"`
	CharacterID    string `json:"characterId"`
	Category       string `json:"category"`
	AttributeName  string `json:"attributeName" binding:"required"`
	AttributeValue string `json:"attributeValue" binding:"required"`
	Confidence     int    `json:"confidence"`
	Source         string `json:"source"`
	SourceConvID   string `json:"sourceConvId"`
}

type UpdateProfileRequest struct {
	AttributeValue *string `json:"attributeValue"`
	Confidence     *int    `json:"confidence"`
	Verified       *bool   `json:"verified"`
}

type ProfileListQuery struct {
	Keyword     string `form:"keyword"`
	UserID      string `form:"userId"`
	CharacterID string `form:"characterId"`
	Category    string `form:"category"`
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
}

type ProfileListResponse struct {
	Items      []UserProfile `json:"items"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}
