export type RuntimeState =
  | "starting"
  | "ready"
  | "stopping"
  | "stopped"
  | "crashed"
  | "failed";

let currentState: RuntimeState = "starting";
let isShuttingDown: boolean = false;
const shutdownHandlers: Array<() => Promise<void> | void> = [];

export function getState(): RuntimeState {
  return currentState;
}

export function setState(state: RuntimeState): void {
  currentState = state;
}

export function onShutdown(handler: () => Promise<void> | void): void {
  shutdownHandlers.push(handler);
}

export async function shutdown(reason: string): Promise<void> {
  if (isShuttingDown) {
    return;
  }
  isShuttingDown = true;
  setState("stopping");
  for (const handler of shutdownHandlers) {
    try {
      await handler();
    } catch (e) {
    }
  }
  setState("stopped");
  process.exit(0);
}
