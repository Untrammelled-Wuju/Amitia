package resource

import "errors"

var (
	ErrResourceNotFound        = errors.New("resource: resource not found")
	ErrResourceAlreadyExists   = errors.New("resource: resource already exists")
	ErrInvalidResourceType     = errors.New("resource: invalid resource type")
	ErrInvalidOwner            = errors.New("resource: invalid owner")
	ErrInvalidStateTransition  = errors.New("resource: invalid state transition")
	ErrReferenceNotFound       = errors.New("resource: reference not found")
	ErrCircularReference       = errors.New("resource: circular reference detected")
	ErrResourceBusy            = errors.New("resource: resource is busy")
	ErrTransferNotAllowed      = errors.New("resource: ownership transfer not allowed")
	ErrReleaseBlocked          = errors.New("resource: release is blocked")
	ErrReleasePlanNotFound     = errors.New("resource: release plan not found")
	ErrCleanupJobNotFound      = errors.New("resource: cleanup job not found")
	ErrOrphanReportNotFound    = errors.New("resource: orphan report not found")
	ErrInvalidReleaseState     = errors.New("resource: invalid release state")
	ErrDuplicateReference      = errors.New("resource: duplicate reference")
)
