package display

type TopologyAdapter struct {
	supported bool
}

func NewTopologyAdapter(supported bool) *TopologyAdapter {
	return &TopologyAdapter{supported: supported}
}

func (a *TopologyAdapter) IsSupported() bool {
	return a.supported
}

func (a *TopologyAdapter) EmptyTopology() *DisplayTopologyPosition {
	return &DisplayTopologyPosition{
		Available: false,
	}
}
