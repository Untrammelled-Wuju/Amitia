package background

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultListLimit             = 50
	MaxListLimit                 = 200
	MaxMaxPendingPerClass        = 1
	MaxIdentifierLength          = 128
	MaxTaskRunIDLength           = 128
	MaxReasonLength              = 256
	MaxTitleLength               = 64
	MaxSubtitleLength            = 128
	MaxPathLength                = 1024
 MaxRelativePathLength        = 512
	MaxContentBytes              = 100 * 1024 * 1024
	MaxExportBytes               = 500 * 1024 * 1024
	MaxCheckpointBytes           = 64 * 1024
	DefaultBackgroundTimeoutSec  = 30
	DefaultCheckpointTTLHours    = 24
	MaxCheckpointTTLHours        = 168
	MaxStaleBookmarkAgeHours     = 24
	MaxMaterializeBytes          = 500 * 1024 * 1024
)

var AllowedSystemClasses = []BackgroundSystemClass{
	BackgroundClassRefresh,
	BackgroundClassProcessing,
	BackgroundClassContinued,
	BackgroundClassCleanup,
}

var AllowedContinuedStrategies = []ContinuedTaskStrategy{
	ContinuedStrategyQueueIfNeeded,
	ContinuedStrategyFailIfNotImmediate,
}

var AllowedContinuedInitiators = []TaskInitiator{
	InitiatorUser,
	InitiatorForegroundShortcut,
	InitiatorExplicitAppIntent,
}

func IsValidSystemClass(class BackgroundSystemClass) bool {
	for _, c := range AllowedSystemClasses {
		if c == class {
			return true
		}
	}
	return false
}

func IsValidContinuedStrategy(strategy ContinuedTaskStrategy) bool {
	for _, s := range AllowedContinuedStrategies {
		if s == strategy {
			return true
		}
	}
	return false
}

func IsValidContinuedInitiator(initiator TaskInitiator) bool {
	for _, i := range AllowedContinuedInitiators {
		if i == initiator {
			return true
		}
	}
	return false
}

func ValidateIdentifier(identifier string) error {
	if identifier == "" {
		return fmt.Errorf("%v: identifier is required", ErrBackgroundIdentifierInvalid)
	}
	if len(identifier) > MaxIdentifierLength {
		return fmt.Errorf("%v: identifier length %d exceeds max %d", ErrBackgroundIdentifierInvalid, len(identifier), MaxIdentifierLength)
	}
	if strings.ContainsAny(identifier, " \t\n\r") {
		return fmt.Errorf("%v: identifier contains whitespace", ErrBackgroundIdentifierInvalid)
	}
	return nil
}

func ValidateTaskRunID(taskRunID string) error {
	if taskRunID == "" {
		return fmt.Errorf("%v: taskRunId is required", ErrBackgroundTaskBindingInvalid)
	}
	if len(taskRunID) > MaxTaskRunIDLength {
		return fmt.Errorf("%v: taskRunId too long", ErrBackgroundTaskBindingInvalid)
	}
	return nil
}

func ValidateSubmission(req BackgroundSubmissionRequest) error {
	if !IsValidSystemClass(req.SystemClass) {
		return fmt.Errorf("%v: invalid system class %q", ErrBackgroundIdentifierInvalid, req.SystemClass)
	}
	if req.IdentifierClass == "" {
		return fmt.Errorf("%v: identifierClass is required", ErrBackgroundIdentifierInvalid)
	}

	if req.SystemClass == BackgroundClassContinued {
		if !IsValidContinuedInitiator(req.Initiator) {
			return fmt.Errorf("%v: continued processing requires user/initiator, got %q", ErrBackgroundNotUserInitiated, req.Initiator)
		}
		if req.TaskRunID == "" {
			return fmt.Errorf("%v: continued processing requires taskRunId", ErrBackgroundTaskBindingInvalid)
		}
		if req.Strategy == "" {
			req.Strategy = ContinuedStrategyQueueIfNeeded
		} else if !IsValidContinuedStrategy(req.Strategy) {
			return fmt.Errorf("%v: invalid continued strategy %q", ErrBackgroundSubmissionFailed, req.Strategy)
		}
		if utf8.RuneCountInString(req.Title) > MaxTitleLength {
			return fmt.Errorf("%v: title exceeds max %d chars", ErrBackgroundSubmissionFailed, MaxTitleLength)
		}
		if utf8.RuneCountInString(req.Subtitle) > MaxSubtitleLength {
			return fmt.Errorf("%v: subtitle exceeds max %d chars", ErrBackgroundSubmissionFailed, MaxSubtitleLength)
		}
	} else {
		if req.TaskRunID == "" && req.TaskDefinitionID == "" {
			return fmt.Errorf("%v: either taskRunId or taskDefinitionId required", ErrBackgroundTaskBindingInvalid)
		}
	}

	return nil
}

