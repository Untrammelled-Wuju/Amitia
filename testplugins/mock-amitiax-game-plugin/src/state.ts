export interface FakeWorldState {
  tick: number;
  playerX: number;
  playerY: number;
  health: number;
  mode: 'idle' | 'running';
}

export class FakeGameRuntime {
  private state: FakeWorldState;
  private running: boolean = false;
  private tickInterval: NodeJS.Timeout | null = null;

  constructor() {
    this.state = {
      tick: 0,
      playerX: 0,
      playerY: 0,
      health: 100,
      mode: 'idle',
    };
  }

  start(): void {
    this.running = true;
    this.state.mode = 'running';
  }

  stop(): void {
    this.running = false;
    this.state.mode = 'idle';
    if (this.tickInterval) {
      clearInterval(this.tickInterval);
      this.tickInterval = null;
    }
  }

  advanceTick(): void {
    if (!this.running) return;
    this.state.tick++;
    this.state.playerX += 1;
    this.state.playerY += 1;
  }

  getState(): FakeWorldState {
    return { ...this.state };
  }

  setMode(mode: 'idle' | 'running'): void {
    this.state.mode = mode;
    if (mode === 'running' && !this.running) {
      this.start();
    } else if (mode === 'idle' && this.running) {
      this.stop();
    }
  }

  applyDamage(amount: number): void {
    this.state.health = Math.max(0, this.state.health - amount);
  }

  heal(amount: number): void {
    this.state.health = Math.min(100, this.state.health + amount);
  }

  isRunning(): boolean {
    return this.running;
  }

  reset(): void {
    this.state = {
      tick: 0,
      playerX: 0,
      playerY: 0,
      health: 100,
      mode: 'idle',
    };
  }
}

export class PluginStateStore {
  private states: Map<string, unknown> = new Map();
  private versions: Map<string, number> = new Map();

  set(key: string, value: unknown): void {
    this.states.set(key, value);
    const current = this.versions.get(key) || 0;
    this.versions.set(key, current + 1);
  }

  get(key: string): { found: boolean; value: unknown; version: number } {
    const found = this.states.has(key);
    return {
      found,
      value: found ? this.states.get(key) : undefined,
      version: this.versions.get(key) || 0,
    };
  }

  has(key: string): boolean {
    return this.states.has(key);
  }

  delete(key: string): boolean {
    const existed = this.states.has(key);
    this.states.delete(key);
    this.versions.delete(key);
    return existed;
  }

  clear(): void {
    this.states.clear();
    this.versions.clear();
  }

  keys(): string[] {
    return Array.from(this.states.keys());
  }

  size(): number {
    return this.states.size;
  }
}
