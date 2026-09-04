import type { GenerationToken, GenerationManager } from "./contracts";

class GenerationTokenImpl implements GenerationToken {
  readonly revision: number;
  readonly generation: number;
  private manager: GenerationManagerImpl;
  readonly controller: AbortController;
  readonly signal: AbortSignal;
  private current: boolean;

  constructor(manager: GenerationManagerImpl, revision: number, generation: number) {
    this.manager = manager;
    this.revision = revision;
    this.generation = generation;
    this.controller = new AbortController();
    this.signal = this.controller.signal;
    this.current = true;
  }

  isCurrent(): boolean {
    return this.current;
  }

  markStale(): void {
    this.current = false;
  }

  abort(): void {
    this.controller.abort();
  }
}

export class GenerationManagerImpl implements GenerationManager {
  private counter = 0;
  private currentToken: GenerationTokenImpl | null = null;

  next(revision: number): GenerationToken {
    if (this.currentToken) {
      this.currentToken.markStale();
      this.currentToken.abort();
    }
    this.counter += 1;
    const token = new GenerationTokenImpl(this, revision, this.counter);
    this.currentToken = token;
    return token;
  }

  current(): GenerationToken | null {
    return this.currentToken;
  }

  isCurrent(token: GenerationToken): boolean {
    return this.currentToken !== null && this.currentToken === token && this.currentToken.isCurrent();
  }

  markCurrentStale(): void {
    if (this.currentToken) {
      this.currentToken.markStale();
      this.currentToken = null;
    }
  }

  hasCurrent(): boolean {
    return this.currentToken !== null;
  }
}
