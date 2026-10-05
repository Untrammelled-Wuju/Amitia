const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { pathToFileURL, fileURLToPath } = require('node:url');
const { registerHooks } = require('node:module');
const { createHash } = require('node:crypto');
const ts = require('typescript');

const loaderPath = path.resolve(__dirname, '../src/task-loader.ts');
const loaderURL = pathToFileURL(loaderPath).href;
const loaderHook = registerHooks({
  load(url, context, next) {
    if (url !== loaderURL) return next(url, context);
    const source = ts.transpileModule(fs.readFileSync(loaderPath, 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.ES2022, target: ts.ScriptTarget.ES2022 },
    }).outputText;
    return { format: 'module', source, shortCircuit: true };
  },
});
const loader = import(loaderURL).finally(() => loaderHook.deregister());
const hash = value => 'sha256:' + createHash('sha256').update(value).digest('hex');

test('loads only the exact pinned CommonJS and ESM entry bytes', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-loader-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const { loadTaskHandler } = await loader;
  for (const [extension, source] of [
    ['cjs', 'module.exports=async()=>({success:true,output:"pinned"});'],
    ['mjs', 'export default async()=>({success:true,output:"pinned"});'],
  ]) {
    const entry = path.join(root, 'task.' + extension);
    fs.writeFileSync(entry, source);
    const handler = await loadTaskHandler(entry, hash(source));
    assert.equal((await handler({}, {})).output, 'pinned');
    await assert.rejects(loadTaskHandler(entry, ''), /完整性校验/);
    fs.writeFileSync(entry, source + '\nthrow new Error("changed");');
    await assert.rejects(loadTaskHandler(entry, hash(source)), /源码已变化/);
  }
});

test('a file replacement during import cannot replace the already verified source', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-loader-race-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const { loadTaskHandler } = await loader;
  const entry = path.join(root, 'task.mjs');
  const source = 'export default async()=>({success:true,output:"original"});';
  fs.writeFileSync(entry, source);
  let replaced = false;
  const replacementHook = registerHooks({
    load(url, context, next) {
      if (url === pathToFileURL(entry).href) {
        fs.writeFileSync(fileURLToPath(url), 'export default async()=>({success:true,output:"replacement"});');
        replaced = true;
      }
      return next(url, context);
    },
  });
  t.after(() => replacementHook.deregister());
  const handler = await loadTaskHandler(entry, hash(source));
  assert.equal(replaced, true);
  assert.equal((await handler({}, {})).output, 'original');
});
