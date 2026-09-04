import { describe, expect, it, vi } from "vitest";
import { CanvasPetVisualSurface } from "../surface/canvas-pet-visual-surface";

function makeCanvas(): HTMLCanvasElement {
  const context = {
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    drawImage: vi.fn(),
    getImageData: vi.fn(() => ({ data: new Uint8ClampedArray(4) })),
    imageSmoothingEnabled: true,
  } as unknown as CanvasRenderingContext2D;

  return {
    width: 0,
    height: 0,
    style: { width: "", height: "" },
    getContext: vi.fn(() => context),
  } as unknown as HTMLCanvasElement;
}

describe("CanvasPetVisualSurface runtime scaling", () => {
  it("keeps logical backing size while CSS always fills the scaled BrowserWindow", () => {
    const canvas = makeCanvas();
    const surface = new CanvasPetVisualSurface({ canvas });

    surface.configureCanvas({
      width: 320,
      height: 480,
      scale: 1,
      interpolationMode: "smooth",
    });

    expect(canvas.width).toBe(320);
    expect(canvas.height).toBe(480);
    expect(canvas.style.width).toBe("100%");
    expect(canvas.style.height).toBe("100%");
    surface.dispose();
  });
});
