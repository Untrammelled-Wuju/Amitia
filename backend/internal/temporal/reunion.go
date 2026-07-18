package temporal

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"time"
)

func reunionLevel(actualGapSeconds, normalizedGap float64) ReunionLevel {
	if actualGapSeconds < (12 * time.Hour).Seconds() {
		return ReunionLevelNone
	}
	absoluteScore := 0
	switch {
	case actualGapSeconds >= (90 * 24 * time.Hour).Seconds():
		absoluteScore = 5
	case actualGapSeconds >= (30 * 24 * time.Hour).Seconds():
		absoluteScore = 4
	case actualGapSeconds >= (7 * 24 * time.Hour).Seconds():
		absoluteScore = 3
	case actualGapSeconds >= (2 * 24 * time.Hour).Seconds():
		absoluteScore = 2
	default:
		absoluteScore = 1
	}
	relativeScore := 0
	switch {
	case normalizedGap >= 20:
		relativeScore = 4
	case normalizedGap >= 7:
		relativeScore = 3
	case normalizedGap >= 3:
		relativeScore = 2
	case normalizedGap >= 1.5:
		relativeScore = 1
	}
	score := int(math.Floor(float64(absoluteScore)*0.55 + float64(relativeScore)*0.45 + 0.5))
	level := ReunionLevelNone
	switch {
	case score >= 4:
		level = ReunionLevelDormant
	case score == 3:
		level = ReunionLevelExtended
	case score == 2:
		level = ReunionLevelLong
	case score == 1:
		level = ReunionLevelNoticeable
	}
	switch {
	case actualGapSeconds >= (90 * 24 * time.Hour).Seconds():
		level = ReunionLevelDormant
	case actualGapSeconds >= (30*24*time.Hour).Seconds() && reunionLevelRank(level) < reunionLevelRank(ReunionLevelExtended):
		level = ReunionLevelExtended
	case actualGapSeconds >= (7*24*time.Hour).Seconds() && reunionLevelRank(level) < reunionLevelRank(ReunionLevelLong):
		level = ReunionLevelLong
	}
	return level
}

func reunionLevelRank(level ReunionLevel) int {
	return map[ReunionLevel]int{ReunionLevelNone: 0, ReunionLevelNoticeable: 1, ReunionLevelLong: 2, ReunionLevelExtended: 3, ReunionLevelDormant: 4}[level]
}

func classifyReunionKind(globalGap, expectedGap float64, level ReunionLevel, lastAssistantContact, now time.Time) ReunionKind {
	if level == ReunionLevelNone {
		return ""
	}
	if !lastAssistantContact.IsZero() && now.Sub(lastAssistantContact) >= 0 && now.Sub(lastAssistantContact) <= 72*time.Hour {
		return ReunionKindReplyToProactive
	}
	if reunionLevel(globalGap, relationshipGapToNormalized(globalGap, expectedGap)) != ReunionLevelNone {
		return ReunionKindGlobalReturn
	}
	return ReunionKindRelationshipReconnect
}

func relationshipGapToNormalized(globalGap, expectedGap float64) float64 {
	if expectedGap <= 0 {
		return 0
	}
	return globalGap / expectedGap
}

func reunionIdempotencyKey(userID, characterID, previousCommitted string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(characterID) + "\x00" + previousCommitted))
	return hex.EncodeToString(sum[:])
}

func continuityScore(gapSeconds, expectedGapSeconds float64) float64 {
	if gapSeconds <= 0 {
		return 1
	}
	horizon := math.Max(expectedGapSeconds*3, (7 * 24 * time.Hour).Seconds())
	return roundFinite(clampFloat(math.Exp(-gapSeconds/horizon), 0.15, 1))
}

func reacclimationTurns(level ReunionLevel) int {
	return map[ReunionLevel]int{ReunionLevelLong: 1, ReunionLevelExtended: 2, ReunionLevelDormant: 3}[level]
}

func temporaryWarmth(level ReunionLevel) float64 {
	return map[ReunionLevel]float64{ReunionLevelNoticeable: 0.02, ReunionLevelLong: 0.04, ReunionLevelExtended: 0.06, ReunionLevelDormant: 0.08}[level]
}

func temporarySocialArousal(level ReunionLevel) float64 {
	return map[ReunionLevel]float64{ReunionLevelNoticeable: 0.01, ReunionLevelLong: 0.03, ReunionLevelExtended: 0.05, ReunionLevelDormant: 0.07}[level]
}
