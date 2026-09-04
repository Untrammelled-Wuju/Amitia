import type {
  PetVisualSurface,
  DecodedFrame,
  AlphaHitMaskSnapshot,
} from "../contracts";

const DEFAULT_THROTTLE_MS = 100;
const DEFAULT_OPACITY_THRESHOLD = 128;
const EMPTY_MASK: AlphaHitMaskSnapshot = {
  width: 0,
  height: 0,
  data: new Uint8Array(0),
  threshold: DEFAULT_OPACITY_THRESHOLD,
};

export class AlphaHitMaskAdapter {
  private readonly surface: PetVisualSurface;
  private readonly throttleMs: number;
  private mask: AlphaHitMaskSnapshot;
  private lastUpdateMs: number = 0;
  private disposed: boolean = false;

  constructor(input: { surface: PetVisualSurface; throttleMs?: number }) {
    this.surface = input.surface;
    this.throttleMs = input.throttleMs ?? DEFAULT_THROTTLE_MS;
    this.mask = EMPTY_MASK;
  }

  updateHitMask(frame: DecodedFrame): boolean {
    if (this.disposed) {
      return false;
    }
    void frame;
    const now = this.now();
    if (now - this.lastUpdateMs < this.throttleMs) {
      return false;
    }
    this.lastUpdateMs = now;
    this.mask = this.surface.captureHitMask();
    return true;
  }

  isOpaque(x: number, y: number, threshold?: number): boolean {
    if (this.disposed) {
      return false;
    }
    const mask = this.mask;
    if (mask.width <= 0 || mask.height <= 0) {
      return false;
    }
    const normalizedX = this.clamp01(x);
    const normalizedY = this.clamp01(y);
    const cellX = Math.floor(normalizedX * mask.width);
    const cellY = Math.floor(normalizedY * mask.height);
    if (cellX < 0 || cellY < 0 || cellX >= mask.width || cellY >= mask.height) {
      return false;
    }
    const alpha = mask.data[cellY * mask.width + cellX];
    const effectiveThreshold = threshold ?? mask.threshold;
    return alpha >= effectiveThreshold;
  }

  getMask(): AlphaHitMaskSnapshot {
    if (this.disposed) {
      return EMPTY_MASK;
    }
    return this.mask;
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    this.mask = EMPTY_MASK;
    this.lastUpdateMs = 0;
  }

  private clamp01(value: number): number {
    if (Number.isNaN(value)) {
      return 0;
    }
    if (value < 0) {
      return 0;
    }
    if (value > 1) {
      return 1;
    }
    return value;
  }

  private now(): number {
    if (typeof performance !== "undefined" && typeof performance.now === "function") {
      return performance.now();
    }
    return Date.now();
  }
}
