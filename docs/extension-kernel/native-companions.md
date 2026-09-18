# Native Companions

`runtime.nativeCompanions` is a public Extension Kernel capability for any trusted service extension that needs platform-native helper binaries or passive native libraries. It is not tied to any specific plugin or product feature.

## Goals

- keep `.amitiax` executable assets fail-closed by default;
- allow a service module to explicitly declare a small set of platform-native companions;
- verify every declared native file by SHA-256 during package validation and service registration;
- expose only the companions matching the current OS/architecture to the service runtime;
- preserve the normal Trusted Service subprocess, sandbox, network, and lifecycle limits.

## Manifest

A service module may declare:

```json
{
  "runtime": {
    "type": "service",
    "entryPoint": "launcher.mjs",
    "maxSubprocesses": 2,
    "nativeCompanions": [
      {
        "id": "native-agent-windows-x64",
        "platform": "windows",
        "architecture": "amd64",
        "path": "native/windows-x64/agent.exe",
        "sha256": "<64 lowercase hex chars>",
        "executable": true,
        "args": []
      },
      {
        "id": "native-helper-linux-x64",
        "platform": "linux",
        "architecture": "amd64",
        "path": "native/linux-x64/helper.so",
        "sha256": "<64 lowercase hex chars>",
        "executable": false
      }
    ]
  }
}
```

The field is intentionally generic. Extension code must discover companions by `id` rather than relying on package-relative paths or host-specific hard-coded plugin names.

## Runtime contract

For a trusted service, the host injects `AMITIA_NATIVE_COMPANIONS_VERSION=1` plus `AMITIA_NATIVE_COMPANIONS` as a JSON array containing only descriptors compatible with the current platform and architecture. Each descriptor contains the canonical installed path and verified hash.

Example:

```json
[
  {
    "id": "native-agent-linux-x64",
    "platform": "linux",
    "architecture": "amd64",
    "path": "/.../modules/runtime/native/linux-x64/agent",
    "sha256": "...",
    "executable": true,
    "args": []
  }
]
```

A service should reject an unsupported non-empty contract version, treat a missing companion as an unsupported capability, and fail closed. It must not download a replacement binary, reinterpret undeclared assets as executables, or bypass package verification.

## Security rules

1. Undeclared executable/shared-library files remain blocked by package security.
2. Declared paths must stay inside the declaring module.
3. SHA-256 must match both package metadata and the installed file before the Trusted Service definition is registered.
4. Only companions for the current OS/architecture are exposed at runtime.
5. `executable: true` controls executable permission restoration; passive libraries do not become executable by default.
6. Native companions do not receive extra network/filesystem/process privileges merely because they are declared. Those remain governed by the service module policy and Trusted Service limits.

## Public channel identity

Channel plugins use the generic [`channel.provider` contract](./channel-provider-contract.md). The host resolves providers by manifest metadata and validates inbound requests by trusted-service token, extension ID, module ID, and declared `channelId`. The host does not contain built-in QQ, WeChat, Telegram, or Feishu channel mappings.