func ValidateRelativePath(path string) error {
	if path == "" {
		return fmt.Errorf("%v: relativePath is required", ErrFilePathInvalid)
	}
	if utf8.RuneCountInString(path) > MaxRelativePathLength {
		return fmt.Errorf("%v: relativePath too long", ErrFilePathInvalid)
	}
	if strings.Contains(path, "../") || strings.Contains(path, "..\\") {
		return fmt.Errorf("%v: path traversal not allowed", ErrFilePathInvalid)
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return fmt.Errorf("%v: absolute path not allowed", ErrFilePathInvalid)
	}
	return nil
}

func ValidateReadRequest(req IOSFileReadRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	if err := ValidateRelativePath(req.RelativePath); err != nil {
		return err
	}
	if req.Offset < 0 {
		return fmt.Errorf("%v: offset must be non-negative", ErrFilePathInvalid)
	}
	if req.Length < 0 {
		return fmt.Errorf("%v: length must be non-negative", ErrFilePathInvalid)
	}
	return nil
}

func ValidateWriteRequest(req IOSFileWriteRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	if err := ValidateRelativePath(req.RelativePath); err != nil {
		return err
	}
	if int64(len(req.Content)) > MaxContentBytes {
		return fmt.Errorf("%v: content size %d exceeds max %d", ErrFileSizeLimitExceeded, len(req.Content), MaxContentBytes)
	}
	return nil
}

func ValidateMkdirRequest(req IOSFileMkdirRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	return ValidateRelativePath(req.RelativePath)
}

func ValidateRenameRequest(req IOSFileRenameRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	if err := ValidateRelativePath(req.RelativePath); err != nil {
		return err
	}
	if req.NewName == "" {
		return fmt.Errorf("%v: newName is required", ErrFilePathInvalid)
	}
	if strings.ContainsAny(req.NewName, "/\\") {
		return fmt.Errorf("%v: newName contains path separator", ErrFilePathInvalid)
	}
	return nil
}

func ValidateMoveRequest(req IOSFileMoveRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	if err := ValidateRelativePath(req.RelativePath); err != nil {
		return err
	}
	return ValidateRelativePath(req.NewRelativePath)
}

func ValidateCopyRequest(req IOSFileCopyRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	if err := ValidateRelativePath(req.RelativePath); err != nil {
		return err
	}
	return ValidateRelativePath(req.NewRelativePath)
}

func ValidateDeleteRequest(req IOSFileDeleteRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	return ValidateRelativePath(req.RelativePath)
}

func ValidateExportRequest(req IOSFileExportRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	if err := ValidateRelativePath(req.RelativePath); err != nil {
		return err
	}
	if req.ResourceURI == "" {
		return fmt.Errorf("%v: resourceUri is required", ErrFileExportFailed)
	}
	return nil
}

func ValidateStatRequest(req IOSFileAccessRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	return ValidateRelativePath(req.RelativePath)
}

func ValidateListRequest(req IOSFileAccessRequest) error {
	if req.MountID == "" {
		return fmt.Errorf("%v: mountId is required", ErrFileGrantInvalid)
	}
	return ValidateRelativePath(req.RelativePath)
}

func ValidateProgress(progress BackgroundTaskProgress) error {
	if progress.TaskRunID == "" {
		return fmt.Errorf("%v: taskRunId is required", ErrBackgroundTaskBindingInvalid)
	}
	if progress.CompletedUnits < 0 || progress.TotalUnits < 0 {
		return fmt.Errorf("%v: progress values must be non-negative", ErrBackgroundProgressInvalid)
	}
	if progress.TotalUnits > 0 && progress.CompletedUnits > progress.TotalUnits {
		return fmt.Errorf("%v: completed units exceeds total", ErrBackgroundProgressInvalid)
	}
	return nil
}

func ValidateCheckpoint(req BackgroundCheckpointSetRequest) error {
	if req.TaskRunID == "" {
		return fmt.Errorf("%v: taskRunId is required", ErrBackgroundTaskBindingInvalid)
	}
	if req.Generation < 0 {
		return fmt.Errorf("%v: generation must be non-negative", ErrBackgroundTaskBindingInvalid)
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
