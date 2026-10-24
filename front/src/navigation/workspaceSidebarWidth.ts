export const WORKSPACE_SIDEBAR_MIN_WIDTH = 240;
export const WORKSPACE_SIDEBAR_DEFAULT_WIDTH = 240;
export const WORKSPACE_SIDEBAR_MAX_RATIO = 0.4;

export function maxWorkspaceSidebarWidth(viewportWidth: number): number {
  const viewport = Number.isFinite(viewportWidth) ? Math.max(0, viewportWidth) : 0;
  return Math.max(WORKSPACE_SIDEBAR_MIN_WIDTH, Math.floor(viewport * WORKSPACE_SIDEBAR_MAX_RATIO));
}

export function clampWorkspaceSidebarWidth(width: number, viewportWidth: number): number {
  const value = Number.isFinite(width) ? Math.round(width) : WORKSPACE_SIDEBAR_DEFAULT_WIDTH;
  return Math.min(maxWorkspaceSidebarWidth(viewportWidth), Math.max(WORKSPACE_SIDEBAR_MIN_WIDTH, value));
}
