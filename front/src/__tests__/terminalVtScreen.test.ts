import { describe, expect, it } from 'vitest';
import { TerminalVtScreen } from '@/components/workbench/terminalVtScreen';

describe('TerminalVtScreen', () => {
  it('uses bounded viewport rows and preserves previous output', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write(Array.from({ length: 20 }, (_, i) => `line-${i}\r\n`).join(''));
    expect(screen.snapshot().length).toBe(21);
    expect(screen.getText()).toContain('line-0');
    expect(screen.getText()).toContain('line-19');
  });
  it('positions the cursor and erases rows with split escape chunks', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write('abcdef\x1b[');
    screen.write('1;3HXY\x1b[0K');
    expect(screen.snapshot()[0].runs.map(run => run.text).join('').trimEnd()).toBe('abXY');
  });
  it('uses alternate screen without overwriting main buffer', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write('prompt> \x1b[?1049h\x1b[2Jvim');
    expect(screen.isAlternate).toBe(true);
    expect(screen.getText()).toContain('vim');
    expect(screen.getText()).not.toContain('prompt>');
    screen.write('\x1b[?1049l');
    expect(screen.isAlternate).toBe(false);
    expect(screen.getText()).toContain('prompt>');
    expect(screen.getText()).not.toContain('vim');
  });
  it('supports cursor movement, carriage return and colored text', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write('hello\rYES\x1b[31m!\x1b[0m');
    const line = screen.snapshot()[0];
    expect(line.runs.map(run => run.text).join('').trimEnd()).toBe('YES!o');
    expect(line.runs.find(run => run.text.includes('!'))?.fg).toBe('#f85149');
  });
  it('safely ignores titles and hyperlinks, without rendering injected markup', () => {
    const screen = new TerminalVtScreen();
    screen.write('\x1b]0;evil-title\x07\x1b]8;;https://example.com\x1b\\safe\x1b]8;;\x1b\\');
    expect(screen.getText()).toBe('safe');
  });
  it('limits terminal history and correctly clips on resize', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write('x\r\n'.repeat(900));
    expect(screen.snapshot().length).toBeLessThanOrEqual(610);
    screen.resize(30, 15);
    expect(screen.snapshot().at(-1)?.runs[0]).toBeDefined();
    expect(screen.columns).toBe(30);
    expect(screen.lines).toBe(15);
  });
  it('tracks the visible cursor and hides it using ANSI private mode', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write('abc');
    expect(screen.snapshot()[0].runs.some(run => run.cursor)).toBe(true);
    screen.write('\x1b[?25l');
    expect(screen.snapshot()[0].runs.some(run => run.cursor)).toBe(false);
    screen.write('\x1b[?25h');
    expect(screen.snapshot()[0].runs.some(run => run.cursor)).toBe(true);
  });
  it('supports bounded scrolling regions and erase-screen commands', () => {
    const screen = new TerminalVtScreen(20, 10);
    screen.write('header\x1b[3;7r\x1b[7;1Hfoo\r\nbar');
    expect(screen.getText()).toContain('header');
    expect(screen.getText()).toContain('bar');
    screen.write('\x1b[2J');
    expect(screen.getText()).not.toContain('header');
    expect(screen.getText()).not.toContain('bar');
  });
  it('tracks SGR 256 color and full RGB without retaining display injection', () => {
    const screen = new TerminalVtScreen();
    screen.write('\x1b[38;5;196mR\x1b[48;2;1;2;3mG');
    const row = screen.snapshot()[0];
    expect(row.runs.some(run => run.fg === 'rgb(255,0,0)')).toBe(true);
    expect(row.runs.some(run => run.bg === 'rgb(1,2,3)')).toBe(true);
  });
});
