export interface WorkbenchPolicyAuthority {
  providerEpoch: number;
  modeRevision: number;
  permissionRevision: number;
  coordinated: boolean;
}

export function readWorkbenchPolicyAuthority(value: unknown): WorkbenchPolicyAuthority | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const source = value as Record<string, unknown>;
  for (const name of ['providerEpoch', 'modeRevision', 'permissionRevision']) {
    const n = source[name];
    if (typeof n !== 'number' || !Number.isSafeInteger(n) || n < 0) return null;
  }
  if (typeof source.coordinated !== 'boolean') return null;
  return {
    providerEpoch: source.providerEpoch as number,
    modeRevision: source.modeRevision as number,
    permissionRevision: source.permissionRevision as number,
    coordinated: source.coordinated,
  };
}

export function sameWorkbenchPolicyAuthority(first: unknown, last: unknown, responseScope: unknown): boolean {
  const before = readWorkbenchPolicyAuthority(first);
  const after = readWorkbenchPolicyAuthority(last);
  const fromResponse = readWorkbenchPolicyAuthority(responseScope);
  if (!before || !after || !fromResponse) return false;
  return (Object.keys(before) as Array<keyof WorkbenchPolicyAuthority>).every(key =>
    before[key] === after[key] && after[key] === fromResponse[key]);
}

export function sameWorkbenchRoleSelection(before: unknown, after: unknown): boolean {
  return (before === undefined || typeof before === 'string') && (after === undefined || typeof after === 'string') && before === after;
}
