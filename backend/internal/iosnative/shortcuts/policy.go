package shortcuts

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultListLimit              = 20
	MaxListLimit                  = 100
	MaxSuggestedEntities          = 10
	MaxDynamicOptionsLimit        = 50
	MaxActionIDLength             = 128
	MaxParameterBytes             = 64 * 1024
	MaxMessageBytes               = 32 * 1024
	MaxResultBytes                = 32 * 1024
	MaxConfirmationMessageChars   = 512
	MaxObjectNameChars            = 256
	MaxSiriPhraseChars            = 256
	MinIdempotencyKeyLength       = 16
	DefaultSnapshotTTLSeconds     = 300
	MaxSnapshotEntities           = 50
	MaxCatalogActions             = 200
	MaxIntentDonationParameters   = 10
	EntityQueryTimeoutSec         = 3
	DefaultActionTimeoutSec       = 30
	ShortcutsSchemaVersion        = 1
)

var AllowedCanonicalTargets = []ShortcutCanonicalTarget{
	ShortcutCanonicalTargetService,
	ShortcutCanonicalTargetNativeCore,
	ShortcutCanonicalTargetTool,
	ShortcutCanonicalTargetInteraction,
}

var AllowedExecutionModes = []ShortcutExecutionMode{
	ShortcutExecutionModeBackgroundSafe,
	ShortcutExecutionModeForegroundDynamic,
	ShortcutExecutionModeForegroundImmediate,
	ShortcutExecutionModeForegroundDeferred,
}

var AllowedRiskLevels = []ShortcutRiskLevel{
	ShortcutRiskLevelReadOnly,
	ShortcutRiskLevelMedium,
	ShortcutRiskLevelUIMediated,
	ShortcutRiskLevelHigh,
}

var HighRiskActions = map[string]bool{
	"cancel_alarm":       true,
	"cancel_repeating_alarm": true,
	"delete_alarm":       true,
	"homekit_write":      true,
	"bluetooth_write":    true,
	"workspace_delete":   true,
	"memory_delete":      true,
	"limited_photo_delete": true,
}

func IsValidCanonicalTarget(target ShortcutCanonicalTarget) bool {
	for _, t := range AllowedCanonicalTargets {
		if t == target {
			return true
		}
	}
	return false
}

func IsValidExecutionMode(mode ShortcutExecutionMode) bool {
	for _, m := range AllowedExecutionModes {
		if m == mode {
			return true
		}
	}
	return false
}

func IsValidRiskLevel(risk ShortcutRiskLevel) bool {
	for _, r := range AllowedRiskLevels {
		if r == risk {
			return true
		}
	}
	return false
}

func IsValidExposure(exposure ShortcutExposure) bool {
	switch exposure {
	case ShortcutExposureNone, ShortcutExposureSiri, ShortcutExposureShortcuts,
		ShortcutExposureSpotlight, ShortcutExposureAppShortcut, ShortcutExposureAll:
		return true
	}
	return false
}

func IsHighRiskAction(actionID string) bool {
	return HighRiskActions[actionID]
}

func ValidateActionID(actionID string) error {
	if actionID == "" {
		return fmt.Errorf("%v: actionId is required", ErrShortcutsParameterRequired)
	}
	if len(actionID) > MaxActionIDLength {
		return fmt.Errorf("%v: actionId length %d exceeds max %d", ErrShortcutsParameterInvalid, len(actionID), MaxActionIDLength)
	}
	if strings.ContainsAny(actionID, " \t\n\r") {
		return fmt.Errorf("%v: actionId contains whitespace", ErrShortcutsParameterInvalid)
	}
	return nil
}

func ValidateEntityID(entityID string) error {
	if entityID == "" {
		return fmt.Errorf("%v: entityId is required", ErrShortcutsEntityNotFound)
	}
	if len(entityID) > MaxActionIDLength {
		return fmt.Errorf("%v: entityId too long", ErrShortcutsEntityNotFound)
	}
	return nil
}

func ValidateParameters(params map[string]any) error {
	if params == nil {
		return nil
	}
	for k, v := range params {
		if k == "" {
			return fmt.Errorf("%v: empty parameter key", ErrShortcutsParameterInvalid)
		}
		if s, ok := v.(string); ok {
			if len(s) > MaxParameterBytes {
				return fmt.Errorf("%v: parameter %q exceeds max bytes", ErrShortcutsParameterInvalid, k)
			}
		}
	}
	return nil
}

