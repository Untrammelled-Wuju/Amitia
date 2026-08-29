import { afterEach, describe, expect, it, vi } from "vitest";
import { resolveLogicalCanvasPoint } from "../pet-pointer-coordinates";

function makeCanvas(input: {
  backingWidth: number;
  backingHeight: number;
  cssWidth: number;
  cssHeight: number;
}): HTMLCanvasElement {
  return {
    width: input.backingWidth,
    height: input.backingHeight,
    getBoundingClientRect: () => ({
      left: 10,
      top: 20,
      width: input.cssWidth,
      height: input.cssHeight,
      right: 10 + input.cssWidth,
      bottom: 20 + input.cssHeight,
      x: 10,
      y: 20,
      toJSON: () => ({}),
    }),
  } as unknown as HTMLCanvasElement;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("resolveLogicalCanvasPoint", () => {
  it("keeps pointer coordinates in package logical space at 2x window scale", () => {
    vi.stubGlobal("window", { devicePixelRatio: 1 });
    const canvas = makeCanvas({
      backingWidth: 320,
      backingHeight: 480,
      cssWidth: 640,
      cssHeight: 960,
    });

    expect(resolveLogicalCanvasPoint(canvas, 330, 500)).toEqual({
      x: 160,
      y: 240,
    });
  });

  it("accounts for devicePixelRatio when deriving the logical backing size", () => {
    vi.stubGlobal("window", { devicePixelRatio: 2 });
    const canvas = makeCanvas({
      backingWidth: 640,
      backingHeight: 960,
      cssWidth: 160,
      cssHeight: 240,
    });

    expect(resolveLogicalCanvasPoint(canvas, 90, 140)).toEqual({
      x: 160,
      y: 240,
    });
  });

  it("clamps out-of-window pointer coordinates to the logical canvas bounds", () => {
    vi.stubGlobal("window", { devicePixelRatio: 1 });
    const canvas = makeCanvas({
      backingWidth: 320,
      backingHeight: 480,
      cssWidth: 320,
      cssHeight: 480,
    });

    expect(resolveLogicalCanvasPoint(canvas, -100, 1000)).toEqual({
      x: 0,
      y: 480,
    });
  });
});
