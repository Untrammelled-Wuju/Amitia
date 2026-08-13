package nativebridge

type HostGeneration uint64

const HostGenerationZero HostGeneration = 0

type GenerationStatus struct {
	Generation HostGeneration `json:"generation"`
	Valid      bool           `json:"valid"`
}

func (g HostGeneration) IsZero() bool {
	return g == HostGenerationZero
}
