package virtualdisplay

import "fmt"

const (
	ErrVirtualDisplayUnsupported     = "VIRTUAL_DISPLAY_UNSUPPORTED"
	ErrVirtualDisplayAlreadyExists   = "VIRTUAL_DISPLAY_ALREADY_EXISTS"
	ErrVirtualDisplayProperty        = "VIRTUAL_DISPLAY_PROPERTY_NOT_SUPPORTED"
	ErrVirtualDisplayCreate          = "VIRTUAL_DISPLAY_CREATE_FAILED"
	ErrVirtualDisplayNotFound        = "VIRTUAL_DISPLAY_NOT_FOUND"
	ErrVirtualDisplayIdMismatch      = "VIRTUAL_DISPLAY_ID_MISMATCH"
	ErrVirtualDisplayResize          = "VIRTUAL_DISPLAY_RESIZE_FAILED"
	ErrVirtualDisplaySurface         = "VIRTUAL_DISPLAY_SURFACE_OPERATION_FAILED"
	ErrVirtualDisplayOutOfResources  = "VIRTUAL_DISPLAY_OUT_OF_RESOURCES"
	ErrVirtualDisplayNative          = "VIRTUAL_DISPLAY_NATIVE_ERROR"
	ErrVirtualDisplayUnavailable     = "VIRTUAL_DISPLAY_UNAVAILABLE"
)

const (
	PropertyWidth          = "width"
	PropertyHeight         = "height"
	PropertyDensityDPI     = "densityDpi"
	PropertyRefreshRate    = "refreshRate"
	PropertyZOrder         = "zOrder"
)

const (
	VirtualDisplayDefaultWidth     = 1080
	VirtualDisplayDefaultHeight    = 1920
	VirtualDisplayDefaultDensity   = 420
	VirtualDisplayMaxNameRunes     = 64
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func NewError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func NewErrorf(code, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
