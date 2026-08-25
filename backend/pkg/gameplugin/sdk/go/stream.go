package sdk

// Stream RPC helpers are intentionally not part of GameHost protocol v1.
// High-frequency message delivery uses declared channels in plugin_to_host,
// host_to_plugin, or bidirectional direction. Dedicated stream RPC lifecycle
// helpers remain outside protocol v1.
