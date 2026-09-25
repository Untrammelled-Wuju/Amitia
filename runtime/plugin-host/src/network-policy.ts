const DISABLED_GLOBALS = ["fetch", "WebSocket", "EventSource", "navigator"];
const DISABLED_PROCESS_MEMBERS = ["getBuiltinModule", "binding"];

function disableProperty(target: Record<string, unknown>, property: string): void {
  try {
    Object.defineProperty(target, property, {
      value: undefined,
      configurable: false,
      enumerable: false,
      writable: false,
    });
  } catch {
    try {
      target[property] = undefined;
    } catch {}
  }
}

export function applyNetworkDisabledPolicy(): void {
  for (const property of DISABLED_GLOBALS) {
    disableProperty(globalThis as unknown as Record<string, unknown>, property);
  }
  for (const property of DISABLED_PROCESS_MEMBERS) {
    disableProperty(process as unknown as Record<string, unknown>, property);
  }
}
