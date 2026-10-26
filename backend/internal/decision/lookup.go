package decision

func IsKnownCandidateID(id string) bool {
	if id == "" {
		return false
	}
	_, ok := DefaultCandidateRegistry().Get(id)
	return ok
}

func IsKnownPreferenceDimension(dimension string) bool {
	if dimension == "" {
		return false
	}
	registry := DefaultCandidateRegistry()
	if _, ok := registry.Get(dimension); ok {
		return true
	}
	for _, definition := range registry.All() {
		if string(definition.Tag) == dimension {
			return true
		}
	}
	return false
}
