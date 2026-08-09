package service_definition

import (
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
)

type ownedDefinition struct {
	DefinitionID string
	ExtensionID  string
	ModuleID     string
}

type FullSyncReport struct {
	TotalExtensions    int                            `json:"totalExtensions"`
	SyncedDefinitions  int                            `json:"syncedDefinitions"`
	FailedDefinitions  int                            `json:"failedDefinitions"`
	Errors             []error                        `json:"errors,omitempty"`
	DefinitionErrors   map[string]error               `json:"definitionErrors,omitempty"`
}

type ReconcileReport struct {
	ExtensionID      string           `json:"extensionId"`
	Added            int              `json:"added"`
	Updated          int              `json:"updated"`
	Removed          int              `json:"removed"`
	Skipped          int              `json:"skipped"`
	Errors           []error          `json:"errors,omitempty"`
	DefinitionErrors map[string]error `json:"definitionErrors,omitempty"`
}

type RemoveReport struct {
	ExtensionID   string  `json:"extensionId"`
	RemovedCount  int     `json:"removedCount"`
	Errors        []error `json:"errors,omitempty"`
}

type DefinitionSyncService struct {
	source         ServiceDefinitionSource
	provider       ServiceDefinitionBatchProvider
	mapper         *DefinitionMapper
	mu             sync.Mutex
	extensionLocks map[string]*sync.Mutex
}

func NewDefinitionSyncService(source ServiceDefinitionSource, provider ServiceDefinitionBatchProvider, mapper *DefinitionMapper) (*DefinitionSyncService, error) {
	if source == nil {
		return nil, &ServiceDefinitionError{Code: ErrDefinitionValidationFailed, Message: "source is nil"}
	}
	if provider == nil {
		return nil, &ServiceDefinitionError{Code: ErrDefinitionValidationFailed, Message: "provider is nil"}
	}
	if mapper == nil {
		mapper = NewDefinitionMapper()
	}
	return &DefinitionSyncService{
		source:         source,
		provider:       provider,
		mapper:         mapper,
		extensionLocks: make(map[string]*sync.Mutex),
	}, nil
}

func (s *DefinitionSyncService) FullSync() *FullSyncReport {
	report := &FullSyncReport{
		DefinitionErrors: make(map[string]error),
	}

	extIDs, err := s.source.GetExtensionIDs()
	if err != nil {
		report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionSourceMismatch,
			"failed to get extension ids from source", err))
		return report
	}

	report.TotalExtensions = len(extIDs)

	desired := make(map[string]*trusted_service.ServiceRuntimeDefinition)
	owned := make(map[string]ownedDefinition)

	for _, extID := range extIDs {
		views, err := s.source.GetServiceViewsByExtension(extID)
		if err != nil {
			report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionSourceMismatch,
				"failed to get service views for extension: "+extID, err))
			continue
		}

		for _, view := range views {
			if !view.Enabled {
				continue
			}

			if err := validateView(view); err != nil {
				report.FailedDefinitions++
				report.DefinitionErrors[view.ToDefinitionID()] = err
				continue
			}

			def, err := s.mapper.MapToDefinition(view)
			if err != nil {
				report.FailedDefinitions++
				report.DefinitionErrors[view.ToDefinitionID()] = err
				continue
			}

			if existing, exists := desired[def.ServiceID]; exists {
				existingExt := owned[def.ServiceID]
				conflict := NewServiceDefinitionErrorWithCause(ErrDefinitionConflict,
					"definition id conflict",
					NewServiceDefinitionError(ErrDefinitionConflict,
						def.ServiceID+" exists in "+existingExt.ExtensionID+"/"+existingExt.ModuleID+" and "+view.ExtensionID+"/"+view.ModuleID))
				report.Errors = append(report.Errors, conflict)
				_ = existing
				continue
			}

			desired[def.ServiceID] = def
			owned[def.ServiceID] = ownedDefinition{
				DefinitionID: def.ServiceID,
				ExtensionID:  view.ExtensionID,
				ModuleID:     view.ModuleID,
			}
		}
	}

	existing := s.provider.ListAll()
	existingIDs := make(map[string]bool)
	for _, def := range existing {
		existingIDs[def.ServiceID] = true
	}

	for id, def := range desired {
		if _, exists := existingIDs[id]; exists {
			if err := s.provider.Register(def); err != nil {
				report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionRegisterFailed,
					"failed to update definition: "+id, err))
			} else {
				report.SyncedDefinitions++
			}
		} else {
			if err := s.provider.Register(def); err != nil {
				report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionRegisterFailed,
					"failed to register definition: "+id, err))
			} else {
				report.SyncedDefinitions++
			}
		}
	}

	return report
}

