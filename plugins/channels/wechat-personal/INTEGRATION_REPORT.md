# Integration Report — 1.5.0

## Completed in source

- Added `wechat_personal` as a separate channel/provider declared by its own extension manifest. Existing `plugins/channels/wechat` remains independent.
- Preserved the host's existing space-default-character routing.
- Added the generic `/api/channels/inbound` contract with trusted-service identity headers, `channelId`, and provider declaration validation.
- Added public Manifest v1 `runtime.nativeCompanions` for trusted service extensions. This is a generic Extension Kernel capability and contains no WeChat-specific fields.
- Added generic Native Companion v1 runtime descriptors, current-platform/current-architecture filtering, canonical-path resolution and SHA-256 re-verification before Trusted Service registration.
- Package security only permits native files at exact manifest-declared paths. Undeclared `.exe/.dll/.so` remain rejected.
- Trusted Service injects `AMITIA_NATIVE_COMPANIONS_VERSION=1` and `AMITIA_NATIVE_COMPANIONS`; the plugin rejects unsupported non-empty versions.
- Added Windows x64 and Linux x64 Native Agent builds using one JSON-lines RPC contract.
- Windows Native Agent manages official Weixin, detects version, captures the official login UI through Win32, and provides a QR/login-only UI fallback when no verified Hook is attached.
- Linux Native Agent manages official Linux WeChat, supports versioned `LD_PRELOAD` transport, and provides an X11/XWayland login UI fallback when the desktop permits cross-window access.
- Windows uses a named-pipe Hook contract; Linux uses a Unix-domain-socket preload contract. Both expose the same `driver.capabilities`, `login.qr`, `login.status`, `account.self`, `events.poll`, and `messages.send_text` operations to the service.
- Service treats QR/login capability separately from message capability. Login-only drivers become `connected_limited`; text delivery is not reported ready unless `receiveText && sendText` are verified.
- hero/aixed HTTP adapters remain development migration adapters and are disabled unless `AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT=1` is explicitly set.
- UI now surfaces individual QR/login/receive/send capability state.


## 1.5.0 security hardening

- Added public Trusted Service Loopback Authentication v1. The host derives a process-lifetime token scoped to `extensionId + moduleId`; the same token is injected into the Trusted Service and attached by the generic Channel HTTP Provider.
- `wechat-personal` now requires `Authorization: Bearer <token>` for every `19878` API route. Missing/wrong credentials return 401 before routing.
- Removed wildcard CORS from the provider response path.
- The legacy unauthenticated hero `9999/message` receiver is no longer started by default. It is created only when both `AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT=1` and `AMITIA_WECHAT_EXTERNAL_CALLBACK_UNAUTHENTICATED=1` are set for explicit development migration.
- Added a dedicated security E2E covering unauthorized access, wrong credentials, no wildcard CORS, closed legacy callback port, and rejection of forged unauthenticated native callbacks.
- Release preflight runs security, Native Companion, version-gate, and compatibility E2E tests before packaging.

## Runtime validation performed

- Native Agent source compiles/tests for Linux amd64.
- Native Agent source cross-compiles/tests for Windows amd64 with CGO disabled.
- Node service syntax validation passes.
- The service mock end-to-end test for inbound normalization and outbound delivery passes.
- Linux preload companion builds and exposes the common JSON-line driver transport while intentionally advertising message capabilities false until a verified version adapter exists.

## Fail-closed boundary

The current self-contained package can manage official clients and provide login UI on supported desktop environments. It does **not** claim a fully verified text Hook for every current Weixin/WeChat build.

A platform Driver may advertise `receiveText` or `sendText` only after that exact implementation is verified for the running client version. Unknown versions remain login-only or unsupported. No version bypass, anti-detection logic, hidden runtime download, or undeclared binary extraction is used.

## Build environment limitation

The uploaded backend declares Go 1.26.1. The available environment has Go 1.23.2 and cannot download the newer toolchain, so the full backend Go test suite cannot be executed here. The modified generic host code is formatted/static-reviewed; the separately versioned Native Agent module builds/tests with the available toolchain.

## Public host capability boundary

No WeChat-specific Extension Kernel permission or runtime primitive was added. `runtime.nativeCompanions`, package allow-listing, SHA-256 verification, runtime descriptor injection, and generic channel identity validation are reusable by any extension.

- Added strict `clientVersion`/`versionVerified` handshake. Version-sensitive send/receive capabilities remain disabled unless the native Driver target version exactly matches the official client version independently detected by the Agent.
