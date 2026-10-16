export class TerminalTextBuffer {
  private history = '';
  private line = '';
  private column = 0;
  private state: 'text' | 'escape' | 'csi' | 'osc' | 'osc-escape' = 'text';
  private sequence = '';
  private readonly cap: number;

  constructor(cap = 240_000) { this.cap = cap; }

  get text(): string { return this.history + this.line; }

  reset(): void {
    this.history = '';
    this.line = '';
    this.column = 0;
    this.state = 'text';
    this.sequence = '';
  }

  truncated(): void {
    this.state = 'text';
    this.sequence = '';
    this.line = '';
    this.column = 0;
    this.history += '\n[较早的终端输出已被截断]\n';
    this.compact();
  }

  write(chunk: string): void {
    for (const char of chunk) this.consume(char);
    this.compact();
  }

  private compact(): void {
    if (this.history.length > this.cap) this.history = this.history.slice(-this.cap);
    if (this.line.length > this.cap) this.line = this.line.slice(-this.cap);
  }

  private consume(char: string): void {
    if (this.state === 'escape') {
      if (char === '[') { this.state = 'csi'; this.sequence = ''; }
      else if (char === ']') { this.state = 'osc'; this.sequence = ''; }
      else this.state = 'text';
      return;
    }
    if (this.state === 'osc') {
      if (char === '\x07') this.state = 'text';
      else if (char === '\x1b') this.state = 'osc-escape';
      else { this.sequence += char; if (this.sequence.length > 512) { this.state = 'text'; this.sequence = ''; } }
      return;
    }
    if (this.state === 'osc-escape') {
      this.state = char === '\\' ? 'text' : 'osc';
      return;
    }
    if (this.state === 'csi') {
      const code = char.charCodeAt(0);
      if (code >= 0x40 && code <= 0x7e) {
        if (char === 'K' && (this.sequence === '' || this.sequence === '0')) this.line = this.line.slice(0, this.column);
        if (char === 'K' && this.sequence === '2') { this.line = ''; this.column = 0; }
        if (char === 'J' && (this.sequence === '2' || this.sequence === '3')) this.reset();
        this.state = 'text'; this.sequence = '';
      } else if (this.sequence.length < 32) this.sequence += char;
      else { this.state = 'text'; this.sequence = ''; }
      return;
    }
    if (char === '\x1b') { this.state = 'escape'; return; }
    if (char === '\r') { this.column = 0; return; }
    if (char === '\b') { this.column = Math.max(0, this.column - 1); return; }
    if (char === '\n') {
      this.history += this.line + '\n';
      this.line = ''; this.column = 0;
      if (this.history.length > this.cap * 2) this.compact();
      return;
    }
    if (char === '\t') { for (let i = 0; i < 4; i++) this.insert(' '); return; }
    if (char.codePointAt(0)! < 32 || char === '\x7f') return;
    this.insert(char);
  }

  private insert(char: string): void {
    const characters = Array.from(this.line);
    if (this.column < characters.length) characters[this.column] = char;
    else characters.push(char);
    this.line = characters.join('');
    this.column++;
  }
}

export function keyToTerminalData(event: Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'altKey' | 'metaKey'>): string | null {
  if (event.metaKey) return null;
  const keys: Record<string, string> = {
    Enter: '\r', Backspace: '\x7f', Tab: '\t', Escape: '\x1b',
    ArrowUp: '\x1b[A', ArrowDown: '\x1b[B', ArrowRight: '\x1b[C', ArrowLeft: '\x1b[D',
    Home: '\x1b[H', End: '\x1b[F', Delete: '\x1b[3~', PageUp: '\x1b[5~', PageDown: '\x1b[6~',
  };
  if (event.ctrlKey && !event.altKey && /^[a-z]$/i.test(event.key)) return String.fromCharCode(event.key.toUpperCase().charCodeAt(0) - 64);
  const key = keys[event.key];
  if (key) return event.altKey ? `\x1b${key}` : key;
  return !event.ctrlKey && event.key.length === 1 ? (event.altKey ? `\x1b${event.key}` : event.key) : null;
}
