/**
 * Convert CSS/window pointer coordinates back into the package's logical canvas
 * coordinate system. The BrowserWindow may be scaled independently from the
 * canvas backing store, so raw offsetX/offsetY are not stable runtime values.
 */
export function resolveLogicalCanvasPoint(
  canvas: HTMLCanvasElement,
  clientX: number,
  clientY: number,
): { x: number; y: number } {
  const rect = canvas.getBoundingClientRect();
  const dpr =
    typeof window !== "undefined" &&
    typeof window.devicePixelRatio === "number" &&
    Number.isFinite(window.devicePixelRatio) &&
    window.devicePixelRatio > 0
      ? window.devicePixelRatio
      : 1;
  const logicalWidth = canvas.width > 0 ? canvas.width / dpr : rect.width;
  const logicalHeight = canvas.height > 0 ? canvas.height / dpr : rect.height;
  const x =
    rect.width > 0
      ? ((clientX - rect.left) / rect.width) * logicalWidth
      : 0;
  const y =
    rect.height > 0
      ? ((clientY - rect.top) / rect.height) * logicalHeight
      : 0;

  return {
    x: Math.max(0, Math.min(logicalWidth, x)),
    y: Math.max(0, Math.min(logicalHeight, y)),
  };
}
