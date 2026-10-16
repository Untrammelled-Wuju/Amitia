import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const sourceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const source = path.join(sourceRoot, 'front/src/components/workbench/terminalSnapshotReconcile.ts');
const typescriptRoot = execFileSync('npm', ['root', '-g'], { encoding: 'utf8', timeout: 10000 }).trim();
const ts = require(path.join(typescriptRoot, 'typescript'));
const result = ts.transpileModule(fs.readFileSync(source, 'utf8'), {
  fileName: source,
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, strict: true },
  reportDiagnostics: true,
});
assert.equal(result.diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0);
const module = { exports: {} };
vm.runInNewContext(result.outputText, { module, exports: module.exports, Uint8Array, atob, Error, Number, Boolean, RegExp }, { filename: source, timeout: 5000 });
const reconcile = module.exports.reconcileTerminalChunk;
const frame = (text, from, options = {}) => ({
  cursor: from + Buffer.byteLength(text), fromCursor: from, data: Buffer.from(text).toString('base64'),
  truncated: false, running: true, ...options,
});
const resultText = page => Buffer.from(page.bytes).toString('utf8');

test('overlapping polling and websocket frames append each output byte once', () => {
  const first = reconcile(frame('abcdef', 0), 0);
  assert.equal(resultText(first), 'abcdef');
  const overlapping = reconcile(frame('defghij', 3), first.cursor);
  assert.equal(resultText(overlapping), 'ghij');
  assert.equal(overlapping.cursor, 10);
  const replay = reconcile(frame('abcdef', 0), overlapping.cursor);
  assert.equal(replay.kind, 'ignore');
  const sameEnd = reconcile(frame('fghij', 5), overlapping.cursor);
  assert.equal(sameEnd.kind, 'apply');
  assert.equal(sameEnd.bytes.length, 0);
});

test('UTF-8 split across requests stays lossless with overlapping prefixes', () => {
  const utf = Buffer.from('你好世界');
  const first = { cursor: 5, fromCursor: 0, data: utf.subarray(0, 5).toString('base64'), truncated: false, running: true };
  const second = { cursor: utf.length, fromCursor: 3, data: utf.subarray(3).toString('base64'), truncated: false, running: false };
  const a = reconcile(first, 0);
  const b = reconcile(second, a.cursor);
  assert.deepEqual(Buffer.concat([Buffer.from(a.bytes), Buffer.from(b.bytes)]), utf);
});

test('buffer overflow reports a real gap exactly once', () => {
  const gap = reconcile(frame('world', 20, { truncated: true }), 6);
  assert.equal(gap.truncated, true);
  assert.equal(resultText(gap), 'world');
  const overlap = reconcile(frame('world!', 20, { truncated: true }), gap.cursor);
  assert.equal(overlap.truncated, false);
  assert.equal(resultText(overlap), '!');
});

test('unexplained missing bytes are rejected for replay', () => {
  assert.throws(() => reconcile(frame('later', 14), 7), /缺口/);
  const old = frame('old', 0);
  assert.equal(reconcile(old, 9).kind, 'ignore');
});

test('legacy server snapshots derive byte start without re-encoding Unicode', () => {
  const legacy = frame('你好!', 13);
  delete legacy.fromCursor;
  const first = reconcile(legacy, 13);
  assert.equal(resultText(first), '你好!');
  const overlap = reconcile(legacy, 16);
  assert.deepEqual([...overlap.bytes], [...Buffer.from('你好!').subarray(3)]);
});

test('malformed cursor, payload and inconsistent boundaries are refused', () => {
  assert.throws(() => reconcile(frame('a', 0, { fromCursor: -1 }), 0), /游标/);
  assert.throws(() => reconcile(frame('a', 0, { fromCursor: 0, cursor: 999 }), 0), /游标/);
  assert.throws(() => reconcile(frame('a', 0, { data: '@nope@' }), 0), /协议/);
  assert.throws(() => reconcile(frame('a', 0, { cursor: -1 }), 0), /协议/);
});

test('late running snapshots cannot resurrect a completed process', () => {
  const reconcileStatus = module.exports.reconcileTerminalStatus;
  const original = reconcileStatus(false, -1, { running: true, exitCode: -1 });
  assert.equal(original.running, true);
  const ended = reconcileStatus(original.ended, original.exitCode, { running: false, exitCode: 12 });
  assert.equal(ended.running, false);
  const late = reconcileStatus(ended.ended, ended.exitCode, { running: true, exitCode: -1 });
  assert.equal(late.running, false);
  assert.equal(late.exitCode, 12);
});
