<template>
  <div ref="viewport" class="vt-viewport" role="log" tabindex="0" aria-label="交互终端画面" @keydown="keyDown" @paste="onPaste" @click="focus">
    <div class="vt-lines">
      <div v-for="row in display" :key="row.key" class="vt-line">
        <span v-for="(run, i) in row.runs" :key="i" :class="{ 'vt-cursor': run.cursor }" :style="{ color: run.fg, backgroundColor: run.bg || 'transparent', fontWeight: run.bold ? 'bold' : 'normal' }">{{ run.text }}</span>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue';
import { TerminalVtScreen, type TerminalRow } from './terminalVtScreen';
import { keyToTerminalData } from './terminalTextBuffer';
import { encodeTerminalPaste } from './terminalPaste';
const props = defineProps<{ columns: number; lines: number; enabled: boolean }>();
const emit = defineEmits<{ input: [value: string]; ready: []; size: [rows: number, cols: number] }>();
const viewport = ref<HTMLDivElement>();
const screen = new TerminalVtScreen(props.columns, props.lines);
const display = shallowRef<TerminalRow[]>(screen.snapshot());
let disposed = false;
let pending = false;
let scrollBottom = true;
let viewportObserver: ResizeObserver | undefined;
let lastSize = '';
function measureSize() {
  const element = viewport.value;
  if (!element) return;
  const styles = getComputedStyle(element);
  const canvas = document.createElement('canvas');
  const context = canvas.getContext('2d');
  if (!context) return;
  context.font = `${styles.fontSize} ${styles.fontFamily}`;
  const charWidth = context.measureText('M').width;
  if (charWidth < 2) return;
  const lineHeight = Number.parseFloat(styles.lineHeight) || 18.6;
  const cols = Math.max(20, Math.min(300, Math.floor((element.clientWidth - 20) / charWidth)));
  const rows = Math.max(10, Math.min(120, Math.floor((element.clientHeight - 20) / lineHeight)));
  const next = `${rows}:${cols}`;
  if (next !== lastSize) { lastSize = next; emit('size', rows, cols); }
}
function refresh() {
  if (disposed || pending) return;
  pending = true;
  void nextTick(() => {
    if (disposed) return;
    pending = false;
    display.value = screen.snapshot();
    void nextTick(() => { if (viewport.value && scrollBottom) viewport.value.scrollTop = viewport.value.scrollHeight; });
  });
}
function write(value: string) { screen.write(value); refresh(); }
function reset() { screen.reset(); refresh(); }
function truncated() { screen.annotate('[较早的终端输出已被截断]'); refresh(); }
function resize(cols: number, rows: number) { screen.resize(cols, rows); refresh(); }
function focus() { viewport.value?.focus(); }
function keyDown(event: KeyboardEvent) {
  if (!props.enabled || event.isComposing) return;
  if (event.ctrlKey && ['v', 'c'].includes(event.key.toLowerCase()) && !event.shiftKey && !event.altKey && !event.metaKey) {
    if (event.key.toLowerCase() === 'v') return;
    if (window.getSelection()?.toString()) return;
  }
  const data = keyToTerminalData(event);
  if (data === null) return;
  event.preventDefault();
  emit('input', data);
}
function onPaste(event: ClipboardEvent) {
  if (!props.enabled) return;
  const content = event.clipboardData?.getData('text/plain');
  if (!content) return;
  event.preventDefault();
  const data = encodeTerminalPaste(content, screen.bracketedPaste);
  if (data) emit('input', data);
}
function onScroll() {
  if (!viewport.value) return;
  scrollBottom = viewport.value.scrollTop + viewport.value.clientHeight >= viewport.value.scrollHeight - 40;
}
onMounted(() => { viewport.value?.addEventListener('scroll', onScroll, { passive: true }); emit('ready'); if (typeof ResizeObserver !== 'undefined') { viewportObserver = new ResizeObserver(measureSize); if (viewport.value) viewportObserver.observe(viewport.value); } measureSize(); });
onBeforeUnmount(() => { disposed = true; viewportObserver?.disconnect(); viewport.value?.removeEventListener('scroll', onScroll); });
defineExpose({ write, reset, truncated, resize, focus });
</script>
<style scoped>
.vt-viewport{flex:1;min-height:0;min-width:0;overflow:auto;padding:10px;font:12px/1.55 ui-monospace,Consolas,monospace;background:#14171b;color:#e1e6ee;outline:0;white-space:pre;user-select:text}
.vt-viewport:focus-visible{box-shadow:inset 0 0 0 2px #789ac5}
.vt-lines{min-width:max-content;min-height:100%}
.vt-line{min-height:1.55em;white-space:pre}
.vt-viewport:focus .vt-cursor{outline:1px solid #d0d7de;background:#d0d7de!important;color:#14171b!important}
.vt-cursor{box-shadow:inset 0 -2px #8b949e}
</style>
