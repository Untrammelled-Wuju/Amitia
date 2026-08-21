export type Disposer = () => void | Promise<void>;

/**
 * ExtensionFiber owns lifecycle resources created by a single extension/module
 * bootstrap. Resources are disposed in reverse registration order so nested UI,
 * event and service registrations tear down safely.
 */
export interface ExtensionFiber {
  readonly id: string;
  readonly disposed: boolean;
  own(disposer: Disposer): Disposer;
  child(name: string): ExtensionFiber;
  dispose(): Promise<void>;
}

export class RuntimeFiber implements ExtensionFiber {
  private readonly disposers: Disposer[] = [];
  private readonly children: RuntimeFiber[] = [];
  private isDisposed = false;

  constructor(readonly id: string) {}

  get disposed(): boolean {
    return this.isDisposed;
  }

  own(disposer: Disposer): Disposer {
    if (typeof disposer !== "function") return disposer;
    if (this.isDisposed) {
      void disposer();
      return disposer;
    }
    this.disposers.push(disposer);
    return disposer;
  }

  child(name: string): ExtensionFiber {
    const child = new RuntimeFiber(`${this.id}/${name}`);
    if (this.isDisposed) {
      void child.dispose();
      return child;
    }
    this.children.push(child);
    return child;
  }

  async dispose(): Promise<void> {
    if (this.isDisposed) return;
    this.isDisposed = true;
    for (let i = this.children.length - 1; i >= 0; i--) {
      const child = this.children[i];
      if (child) await child.dispose();
    }
    for (let i = this.disposers.length - 1; i >= 0; i--) {
      const disposer = this.disposers[i];
      if (disposer) await disposer();
    }
    this.children.length = 0;
    this.disposers.length = 0;
  }
}