func ValidateMessage(message string) error {
	if message == "" {
		return nil
	}
	if len(message) > MaxMessageBytes {
		return fmt.Errorf("%v: message length %d exceeds max %d", ErrShortcutsParameterInvalid, len(message), MaxMessageBytes)
	}
	return nil
}

func ValidateActionRequest(req ShortcutActionRequest) error {
	if err := ValidateActionID(req.ActionID); err != nil {
		return err
	}
	if err := ValidateParameters(req.Parameters); err != nil {
		return err
	}
	if req.Invocation.IdempotencyKey != "" && len(req.Invocation.IdempotencyKey) < MinIdempotencyKeyLength {
		return fmt.Errorf("%v: idempotencyKey too short, min %d", ErrShortcutsIdempotencyInvalid, MinIdempotencyKeyLength)
	}
	return nil
}

func ValidateQueryResultCount(count int) error {
	if count > MaxListLimit {
		return fmt.Errorf("%v: result count %d exceeds max %d", ErrShortcutsResultTooLarge, count, MaxListLimit)
	}
	return nil
}

func ValidateResultSize(result string) error {
	if len(result) > MaxResultBytes {
		return fmt.Errorf("%v: result size %d exceeds max %d", ErrShortcutsResultTooLarge, len(result), MaxResultBytes)
	}
	return nil
}

func ValidateConfirmationRequest(req ConfirmationRequest) error {
	if req.ActionID == "" {
		return fmt.Errorf("%v: actionId is required for confirmation", ErrShortcutsParameterRequired)
	}
	if req.Title == "" {
		return fmt.Errorf("%v: title is required for confirmation", ErrShortcutsConfirmationRequired)
	}
	if utf8.RuneCountInString(req.Message) > MaxConfirmationMessageChars {
		return fmt.Errorf("%v: message too long for confirmation", ErrShortcutsConfirmationRequired)
	}
	if utf8.RuneCountInString(req.ObjectName) > MaxObjectNameChars {
		return fmt.Errorf("%v: objectName too long", ErrShortcutsParameterInvalid)
	}
	return nil
}

func ValidateShortcutPhrase(phrase string) error {
	if phrase == "" {
		return fmt.Errorf("%v: phrase is required", ErrShortcutsPhraseInvalid)
	}
	if utf8.RuneCountInString(phrase) > MaxSiriPhraseChars {
		return fmt.Errorf("%v: phrase too long", ErrShortcutsPhraseInvalid)
	}
	return nil
}

func ValidateContribution(contrib ShortcutContribution) error {
	if contrib.ActionID == "" {
		return fmt.Errorf("%v: actionId is required for contribution", ErrShortcutsParameterRequired)
	}
	if contrib.Title == "" {
		return fmt.Errorf("%v: title is required for contribution", ErrShortcutsParameterRequired)
	}
	if !IsValidRiskLevel(contrib.Risk) {
		return fmt.Errorf("%v: invalid risk level %q", ErrShortcutsRiskLevelInvalid, contrib.Risk)
	}
	if !IsValidExposure(contrib.Exposure) {
		return fmt.Errorf("%v: invalid exposure %q", ErrShortcutsExposureInvalid, contrib.Exposure)
	}
	if utf8.RuneCountInString(contrib.Title) > MaxObjectNameChars {
		return fmt.Errorf("%v: contribution title too long", ErrShortcutsParameterInvalid)
	}
	return nil
}

func ValidateDonation(req IntentDonationRequest) error {
	if req.IntentID == "" {
		return fmt.Errorf("%v: intentId is required for donation", ErrShortcutsParameterRequired)
	}
	if len(req.Parameters) > MaxIntentDonationParameters {
		return fmt.Errorf("%v: donation parameters too many, max %d", ErrShortcutsParameterInvalid, MaxIntentDonationParameters)
	}
	return nil
}

func ClampLimit(limit int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

func ClampLimitWithMax(limit int, max int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > max {
		return max
	}
	return limit
}

func RiskRequiresConfirmation(risk ShortcutRiskLevel) bool {
	return risk == ShortcutRiskLevelHigh || risk == ShortcutRiskLevelUIMediated
}

func RiskAllowsBackground(risk ShortcutRiskLevel) bool {
	return risk == ShortcutRiskLevelReadOnly
}
