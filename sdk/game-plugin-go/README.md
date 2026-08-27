# Amitia Game Plugin SDK for Go

Public, standalone Go SDK for `amitia-game-host/1`.

This module intentionally has no dependency on the Amitia backend module or any
`internal/` package. Game plugins may depend on this module alone and use:

- `github.com/u-ai/game-plugin-sdk-go` for the SDK client/runner/helpers;
- `github.com/u-ai/game-plugin-sdk-go/protocol` for public wire contracts;
- `github.com/u-ai/game-plugin-sdk-go/protocol/contracts` for shared control contracts.

The source is synchronized from the canonical backend protocol/SDK by
`scripts/sync_game_plugin_go_sdk.py`; CI rejects drift.

## Host-mediated game networking

Use a `restricted` plugin network policy plus `service.network.request` for portable
game/companion communication. The plugin process stays network-isolated while
GameHost owns and policy-checks HTTP(S), TCP, UDP, and WebSocket connections.
`allowHostLoopback: true` enables the portable `host-loopback` target, so plugins
never need platform-specific host-network addresses. The root SDK exposes
`NetworkRequest`, `NetworkTCPOpen/Read/Write/Close`,
`NetworkUDPOpen/Receive/Send/Close`, and
`NetworkWebSocketOpen/Receive/Send/Close`. Handles are bound to the runtime
generation/session and are released on service stop/crash/restart or host shutdown.
