package proactive

import (
	"math"
	"time"
)

const (
	IdleThresholdActive    = 1 * time.Hour
	IdleThresholdLonely    = 6 * time.Hour
	IdleThresholdIgnored   = 12 * time.Hour
	IdleThresholdSad       = 24 * time.Hour
	IdleThresholdDepressed = 48 * time.Hour
	IdleThresholdDormant   = 72 * time.Hour
)

type IdleCategory string

const (
	IdleCategoryActive    IdleCategory = "active"
	IdleCategoryLonely    IdleCategory = "lonely"
	IdleCategoryIgnored   IdleCategory = "ignored"
	IdleCategorySad       IdleCategory = "sad"
	IdleCategoryDepressed IdleCategory = "depressed"
	IdleCategoryDormant   IdleCategory = "dormant"
)

func ClassifyIdle(duration time.Duration) IdleCategory {
	switch {
	case duration >= IdleThresholdDormant:
		return IdleCategoryDormant
	case duration >= IdleThresholdDepressed:
		return IdleCategoryDepressed
	case duration >= IdleThresholdSad:
		return IdleCategorySad
	case duration >= IdleThresholdIgnored:
		return IdleCategoryIgnored
	case duration >= IdleThresholdLonely:
		return IdleCategoryLonely
	default:
		return IdleCategoryActive
	}
}

func IdleMood(category IdleCategory) string {
	switch category {
	case IdleCategoryDepressed:
		return "depressed"
	case IdleCategorySad:
		return "sad"
	case IdleCategoryIgnored:
		return "ignored"
	case IdleCategoryLonely:
		return "lonely"
	default:
		return "neutral"
	}
}

func IdleMultiplier(category IdleCategory) float64 {
	switch category {
	case IdleCategoryDormant:
		return 0.1
	case IdleCategoryDepressed:
		return 0.2
	case IdleCategorySad:
		return 0.4
	case IdleCategoryIgnored:
		return 0.6
	case IdleCategoryLonely:
		return 0.8
	default:
		return 1.0
	}
}

func IdleChaseThreshold(category IdleCategory) bool {
	return category == IdleCategoryIgnored || category == IdleCategorySad || category == IdleCategoryDepressed || category == IdleCategoryDormant
}

func IdleDurationRisk(duration time.Duration) float64 {
	if duration < IdleThresholdLonely {
		return 0
	}
	hours := duration.Hours()
	raw := math.Log2(hours/4.0) * 0.15
	if raw < 0 {
		raw = 0
	}
	if raw > 0.55 {
		return 0.55
	}
	return math.Round(raw*1000) / 1000
}
