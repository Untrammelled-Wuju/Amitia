export interface TerminalCell {
  value: string;
  fg: string;
  bg: string;
  bold: boolean;
}

export interface TerminalRun {
  text: string;
  fg: string;
  bg: string;
  bold: boolean;
  cursor: boolean;
}

export interface TerminalRow {
  key: number;
  runs: TerminalRun[];
}

const ansiDark = ['#24292f', '#f85149', '#3fb950', '#d29922', '#79c0ff', '#bc8cff', '#39c5cf', '#c9d1d9'];
const ansiBright = ['#6e7681', '#ff7b72', '#56d364', '#e3b341', '#a5d6ff', '#d2a8ff', '#56d4dd', '#f0f6fc'];
const defaultFG = '#e1e6ee';
const defaultBG = '';
const emptyCell = (): TerminalCell => ({ value: ' ', fg: defaultFG, bg: defaultBG, bold: false });
const blankRow = (cols: number): TerminalCell[] => Array.from({ length: cols }, emptyCell);

function palette(index: number): string {
  if (index < 8) return ansiDark[index];
  if (index < 16) return ansiBright[index - 8];
  if (index >= 232) {
    const component = (index - 232) * 10 + 8;
    return `rgb(${component},${component},${component})`;
  }
  const cube = index - 16;
  const component = (value: number) => value === 0 ? 0 : 55 + value * 40;
  return `rgb(${component(Math.floor(cube / 36))},${component(Math.floor(cube / 6) % 6)},${component(cube % 6)})`;
}

export class TerminalVtScreen {
  readonly maxScrollback = 600;
  private cols: number;
  private rows: number;
  private primary: TerminalCell[][];
  private alternate: TerminalCell[][];
  private history: TerminalCell[][] = [];
  private alternateMode = false;
  private x = 0;
  private y = 0;
  private savedX = 0;
  private savedY = 0;
  private fg = defaultFG;
  private bg = defaultBG;
  private bold = false;
  private state: 'normal' | 'escape' | 'csi' | 'osc' | 'oscEsc' = 'normal';
  private params = '';
  private scrollTop = 0;
  private scrollBottom: number;
  cursorVisible = true;
  bracketedPaste = false;
  private revision = 0;

  constructor(cols = 100, rows = 30) {
    this.cols = Math.max(20, Math.min(300, Math.floor(cols)));
    this.rows = Math.max(10, Math.min(120, Math.floor(rows)));
    this.scrollBottom = this.rows - 1;
    this.primary = Array.from({ length: this.rows }, () => blankRow(this.cols));
    this.alternate = Array.from({ length: this.rows }, () => blankRow(this.cols));
  }

  get columns(): number { return this.cols; }
  get lines(): number { return this.rows; }
  get isAlternate(): boolean { return this.alternateMode; }
  get cursor(): { x: number; y: number } { return { x: this.x, y: this.y }; }
  get version(): number { return this.revision; }

  reset(): void {
    this.primary = Array.from({ length: this.rows }, () => blankRow(this.cols));
    this.alternate = Array.from({ length: this.rows }, () => blankRow(this.cols));
    this.history = [];
    this.alternateMode = false;
    this.x = 0; this.y = 0; this.savedX = 0; this.savedY = 0;
    this.fg = defaultFG; this.bg = defaultBG; this.bold = false;
    this.state = 'normal'; this.params = '';
    this.scrollTop = 0; this.scrollBottom = this.rows - 1;
    this.cursorVisible = true;
    this.bracketedPaste = false;
    this.revision++;
  }

  resize(cols: number, rows: number): void {
    const width = Math.max(20, Math.min(300, Math.floor(cols)));
    const height = Math.max(10, Math.min(120, Math.floor(rows)));
    if (width === this.cols && height === this.rows) return;
    const adapt = (source: TerminalCell[][]): TerminalCell[][] => Array.from({ length: height }, (_, i) => {
      const original = source[i] || [];
      return [...original.slice(0, width), ...Array.from({ length: Math.max(0, width - original.length) }, emptyCell)];
    });
    this.primary = adapt(this.primary);
    this.alternate = adapt(this.alternate);
    this.cols = width;
    this.rows = height;
    this.x = Math.min(this.x, width - 1);
    this.y = Math.min(this.y, height - 1);
    this.scrollTop = 0;
    this.scrollBottom = height - 1;
    this.revision++;
  }

  write(input: string): void {
    for (const value of input) this.consume(value);
    if (input) this.revision++;
  }

  annotate(text: string): void {
    this.write(`\r\n${text}\r\n`);
  }

  snapshot(includeHistory = true): TerminalRow[] {
    const scrollback = includeHistory && !this.alternateMode ? this.history : [];
    const visible = this.screen;
    return [...scrollback, ...visible].map((line, index) => ({ key: index, runs: this.runs(line, this.cursorVisible && index === scrollback.length + this.y ? Math.min(this.x, this.cols - 1) : -1) }));
  }

  getText(): string {
    return this.snapshot().map(row => row.runs.map(run => run.text).join('').trimEnd()).join('\n').trimEnd();
  }

