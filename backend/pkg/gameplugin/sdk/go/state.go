package sdk

// Durable plugin state in protocol v1 is published through a declared state
// channel. plugin.state.publish/plugin.state.get are intentionally not public
// Host RPCs so the SDK exposes a single state model.
