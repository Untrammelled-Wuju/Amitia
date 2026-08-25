package protocol

const (
	ProtocolName    = "amitia-game-host"
	ProtocolMajor   = 1
	ProtocolVersion = "amitia-game-host/1"
)

// Host features are frozen per protocol major. Adding a new standard host
// feature requires a protocol-major bump; unknown/namespaced values are never
// opportunistically accepted by an older host. This keeps feature negotiation
// deterministic across SDKs and prevents a plugin-defined capability from
// accidentally becoming a host capability.
func HostFeatureIntroducedInMajor(feature HostFeature) (int, bool) {
	if !IsKnownHostFeature(feature) {
		return 0, false
	}
	return 1, true
}

func HostFeatureSupportedByCurrentProtocol(feature HostFeature) bool {
	major, ok := HostFeatureIntroducedInMajor(feature)
	return ok && major <= ProtocolMajor
}
