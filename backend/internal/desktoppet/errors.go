// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package desktoppet

const (
	ErrCodeDesktopPetNameRequired      = "DESKTOP_PET_NAME_REQUIRED"
	ErrCodeReferenceImageRequired      = "REFERENCE_IMAGE_REQUIRED"
	ErrCodeReferenceImageInvalid       = "REFERENCE_IMAGE_INVALID"
	ErrCodeReferenceImageTooLarge      = "REFERENCE_IMAGE_TOO_LARGE"
	ErrCodeCharacterNotFound           = "CHARACTER_NOT_FOUND"
	ErrCodeImageModelNotFound          = "IMAGE_MODEL_NOT_FOUND"
	ErrCodeImageModelDisabled          = "IMAGE_MODEL_DISABLED"
	ErrCodeImageModelTypeUnsupported   = "IMAGE_MODEL_TYPE_UNSUPPORTED"
	ErrCodeActionSelectionRequired     = "ACTION_SELECTION_REQUIRED"
	ErrCodeActionNotFound              = "ACTION_NOT_FOUND"
	ErrCodeActionDisabled              = "ACTION_DISABLED"
	ErrCodeDefaultIdleActionRequired   = "DEFAULT_IDLE_ACTION_REQUIRED"
	ErrCodeGenerationTaskCreateFailed  = "GENERATION_TASK_CREATE_FAILED"
	ErrCodeGenerationTaskNotFound      = "GENERATION_TASK_NOT_FOUND"
	ErrCodeTaskStatusNotDeletable      = "TASK_STATUS_NOT_DELETABLE"
)

type BusinessError struct {
	Code    int
	Msg     string
	ErrCode string
}

func (e *BusinessError) Error() string { return e.Msg }

func NewBusinessError(code int, errCode, msg string) *BusinessError {
	return &BusinessError{Code: code, Msg: msg, ErrCode: errCode}
}
