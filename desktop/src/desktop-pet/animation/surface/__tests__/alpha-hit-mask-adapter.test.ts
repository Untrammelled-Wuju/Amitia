import { afterEach, describe, expect, it, vi } from "vitest";

import type { DecodedFrame, PetVisualSurface } from "../../contracts";
import { AlphaHitMaskAdapter } from "../alpha-hit-mask-adapter";

function makeSurface(): PetVisualSurface & { captureHitMask: ReturnType<typeof vi.fn> } {
  return {
    configureCanvas: vi.fn(),
    present: vi.fn(),
    retainLastFrame: vi.fn(),
    clear: vi.fn(),
    captureHitMask: vi.fn(() => ({
      width: 2,
      height: 2,
      data: new Uint8Array([0, 255, 255, 0]),
      threshold: 128,
    })),
    dispose: vi.fn(),
  };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("AlphaHitMaskAdapter", () => {
  it("reports whether a new mask was captured so callers do not resend throttled snapshots", () => {
    const surface = makeSurface();
    const adapter = new AlphaHitMaskAdapter({ surface, throttleMs: 100 });
    const frame = {} as DecodedFrame;
    const now = vi.spyOn(performance, "now");
    now.mockReturnValueOnce(100);
    now.mockReturnValueOnce(150);
    now.mockReturnValueOnce(201);

    expect(adapter.updateHitMask(frame)).toBe(true);
    expect(adapter.updateHitMask(frame)).toBe(false);
    expect(adapter.updateHitMask(frame)).toBe(true);
    expect(surface.captureHitMask).toHaveBeenCalledTimes(2);
  });

  it("never reports updates after disposal", () => {
    const surface = makeSurface();
    const adapter = new AlphaHitMaskAdapter({ surface, throttleMs: 0 });
    adapter.dispose();

    expect(adapter.updateHitMask({} as DecodedFrame)).toBe(false);
    expect(surface.captureHitMask).not.toHaveBeenCalled();
  });
});
