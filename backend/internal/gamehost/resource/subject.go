package resource

import "fmt"

type RuntimeIdentityReader interface {
	ResolveRuntime(runtimeID string) (pluginID string, extensionID string, state string, err error)
	ResolveService(runtimeID, serviceID string) (pluginID string, extensionID string, state string, err error)
	CurrentGeneration(runtimeID string) (int64, error)
	ExtensionEnabled(extensionID string) bool
	RuntimeIDsByExtension(extensionID string) []string
}

type SubjectMapper struct {
	reader RuntimeIdentityReader
}

func NewSubjectMapper(reader RuntimeIdentityReader) *SubjectMapper {
	return &SubjectMapper{reader: reader}
}

func (m *SubjectMapper) Validate(runtimeID, pluginID, serviceID string, generation int64) (RuntimeIdentitySubject, error) {
	subj := RuntimeIdentitySubject{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		ServiceID:  serviceID,
		Generation: generation,
	}

	if runtimeID == "" || pluginID == "" || serviceID == "" || generation <= 0 {
		return subj, ErrSubjectInvalid
	}
	// Resource admission is a security boundary. Missing identity infrastructure
	// must fail closed rather than accepting an unverifiable subject.
	if m == nil || m.reader == nil {
		return subj, ErrSubjectInvalid
	}

	pluginOK, extID, _, err := m.reader.ResolveRuntime(runtimeID)
	if err != nil {
		return subj, ErrRuntimeNotFound
	}
	if pluginOK != pluginID {
		return subj, ErrSubjectInvalid
	}
	subj.ExtensionID = extID

	svcPluginID, svcExtID, _, err := m.reader.ResolveService(runtimeID, serviceID)
	if err != nil {
		return subj, ErrServiceNotFound
	}
	if svcPluginID != pluginID || svcExtID != extID {
		return subj, ErrSubjectInvalid
	}

	currentGeneration, err := m.reader.CurrentGeneration(runtimeID)
	if err != nil {
		return subj, ErrRuntimeNotFound
	}
	if currentGeneration <= 0 || currentGeneration != generation {
		return subj, fmt.Errorf("%w: runtime generation expected=%d got=%d", ErrSubjectInvalid, currentGeneration, generation)
	}

	if !m.reader.ExtensionEnabled(extID) {
		return subj, ErrExtensionDisabled
	}

	return subj, nil
}

func (m *SubjectMapper) RuntimeIDsByExtension(extensionID string) []string {
	if m == nil || m.reader == nil || extensionID == "" {
		return nil
	}
	return m.reader.RuntimeIDsByExtension(extensionID)
}
