# Amitia Game Plugin SDK for TypeScript

Public TypeScript SDK for `amitia-game-host/1`.

The package is intended for independent third-party game plugins and exposes only public GameHost wire contracts and helpers. A plugin must not import Amitia backend `internal/` packages.

## Public contract layers

Two service-shaped contracts exist intentionally and are not interchangeable:

- `PluginServiceSpec` is the install-time entry in `PluginHostSpec.services[]`. It binds a logical service ID to a packaged runtime module through `moduleId`.
- `ServiceDescriptor` is the runtime/handshake descriptor used after a process is running. It has runtime capabilities but no `moduleId`.

The canonical install contribution is `PluginHostSpec`. Validate it before packaging:

```ts
import {
  PROTOCOL_VERSION,
  PluginHostSpec,
  assertPluginHostSpec,
  validatePluginHostSpec,
} from '@amitia/game-plugin-sdk';

const spec: PluginHostSpec = {
  protocolVersion: PROTOCOL_VERSION,
  runtimeModuleId: 'runtime-main',
  hostFeatures: ['custom_rpc'],
  network: { mode: 'none' },
};

const errors = validatePluginHostSpec(spec);
if (errors.length > 0) {
  throw new Error(errors.join('; '));
}
assertPluginHostSpec(spec);
```

`validatePluginHostSpec` follows the same semantic rules as the Go host validator, including service dependency topology, service-scoped channel uniqueness, feature requirements, network policy allowlists, companion artifact paths, and compatibility constraints.

## Host-mediated game networking

For portable access to a game, companion, RCON bridge, telemetry endpoint, or other local service, use `network.mode: 'restricted'` with `service.network.request`. The plugin process remains network-isolated; the host owns the real sockets and enforces the declared transport, destination, and port policy.

`allowedTransports` may contain `http`, `https`, `tcp`, `udp`, or `websocket`. Set `allowHostLoopback: true` to allow the portable target `host-loopback`, which always means the host machine rather than the plugin sandbox/network namespace. `maxConnections` bounds simultaneous host-owned socket handles for one runtime service/session; omission or `0` uses the host default.

The SDK exports `networkRequest`, TCP open/read/write/close, UDP open/receive/send/close, and WebSocket open/receive/send/close helpers. Socket handles are generation/session-bound and are invalidated when the owning service stops, crashes, restarts, or the host shuts down. Long-lived connections are not terminated merely for being idle.

## Descriptor helpers

`createPluginDescriptor` builds the runtime/handshake descriptor and validates `ServiceDescriptor`, `ChannelDescriptor`, and host features before returning it. Invalid IDs, control characters, unknown host features, duplicate channels, oversized channel IDs, and oversized schema IDs are rejected locally.

## Contract parity

The Go protocol, JSON Schemas, and TypeScript public types are checked by `scripts/verify_game_plugin_contract_parity.py`. Go and TypeScript also execute the same `host_spec_validation_cases.json` fixture so validator acceptance/rejection behavior cannot drift silently.
