# Mock AmitiaX Game Plugin

An independent mock game plugin for AmitiaX, built using only the formal public SDK.

This project demonstrates that third-party developers can build AmitiaX game plugins without access to the Amitia internal source code.

## Project Structure

```
mock-amitiax-game-plugin/
├── go/                          # Go implementation
│   ├── go.mod
│   ├── cmd/mock-game-plugin/    # Entry point
│   ├── manifest.json            # Manifest V1
│   └── README.md
├── typescript/                  # TypeScript implementation
│   ├── package.json
│   ├── tsconfig.json
│   ├── src/
│   ├── manifest.json            # Manifest V1
│   └── README.md
└── README.md                    # This file
```

## Design Principles

This project follows the G34 specification:

- **Independent Build**: Each implementation builds without the Amitia backend source
- **No Internal Imports**: Only depends on the formal public SDK (`@amitia/game-plugin-sdk`)
- **Public SDK Only**: Uses only exported APIs from the SDK
- **Same Protocol**: Both implementations use `amitia-game-host/1`

## Prerequisites

### Go
- Go 1.26.1 or later
- The Amitia Go Game Plugin SDK

### TypeScript
- Node.js 18 or later
- The `@amitia/game-plugin-sdk` npm package

## Build

### Go

```bash
cd go
go mod tidy
go build -o mock-game-plugin ./cmd/mock-game-plugin
```

### TypeScript

```bash
cd typescript
npm install
npm run build
```

## Run

### Go

```bash
./mock-game-plugin
```

### TypeScript

```bash
npm start
```

## Protocol

Both implementations:

1. Connect to GameHost via stdio (length-prefixed JSON frames)
2. Perform handshake via `control.handshake.hello`
3. Register RPC handlers:
   - `mock.echo` - Echoes back the message with a counter
   - `mock.increment` - Increments an internal counter
4. Send `mock.ready` notification on startup
5. Send `mock.shutdown` notification on graceful shutdown

## Conformance

This plugin is designed to pass G34 conformance tests:

- Clean build without Amitia internal source
- No `backend/internal` imports
- No `internal/gamehost` imports
- No `internal/extension/kernel` imports
- No direct GameHost registry access
- No direct database access
- No host endpoint hard-coding

## License

MIT
