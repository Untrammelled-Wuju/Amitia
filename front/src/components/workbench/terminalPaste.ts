export function encodeTerminalPaste(text: string, bracketed: boolean, maxBytes = 4096): string | null {
  const safe = text.replace(/\x1b/g, '').replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, '');
  const payload = bracketed ? `\x1b[200~${safe}\x1b[201~` : safe;
  return safe && new TextEncoder().encode(payload).length <= maxBytes ? payload : null;
}
