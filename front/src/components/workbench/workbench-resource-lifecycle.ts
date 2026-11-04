import type { WorkbenchViewKind } from '@/stores/workbench';

export function needsWorkbenchWorkspaceData(kind: WorkbenchViewKind, targetDeviceId?: string): boolean {
  if (kind === 'agents') return false;
  return !targetDeviceId && (kind === 'files' || kind === 'git' || kind === 'diff');
}
