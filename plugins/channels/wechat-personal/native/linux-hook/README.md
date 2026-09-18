# Linux native hook companion

This library is intentionally fail-closed. The current build only provides an IPC liveness endpoint when loaded with `LD_PRELOAD`; it does not call unverified WeChat internal offsets.

The Linux driver architecture is compatible with official Linux WeChat. Version-specific QR/message adapters must be added only after verifying the target build. The plugin will expose the platform as installed but will report `driver_required` until those capabilities are present.
