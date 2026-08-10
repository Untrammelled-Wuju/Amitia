package resource

type RuntimeIdentityReader interface {
	ResolveRuntime(runtimeID string) (pluginID string, extensionID string, state string, err error)
	ResolveService(runtimeID, serviceID string) (pluginID string, extensionID string, state string, err error)
	ExtensionEnabled(extensionID string) bool
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

	if runtimeID == "" {
		return subj, ErrSubjectInvalid
	}
	if pluginID == "" {
		return subj, ErrSubjectInvalid
	}
	if serviceID == "" {
		return subj, ErrSubjectInvalid
	}

	if m.reader == nil {
		return subj, nil
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
	if svcPluginID != pluginID {
		return subj, ErrSubjectInvalid
	}
	if svcExtID != extID {
		return subj, ErrSubjectInvalid
	}

	if !m.reader.ExtensionEnabled(extID) {
		return subj, ErrExtensionDisabled
	}

	return subj, nil
}
