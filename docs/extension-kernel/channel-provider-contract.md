# Channel Provider Contract

The host does not implement platform-specific channel behavior. It only provides the shared message pipeline, delivery queue, provider registry, trusted-service authentication, and generic channel commands.

Every concrete channel is implemented by an extension that declares the `channel.provider` capability and owns its platform connection, login flow, inbound events, outbound send calls, message history, and platform-specific error handling. Telegram, Feishu, WeChat, QQ, or any other platform must use the same contract without adding host-side branches.

## Manifest

```json
{
  "modules": [
    {
      "id": "telegram-channel-service",
      "type": "service",
      "runtime": {
        "type": "service",
        "entryPoint": "launcher.mjs"
      },
      "providedCapabilities": [
        {
          "id": "channel.provider",
          "version": "1.0.0"
        }
      ],
      "provider": {
        "id": "com.example.channel-telegram.provider",
        "priority": 100,
        "labels": {
          "channelId": "telegram"
        },
        "metadata": {
          "channelId": "telegram",
          "capabilities": {
            "text": true,
            "image": true,
            "video": false,
            "voice": true,
            "file": true,
            "group": true,
            "thread": true,
            "reply": true
          },
          "transport": {
            "type": "http",
            "host": "127.0.0.1",
            "defaultPort": 19880,
            "healthPath": "/api/health",
            "statusPath": "/api/status",
            "configPath": "/api/config",
            "connectPath": "/api/connect",
            "disconnectPath": "/api/disconnect",
            "messagesPath": "/api/messages",
            "sendPath": "/api/send",
            "imagePath": "/api/send-image",
            "voicePath": "/api/send-voice"
          }
        }
      }
    }
  ]
}
```

`transport.type` currently supports `http`. The endpoint must bind to loopback. `baseUrl` may be used instead of `host` and `defaultPort`. `portEnv` may be used when the plugin service allocates a dynamic port and exports it to the host process environment.

## Authentication

Trusted service extensions receive:

```text
AMITIA_SERVICE_AUTH_VERSION=1
AMITIA_SERVICE_AUTH_TOKEN=<process-scoped token>
AMITIA_CORE_URL=http://127.0.0.1:<host port>
```

The host sends this header to the plugin transport:

```http
Authorization: Bearer <AMITIA_SERVICE_AUTH_TOKEN>
X-Amitia-Extension-ID: <extension ID>
X-Amitia-Module-ID: <service module ID>
```

Plugin inbound requests to the host use the same three headers. The host validates the token against the exact extension and module pair, then verifies that the same module declares a matching `channel.provider` and `channelId`.

## Inbound

Plugin endpoint:

```http
POST /api/channels/inbound
Authorization: Bearer <AMITIA_SERVICE_AUTH_TOKEN>
X-Amitia-Extension-ID: com.example/channel-telegram
X-Amitia-Module-ID: telegram-channel-service
Content-Type: application/json
```

Text message:

```json
{
  "channelId": "telegram",
  "accountId": "bot-1",
  "conversationId": "chat-42",
  "peerId": "user-7",
  "messageId": "message-1001",
  "contentType": "text",
  "text": "hello",
  "replyToMessageId": "message-1000",
  "spaceId": "",
  "characterId": ""
}
```

Media message:

```json
{
  "channelId": "feishu",
  "accountId": "app-1",
  "conversationId": "oc_xxx",
  "peerId": "ou_xxx",
  "messageId": "om_xxx",
  "contentType": "audio",
  "audioUrl": "https://example.invalid/voice.ogg",
  "audioDuration": 3.2
}
```

Required fields:

- `channelId`
- `peerId`
- content appropriate to `contentType`

`contentType` supports `text`, `image`, `video`, `audio`, and `voice`. The default is `text`. `conversationId` should be stable for the platform conversation. If it is omitted, the host derives a stable ID from channel, account, and peer.

`messageId` is the idempotency key. The host scopes it by extension, module, and channel before submitting it to the unified interaction pipeline.

## Outbound

The host calls the plugin transport configured by `transport`:

```http
GET /api/status
GET /api/config
POST /api/connect
POST /api/disconnect
POST /api/messages
POST /api/send
POST /api/send-image
POST /api/send-voice
```

`POST /api/send` receives:

```json
{
  "toUserId": "user-7",
  "conversationId": "chat-42",
  "text": "hello",
  "contextToken": "",
  "deliveryKey": "delivery-intent-id"
}
```

The host also sends `Idempotency-Key: <delivery-intent-id>`. The plugin must treat a repeated key as the same delivery and return `accepted: true` with `duplicate: true` when appropriate.

Image delivery uses `assetUrl` and `fallbackUrl`. Voice delivery uses the configured voice path. Plugins that do not support a declared media path must fail explicitly instead of silently dropping the message.

## Status

The status endpoint returns either a direct object or an object under `data`:

```json
{
  "data": {
    "accountId": "bot-1",
    "connected": true,
    "status": "connected",
    "running": true,
    "lastError": ""
  }
}
```

The host does not infer platform-specific online fields. Plugins must set `connected` or the standard `connected` or `online` status value.

Channel status is owned by the plugin service. Host web and mobile shells must not poll `channel.status` on a timer. A plugin page may request status when it opens or after an explicit user action, but ongoing connection, login, retry, and transport state transitions remain inside the plugin runtime.

## Host Commands

Extensions use the public host commands:

- `channel.status`
- `channel.connect`
- `channel.disconnect`
- `channel.messages`

The commands take `channelId` and resolve the provider dynamically from installed extension manifests. No channel-specific host command or host route is required.

## Delivery And Pipeline

The plugin is responsible for receiving platform events and posting them to `/api/channels/inbound`.

The host is responsible for:

- extension and module identity validation
- provider declaration validation
- scope resolution and default character selection
- unified interaction orchestration
- message persistence
- delivery queue scheduling and retry
- routing outbound delivery back to the plugin transport

The host must not contain Telegram, Feishu, QQ, WeChat, or other platform-specific branches.
