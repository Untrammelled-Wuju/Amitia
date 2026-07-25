export type ClickThroughMode = "alpha" | "boundingBox" | "none";

export interface Position {
  x: number;
  y: number;
  screenId?: string;
}

export interface PetRuntimePositionRecord {
  screenId: string;
  x: number;
  y: number;
  scale: number;
}

export type PetWindowEvent = "move" | "resize" | "close" | "crashed";

export type PetWindowEventListener = (...args: unknown[]) => void;

export interface ScreenInfo {
  id: string;
  bounds: { x: number; y: number; width: number; height: number };
  workArea: { x: number; y: number; width: number; height: number };
  scaleFactor: number;
  isPrimary: boolean;
  label: string;
}

export interface DesktopPetWindowOptions {
  canvasWidth: number;
  canvasHeight: number;
  scale: number;
  alwaysOnTop: boolean;
  clickThroughMode?: ClickThroughMode;
  position?: Position;
}

export interface DesktopPetPosition {
  screenId: string;
  x: number;
  y: number;
  scale: number;
}

export const PET_WINDOW_SCALE_MIN = 0.5;
export const PET_WINDOW_SCALE_MAX = 2.0;
export const PET_WINDOW_SCALE_DEFAULT = 1.0;
