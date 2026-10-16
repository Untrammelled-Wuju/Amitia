import { describe, expect, it } from 'vitest';
import { encodeTerminalPaste } from '../components/workbench/terminalPaste';
import { TerminalVtScreen } from '../components/workbench/terminalVtScreen';

describe('Workbench terminal paste and Unicode', () => {
 it('uses bracketed paste only when the terminal explicitly requests it', () => {
  const screen = new TerminalVtScreen();
  expect(screen.bracketedPaste).toBe(false);
  screen.write('\x1b[?2004h');
  expect(screen.bracketedPaste).toBe(true);
  expect(encodeTerminalPaste('hello\nworld', screen.bracketedPaste)).toBe('\x1b[200~hello\nworld\x1b[201~');
  screen.write('\x1b[?2004l');
  expect(screen.bracketedPaste).toBe(false);
 });
 it('discards control escape sequences from untrusted clipboard data', () => {
  expect(encodeTerminalPaste('hi\x1b[201~\x00there', true)).toBe('\x1b[200~hi[201~there\x1b[201~');
  expect(encodeTerminalPaste('X'.repeat(4100), true)).toBeNull();
  expect(encodeTerminalPaste('')).toBeNull();
 });
 it('renders combining accents in the preceding terminal cell', () => {
  const screen = new TerminalVtScreen(20, 10);
  screen.write('e\u0301');
  expect(screen.snapshot()[0].runs.map(r => r.text).join('').trimEnd()).toBe('e\u0301');
  expect(screen.cursor.x).toBe(1);
 });
});