  private get screen(): TerminalCell[][] { return this.alternateMode ? this.alternate : this.primary; }

  private runs(line: TerminalCell[], cursorCol: number): TerminalRun[] {
    const result: TerminalRun[] = [];
    for (let i = 0; i < line.length; i++) {
      const cell = line[i];
      const cursor = i === cursorCol;
      const last = result[result.length - 1];
      if (last && last.fg === cell.fg && last.bg === cell.bg && last.bold === cell.bold && last.cursor === cursor) last.text += cell.value;
      else result.push({ text: cell.value, fg: cell.fg, bg: cell.bg, bold: cell.bold, cursor });
    }
    while (result.length && result[result.length - 1].text.trimEnd() === '' && !result[result.length - 1].cursor) {
      result.pop();
    }
    if (!result.length) result.push({ text: ' ', fg: defaultFG, bg: '', bold: false, cursor: false });
    return result;
  }

  private consume(value: string): void {
    if (this.state === 'osc') {
      if (value === '\x07') this.state = 'normal';
      else if (value === '\x1b') this.state = 'oscEsc';
      return;
    }
    if (this.state === 'oscEsc') {
      this.state = value === '\\' ? 'normal' : 'osc';
      return;
    }
    if (this.state === 'escape') {
      this.state = 'normal';
      if (value === '[') { this.state = 'csi'; this.params = ''; }
      else if (value === ']') this.state = 'osc';
      else if (value === '7') { this.savedX = this.x; this.savedY = this.y; }
      else if (value === '8') { this.x = this.savedX; this.y = this.savedY; this.clamp(); }
      else if (value === 'D') this.newline(false);
      else if (value === 'E') this.newline(true);
      else if (value === 'M') this.reverseIndex();
      else if (value === 'c') this.reset();
      return;
    }
    if (this.state === 'csi') {
      if (value >= '@' && value <= '~') {
        this.executeCSI(value, this.params);
        this.state = 'normal'; this.params = '';
      } else if (this.params.length < 64) this.params += value;
      else { this.params = ''; this.state = 'normal'; }
      return;
    }
    if (value === '\x1b') { this.state = 'escape'; return; }
    if (value === '\r') { this.x = 0; return; }
    if (value === '\n' || value === '\x0b' || value === '\x0c') { this.newline(true); return; }
    if (value === '\b') { this.x = Math.max(0, this.x - 1); return; }
    if (value === '\t') { this.x = Math.min(this.cols - 1, Math.ceil((this.x + 1) / 8) * 8); return; }
    if (value.codePointAt(0)! < 32 || value === '\x7f') return;
    this.put(value);
  }

  private put(value: string): void {
    if (/\p{Mark}/u.test(value)) {
      const row = this.screen[this.y];
      let at = Math.min(this.x - 1, this.cols - 1);
      while (at >= 0 && row[at].value === "") at--;
      if (at >= 0) row[at].value += value;
      return;
    }
    const width = this.width(value);
    if (this.x + width > this.cols) this.newline(true);
    const current = this.screen[this.y];
    current[this.x] = { value, fg: this.fg, bg: this.bg, bold: this.bold };
    if (width === 2 && this.x + 1 < this.cols) current[this.x + 1] = { value: '', fg: this.fg, bg: this.bg, bold: this.bold };
    this.x += width;
    if (this.x >= this.cols) this.x = this.cols;
  }

  private width(value: string): number {
    const point = value.codePointAt(0)!;
    return point >= 0x1100 && (point <= 0x115f || point >= 0x2e80 && point <= 0xa4cf || point >= 0xac00 && point <= 0xd7a3 || point >= 0xf900 && point <= 0xfaff || point >= 0xfe10 && point <= 0xfe6f || point >= 0xff00 && point <= 0xff60 || point >= 0xffe0 && point <= 0xffe6 || point >= 0x20000 && point <= 0x3fffd) ? 2 : 1;
  }

  private newline(resetX: boolean): void {
    if (resetX) this.x = 0;
    if (this.y === this.scrollBottom) this.scrollUp(1);
    else this.y = Math.min(this.rows - 1, this.y + 1);
  }

  private scrollUp(count: number): void {
    const screen = this.screen;
    for (let i = 0; i < count; i++) {
      const removed = screen.splice(this.scrollTop, 1)[0];
      screen.splice(this.scrollBottom, 0, blankRow(this.cols));
      if (!this.alternateMode && this.scrollTop === 0) {
        this.history.push(removed);
        if (this.history.length > this.maxScrollback) this.history.splice(0, this.history.length - this.maxScrollback);
      }
    }
  }

  private reverseIndex(): void {
    if (this.y === this.scrollTop) {
      this.screen.splice(this.scrollBottom, 1);
      this.screen.splice(this.scrollTop, 0, blankRow(this.cols));
    } else this.y = Math.max(0, this.y - 1);
  }

  private clamp(): void {
    this.x = Math.max(0, Math.min(this.cols - 1, this.x));
    this.y = Math.max(0, Math.min(this.rows - 1, this.y));
  }

