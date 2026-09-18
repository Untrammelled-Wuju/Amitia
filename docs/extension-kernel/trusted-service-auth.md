# Trusted Service Loopback Authentication

Trusted service runtimes receive a host-generated, process-lifetime bearer token for authenticated loopback RPC.

Environment contract:

- `AMITIA_SERVICE_AUTH_VERSION=1`
- `AMITIA_SERVICE_AUTH_TOKEN=<opaque random-derived token>`

The token is scoped to the extension ID and service module ID. It is never persisted and changes when the Amitia host process restarts.

For plugin channels backed by a trusted-service sidecar, the host-side HTTP provider automatically sends:

```http
Authorization: Bearer <AMITIA_SERVICE_AUTH_TOKEN>
```

Trusted services that expose loopback HTTP endpoints SHOULD require this header on every application endpoint. Health endpoints may also require it when the host owns all callers.

Security requirements:

1. Bind only to loopback unless a separate host capability explicitly authorizes broader ingress.
2. Do not expose the bearer token to restricted web UI modules.
3. Do not persist the bearer token to extension data, logs, diagnostics, or crash reports.
4. Do not enable wildcard CORS on authenticated loopback services.
5. Legacy unauthenticated callback listeners must be opt-in development compatibility only and disabled by default.

This is a public Extension Kernel capability. It is not tied to any particular plugin or channel.
