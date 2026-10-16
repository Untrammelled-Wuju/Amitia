import type { WorkbenchViewKind } from '@/stores/workbench';

export function needsWorkbenchWorkspaceData(kind: WorkbenchViewKind, targetDeviceId?: string): boolean {
  return !targetDeviceId && (kind === 'files' || kind === 'git' || kind === 'diff');
}
