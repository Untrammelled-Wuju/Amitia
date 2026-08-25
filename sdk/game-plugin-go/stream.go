package sdk

// Stream RPC helpers are intentionally not part of GameHost protocol v1.
// Plugin-to-host high-frequency data uses declared channels. Host-to-plugin
// command/data delivery uses ordinary plugin RPC until a bidirectional stream
// protocol is implemented and negotiated in a future protocol major.