  private executeCSI(op: string, raw: string): void {
    const privateMode = raw.startsWith('?');
    const values = (privateMode ? raw.slice(1) : raw).split(';').map(v => v === '' ? 0 : Number.parseInt(v, 10)).slice(0, 24);
    if (values.some(v => !Number.isFinite(v) || v < 0 || v > 9999)) return;
    const first = values[0] || 1;
    const position = (value: number) => Math.max(0, value - 1);
    if (privateMode) {
      if (op === 'h' || op === 'l') {
        const on = op === 'h';
        if (values.includes(25)) this.cursorVisible = on;
        if (values.includes(2004)) this.bracketedPaste = on;
        if (values.some(v => v === 47 || v === 1047 || v === 1049)) {
          if (this.alternateMode !== on) {
            this.alternateMode = on; this.x = 0; this.y = 0;
            if (on) this.alternate = Array.from({ length: this.rows }, () => blankRow(this.cols));
          }
        }
      }
      return;
    }
    switch (op) {
      case 'A': this.y -= first; break;
      case 'B': this.y += first; break;
      case 'C': this.x += first; break;
      case 'D': this.x -= first; break;
      case 'E': this.y += first; this.x = 0; break;
      case 'F': this.y -= first; this.x = 0; break;
      case 'G': this.x = position(first); break;
      case 'd': this.y = position(first); break;
      case 'H': case 'f': this.y = position(first); this.x = position(values[1] || 1); break;
      case 's': this.savedX = this.x; this.savedY = this.y; break;
      case 'u': this.x = this.savedX; this.y = this.savedY; break;
      case 'r': {
        const top = position(first);
        const bottom = position(values[1] || this.rows);
        if (top < bottom && bottom < this.rows) { this.scrollTop = top; this.scrollBottom = bottom; this.x = 0; this.y = 0; }
        break;
      }
      case 'J': this.eraseDisplay(values[0] || 0); break;
      case 'K': this.eraseLine(values[0] || 0); break;
      case 'm': this.sgr(values); break;
      case 'P': this.screen[this.y].splice(this.x, first); this.screen[this.y].push(...Array.from({ length: first }, emptyCell)); this.screen[this.y].length = this.cols; break;
      case '@': this.screen[this.y].splice(this.x, 0, ...Array.from({ length: Math.min(first, this.cols) }, emptyCell)); this.screen[this.y].length = this.cols; break;
      case 'L': for (let i = 0; i < Math.min(first, this.rows); i++) { this.screen.splice(this.y, 0, blankRow(this.cols)); this.screen.splice(this.scrollBottom + 1, 1); } break;
      case 'M': for (let i = 0; i < Math.min(first, this.rows); i++) { this.screen.splice(this.y, 1); this.screen.splice(this.scrollBottom, 0, blankRow(this.cols)); } break;
    }
    this.clamp();
  }

  private eraseLine(mode: number): void {
    const line = this.screen[this.y];
    const start = mode === 1 ? 0 : mode === 2 ? 0 : this.x;
    const end = mode === 0 ? this.cols : mode === 1 ? this.x + 1 : this.cols;
    for (let i = start; i < end; i++) line[i] = emptyCell();
  }

  private eraseDisplay(mode: number): void {
    if (mode === 3) { this.history = []; return; }
    if (mode === 2) {
      for (let i = 0; i < this.rows; i++) this.screen[i] = blankRow(this.cols);
      return;
    }
    this.eraseLine(mode);
    for (let y = 0; y < this.rows; y++) {
      if (mode === 0 && y > this.y || mode === 1 && y < this.y) this.screen[y] = blankRow(this.cols);
    }
  }

  private sgr(values: number[]): void {
    for (let i = 0; i < values.length; i++) {
      const v = values[i];
      if (v === 0) { this.fg = defaultFG; this.bg = defaultBG; this.bold = false; }
      else if (v === 1) this.bold = true;
      else if (v === 22) this.bold = false;
      else if (v === 39) this.fg = defaultFG;
      else if (v === 49) this.bg = defaultBG;
      else if (v >= 30 && v <= 37) this.fg = ansiDark[v - 30];
      else if (v >= 40 && v <= 47) this.bg = ansiDark[v - 40];
      else if (v >= 90 && v <= 97) this.fg = ansiBright[v - 90];
      else if (v >= 100 && v <= 107) this.bg = ansiBright[v - 100];
      else if ((v === 38 || v === 48) && values[i + 1] === 5 && values[i + 2] <= 255) {
        const color = palette(values[i + 2]);
        if (v === 38) this.fg = color; else this.bg = color;
        i += 2;
      } else if ((v === 38 || v === 48) && values[i + 1] === 2 && i + 4 < values.length && values.slice(i + 2, i + 5).every(n => n <= 255)) {
        const color = `rgb(${values[i + 2]},${values[i + 3]},${values[i + 4]})`;
        if (v === 38) this.fg = color; else this.bg = color;
        i += 4;
      }
    }
  }
}
