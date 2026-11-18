export interface UIRightPaneSnapshot {
  open: boolean;
  address?: string;
  title?: string;
  params?: Record<string, unknown>;
  sessionId?: string;
}

export interface UIRightPaneOpenOptions {
  title?: string;
  params?: Record<string, unknown>;
  sessionId?: string;
}

export class UIRightPaneRuntime {
  private snapshot: UIRightPaneSnapshot = Object.freeze({ open: false });
  private readonly listeners = new Set<() => void>();

  getSnapshot(): UIRightPaneSnapshot {
    return this.snapshot;
  }

  subscribe(listener: () => void): () => void {
    if (typeof listener !== "function") throw new Error("right pane subscriber must be a function");
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  open(address: string, options: UIRightPaneOpenOptions = {}): void {
    const value = address?.trim();
    if (!value) throw new Error("right pane address is required");
    this.publish({
      open: true,
      address: value,
      ...(options.title?.trim() ? { title: options.title.trim() } : {}),
      ...(options.params ? { params: { ...options.params } } : {}),
      ...(options.sessionId?.trim() ? { sessionId: options.sessionId.trim() } : {}),
    });
  }

  close(): void {
    if (!this.snapshot.open) return;
    this.publish({ open: false });
  }

  private publish(next: UIRightPaneSnapshot): void {
    this.snapshot = Object.freeze({
      ...next,
      ...(next.params ? { params: Object.freeze({ ...next.params }) } : {}),
    });
    for (const listener of Array.from(this.listeners)) listener();
  }
}

export const rightPaneRuntime = new UIRightPaneRuntime();
