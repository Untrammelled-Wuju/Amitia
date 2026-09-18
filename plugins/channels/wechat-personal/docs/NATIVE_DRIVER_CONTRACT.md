# Personal WeChat Native Driver Contract v1

The extension consumes the public Amitia Native Companion v1 contract; its WeChat-specific driver protocol stays entirely inside the extension/native agent boundary.

## Native Companion discovery

Trusted Service receives:

```text
AMITIA_NATIVE_COMPANIONS_VERSION=1
AMITIA_NATIVE_COMPANIONS=[...]
```

Each entry is already filtered for the current platform/architecture and contains a canonical path plus verified SHA-256. The service must not infer undeclared native package paths.


## Trusted Service loopback authentication

The plugin provider service consumes the generic Trusted Service Loopback Authentication v1 contract:

```text
AMITIA_SERVICE_AUTH_VERSION=1
AMITIA_SERVICE_AUTH_TOKEN=<host-derived module token>
AMITIA_CORE_URL=http://127.0.0.1:<host-port>
```

Every HTTP route exposed by the provider service requires `Authorization: Bearer <token>`. The host's generic Channel HTTP Provider attaches this header automatically. The token is process-scoped, is not persisted, and must never be exposed to UI code. Wildcard CORS is not permitted on the authenticated loopback service.

Inbound messages posted to `/api/channels/inbound` also carry:

```http
X-Amitia-Extension-ID: com.amitia/channel-wechat-personal
X-Amitia-Module-ID: wechat-personal-channel-service
```

The host validates the token against that exact extension/module pair and verifies the declared `channelId`.

## Agent RPC

Agent stdin/stdout is one JSON object per line.

Requests:

```json
{"id":"1","op":"probe"}
{"id":"2","op":"start","driverLibraryPath":"..."}
{"id":"3","op":"hide"}
{"id":"4","op":"driver.probe"}
{"id":"5","op":"driver.call","driverOp":"login.qr","payload":{}}
```

The response reports the official-client state and, where relevant, the selected platform Driver.

## Unified Driver operations

Both Windows and Linux implement the same upper contract:

- `driver.capabilities`
- `login.qr`
- `login.status`
- `account.self`
- `events.poll`
- `messages.send_text`

A version-sensitive Driver must also report the exact official client version it targets. The Agent compares that value with the version independently read from the official client and exposes `versionVerified=true` only on an exact match. Message capabilities are unusable unless that verification succeeds.

Capabilities are explicit booleans:

```json
{
  "attached": true,
  "clientVersion": "4.1.x.x",
  "versionVerified": true,
  "qr": true,
  "loginStatus": true,
  "selfProfile": false,
  "receiveText": false,
  "sendText": false
}
```

The service must never infer message support from login support.

## Windows transport

Preferred full Driver:

```text
\\.\pipe\amitia-wechat-<pid>
```

JSON-line requests use the same Driver operation names above. If no verified Hook answers, the Agent can expose a Win32 UI Driver for `login.qr` and `login.status` only.

## Linux transport

Preferred full Driver:

```text
/tmp/amitia-wechat-hook-<pid>.sock
```

The bundled Linux Driver Library is loaded through `LD_PRELOAD` and establishes the transport but keeps unverified internal WeChat capabilities false. A version-specific adapter may later implement the same operations.

If no verified preload adapter is usable, an X11/XWayland UI Driver may provide `login.qr` / UI-derived `login.status`. Pure Wayland can legitimately expose no UI fallback.

## Message event format

A verified Driver's `events.poll` should return zero or more normalized events:

```json
{
  "events": [
    {
      "messageId": "...",
      "accountId": "wxid_ai",
      "peerId": "wxid_peer_or_chatroom",
      "senderId": "wxid_sender",
      "type": 1,
      "text": "hello",
      "createdAt": 0
    }
  ]
}
```

For groups, `peerId` is the `@chatroom` conversation and `senderId` is the actual member.

## Security requirements

- local named-pipe / Unix-domain IPC only for internal Hook transports;
- unknown client versions fail closed for internal calls;
- Native Companion hashes are checked by package validation and again during Trusted Service registration;
- no runtime download of Hook binaries;
- no disguised extraction of undeclared native binaries;
- no WeChat-specific host capability;
- no modification of the existing OpenClaw/iLink plugin.

## Development compatibility

Legacy hero/aixed HTTP adapters remain available only when:

```text
AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT=1
```

is explicitly set. They are not part of the default production route. The legacy unauthenticated `9999/message` receiver is even more restricted and is created only when the separate development-only flag is also set:

```text
AMITIA_WECHAT_EXTERNAL_CALLBACK_UNAUTHENTICATED=1
```

Production packages must leave that second flag unset.
