# Amitia Game Plugin SDK for Go

Public, standalone Go SDK for `amitia-game-host/1`.

This module intentionally has no dependency on the Amitia backend module or any
`internal/` package. Game plugins may depend on this module alone and use:

- `github.com/u-ai/game-plugin-sdk-go` for the SDK client/runner/helpers;
- `github.com/u-ai/game-plugin-sdk-go/protocol` for public wire contracts;
- `github.com/u-ai/game-plugin-sdk-go/protocol/contracts` for shared control contracts.

The source is synchronized from the canonical backend protocol/SDK by
`scripts/sync_game_plugin_go_sdk.py`; CI rejects drift.