func (s *DefinitionSyncService) ReconcileExtension(extensionID string) *ReconcileReport {
	lock := s.getExtensionLock(extensionID)
	lock.Lock()
	defer lock.Unlock()

	report := &ReconcileReport{
		ExtensionID:      extensionID,
		DefinitionErrors: make(map[string]error),
	}

	views, err := s.source.GetServiceViewsByExtension(extensionID)
	if err != nil {
		report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionSourceMismatch,
			"failed to get service views for extension: "+extensionID, err))
		return report
	}

	var desired []ServiceRuntimeView
	for _, view := range views {
		if !view.Enabled {
			report.Skipped++
			continue
		}
		if err := validateView(view); err != nil {
			report.Skipped++
			report.DefinitionErrors[view.ToDefinitionID()] = err
			continue
		}
		desired = append(desired, view)
	}

	existing := s.provider.ListByExtension(extensionID)
	existingIDs := make(map[string]bool)
	for _, def := range existing {
		existingIDs[def.ServiceID] = false
	}

	for _, view := range desired {
		def, err := s.mapper.MapToDefinition(view)
		if err != nil {
			report.Errors = append(report.Errors, err)
			continue
		}

		if _, exists := existingIDs[def.ServiceID]; exists {
			if err := s.provider.Register(def); err != nil {
				report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionRegisterFailed,
					"failed to update definition: "+def.ServiceID, err))
			} else {
				report.Updated++
			}
			existingIDs[def.ServiceID] = true
		} else {
			if err := s.provider.Register(def); err != nil {
				report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionRegisterFailed,
					"failed to register definition: "+def.ServiceID, err))
			} else {
				report.Added++
			}
		}
	}

	for serviceID, processed := range existingIDs {
		if !processed {
			if err := s.provider.Remove(serviceID); err != nil {
				report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionRemoveFailed,
					"failed to remove definition: "+serviceID, err))
			} else {
				report.Removed++
			}
		}
	}

	return report
}

func (s *DefinitionSyncService) RemoveExtension(extensionID string) *RemoveReport {
	lock := s.getExtensionLock(extensionID)
	lock.Lock()
	defer lock.Unlock()

	report := &RemoveReport{
		ExtensionID: extensionID,
	}

	existing := s.provider.ListByExtension(extensionID)

	for _, def := range existing {
		if err := s.provider.Remove(def.ServiceID); err != nil {
			report.Errors = append(report.Errors, NewServiceDefinitionErrorWithCause(ErrDefinitionRemoveFailed,
				"failed to remove definition: "+def.ServiceID, err))
		} else {
			report.RemovedCount++
		}
	}

	return report
}

func (s *DefinitionSyncService) getExtensionLock(extensionID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lock, exists := s.extensionLocks[extensionID]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	s.extensionLocks[extensionID] = lock
	return lock
}

func DefinitionStub(view ServiceRuntimeView) (*trusted_service.ServiceRuntimeDefinition, error) {
	if err := validateView(view); err != nil {
		return nil, err
	}
	mapper := NewDefinitionMapper()
	return mapper.MapToDefinition(view)
}

func validateView(view ServiceRuntimeView) error {
	if view.ExtensionID == "" {
		return &ServiceDefinitionError{Code: ErrDefinitionMappingFailed, Message: "extension id must not be empty"}
	}
	if view.ModuleID == "" {
		return &ServiceDefinitionError{Code: ErrDefinitionMappingFailed, Message: "module id must not be empty"}
	}
	if !IsValidServiceRuntimeType(view.RuntimeType) {
		return &ServiceDefinitionError{Code: ErrUnsupportedServiceKind, Message: "unsupported service runtime type: " + view.RuntimeType}
	}
	if view.EntryPoint == "" {
		return &ServiceDefinitionError{Code: ErrDefinitionMappingFailed, Message: "entry point must not be empty for process service"}
	}
	return nil
}
