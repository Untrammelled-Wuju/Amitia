# Mock AmitiaX Game Plugin (G46-F15)

An independent external Mock Game Plugin that proves third-party developers can build AmitiaX game plugins using only the Public SDK.

## Purpose

This plugin serves as the G46-F15 external consumer proof. It demonstrates that a project which:

- Has its own independent `package.json`
- Has its own independent dependency graph
- Builds without access to Amitia internal source
- Only depends on the `@amitia/game-plugin-sdk`
- Never imports `backend/internal/*`

Can successfully:
- Build
- Package
- Install (via Game Center)
- Start
- Perform Handshake
- Reach Ready
- Execute Custom RPC
- Publish Events
- Use Binary References
- Operate Streams
- Exercise Permissions
- Use SecretLease
- Invoke Host API
- Perform Control operations (Takeover/Release/EStop)
- Handle Crash Recovery
- Survive Restarts
- Upgrade
- Uninstall with zero residue

## Public SDK Dependency

```json
{
  "dependencies": {
    "@amitia/game-plugin-sdk": "file:vendor/amitia-game-plugin-sdk-0.1.0.tgz"
  }
}
```

No internal Amitia packages are referenced.

## Project Structure

```
mock-amitiax-game-plugin/
├── package.json              # Independent project manifest
├── tsconfig.json             # TypeScript configuration
├── amitia-extension.json     # Manifest V1 (target: gamex)
├── fault-matrix.yaml         # G37 Fault Matrix definition
├── src/
│   ├── index.ts              # Entry point
│   ├── plugin.ts             # Main Plugin class
│   ├── state.ts              # Fake Game Runtime state
│   ├── rpc.ts                # Custom RPC handlers
│   ├── control.ts            # Control authority handlers
│   ├── hostapi.ts            # Host API / Permission / Secret handlers
│   └── fault.ts              # Fault injection handlers
├── scripts/
│   ├── build-package.mjs     # Build and package script
│   └── verify-package.mjs    # Package verification script
├── fixtures/
│   ├── wrong-protocol.js     # Protocol mismatch test fixture
│   ├── malformed-frame.js    # Malformed frame test fixture
│   └── oversized-frame.js    # Oversized frame test fixture
└── README.md                 # This file
```

## Build

```bash
npm install
npm run build
```

## Package

```bash
npm run package
```

This produces the canonical Extension Package `dist-package/world-game-plugin.amitiax`.

The archive uses the real Manifest v1 package layout consumed by Extension Kernel:

```text
manifest.json
integrity/files.json
integrity/content-tree.json
modules/mock-game-runtime/dist/...
modules/mock-game-runtime/node_modules/@amitia/game-plugin-sdk/...
```

`amitia-extension.json` remains the editable development-workspace manifest; the package builder emits it as canonical `manifest.json` and generates the integrity documents.

For the update E2E matrix, the same source can emit a second package version without changing the repository manifest:

```bash
MOCK_PLUGIN_VERSION=1.1.0 npm run package
```

## Verify Package

```bash
npm run verify-package
```

Checks:
- Canonical `manifest.json` exists and is Manifest v1
- Canonical runtime entrypoint exists at `modules/mock-game-runtime/dist/index.js`
- Only allowed package roots are emitted
- `integrity/files.json` exactly covers the payload
- `integrity/content-tree.json` matches the canonical tree hash
- Only the explicitly whitelisted public SDK runtime dependency is packaged

## Run

```bash
npm start
```

The plugin connects to GameHost via stdio (length-prefixed JSON frames) and performs the full G34-G37 lifecycle.

## Protocol

- Protocol: `amitia-game-host/1`
- Transport: stdio (length-prefixed JSON frames)
- Handshake: `control.handshake.hello`

## Custom RPC Namespaces

All under `mockgame.*` (never occupying reserved namespaces):

- `mockgame.echo` - Echo with counter
- `mockgame.status` - Full state snapshot
- `mockgame.command` - Game command sink (start/stop/damage/heal/move/reset)
- `mockgame.long_task` - Long-running task for timeout testing
- `mockgame.fail` - Intentional failure for error testing
- `mockgame.binary.consume` - Binary reference consumer
- `mockgame.fault.*` - Fault injection methods

## Security

- `gamehost.control` - Control authority operations
- `gamehost.channel.use` - Channel messaging
- `gamehost.host_api.invoke` - Host API invocation
- `service.secret.use` - SecretLease acquisition
- Network mode is `restricted`: the plugin process receives no ambient network access. HTTP uses host-mediated `host.network.request`, while TCP/UDP/WebSocket use `host.network.*` handles. The E2E starts an independent generic-game fixture on host loopback and proves the portable `host-loopback` target can reach it without sharing the host network namespace; undeclared targets and ports are rejected.

## Services

- `mock-game-runtime` - The fixture's single process service. Multi-service composition is tested by the host contract/unit suites; this external package focuses on the complete third-party lifecycle through one independent runtime.

## Control Effect Sink

- `mockgame.effect` - Opaque effect sink (Host enforces G9 gate)

## Conformance Coverage

| Group | Coverage |
|-------|----------|
| G34 | Build, Package, Install, Start, Handshake, Ready, RPC, Event |
| G35 | Custom RPC, Event, Binary Reference, Stream, Reconnect, State Snapshot |
| G36 | Permission, SecretLease, Host API, Control, Takeover, EStop |
| G37 | 30-case Fault Matrix (Crash, Protocol, RPC, Security, Resource, Lifecycle, Recovery) |

## No Internal Dependency Guarantee

This project:

- Does NOT import `backend/internal`
- Does NOT import `internal/gamehost`
- Does NOT import `internal/extension/kernel`
- Does NOT access Host concrete types
- Does NOT bypass Permission/SecretLease/G9
- Does NOT use any private Amitia APIs

## License

MIT
