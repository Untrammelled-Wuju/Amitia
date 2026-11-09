package desktop

import (
	"errors"
	"net/http"
)

func HTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrContributionNotFound),
		errors.Is(err, ErrContractNotFound),
		errors.Is(err, ErrContractVersionNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidContractID),
		errors.Is(err, ErrInvalidDefinition),
		errors.Is(err, ErrInvalidMenuTarget),
		errors.Is(err, ErrInvalidTrayTarget),
		errors.Is(err, ErrInvalidShortcut),
		errors.Is(err, ErrInvalidAccelerator),
		errors.Is(err, ErrInvalidActionType),
		errors.Is(err, ErrForbiddenActionType),
		errors.Is(err, ErrInvalidOrder),
		errors.Is(err, ErrInvalidVisibility),
		errors.Is(err, ErrInvalidIconResource),
		errors.Is(err, ErrInvalidPlatform),
		errors.Is(err, ErrUnsupportedPlatform),
		errors.Is(err, ErrInvalidScope),
		errors.Is(err, ErrInvalidPermissionReq),
		errors.Is(err, ErrInvalidConflictResolution),
		errors.Is(err, ErrSeparatorNotAllowed),
		errors.Is(err, ErrRadioGroupViolation),
		errors.Is(err, ErrTooManyItems):
		return http.StatusBadRequest
	case errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrScopeDenied),
		errors.Is(err, ErrResourceNotOwned):
		return http.StatusForbidden
	case errors.Is(err, ErrContributionExists),
		errors.Is(err, ErrShortcutExists),
		errors.Is(err, ErrShortcutConflict),
		errors.Is(err, ErrMenuConflict),
		errors.Is(err, ErrTrayConflict),
		errors.Is(err, ErrActionConflict),
		errors.Is(err, ErrSnapshotConflict),
		errors.Is(err, ErrContractFrozen),
		errors.Is(err, ErrGeneration),
		errors.Is(err, ErrReservedShortcut):
		return http.StatusConflict
	case errors.Is(err, ErrHostNotReady),
		errors.Is(err, ErrCircuitOpen),
		errors.Is(err, ErrQuarantined),
		errors.Is(err, ErrDependencyNotMet):
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}
