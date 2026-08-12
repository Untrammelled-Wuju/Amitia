package deepsearch

type Config struct {
	Enabled            bool
	Policy             DeepSearchPolicy
	GeneralSearchToolID string
}

func DefaultConfig() DeepSearchPolicy {
	return DefaultDeepSearchPolicy()
}
