export function shouldInvalidateWorkbenchSnapshot(cause: unknown, previouslyVerifiedAuthority: string, currentAuthority: string): boolean {
  const explicitlyInvalid = !!cause && typeof cause === 'object' && 'invalidatesSnapshot' in cause && (cause as { invalidatesSnapshot?: unknown }).invalidatesSnapshot === true;
  return explicitlyInvalid || previouslyVerifiedAuthority !== currentAuthority;
}
