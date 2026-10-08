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
function treeHash(root) {
  const entries = [];
  const visit = file => {
    if (fs.statSync(file).isDirectory()) {
      for (const name of fs.readdirSync(file)) visit(path.join(file, name));
    } else {
      entries.push([path.relative(root, file).split(path.sep).join('/'), hash(fs.readFileSync(file)).slice(7)]);
    }
  };
  visit(root);
  const digest = createHash('sha256');
  for (const [name, value] of entries.sort((a, b) => Buffer.compare(Buffer.from(a[0]), Buffer.from(b[0])))) digest.update(name).update('\0').update(value).update('\0');
  return 'sha256:' + digest.digest('hex');
}

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

test('bundle pin validates static and dynamic CommonJS and ESM dependencies', async t => {
  const { loadTaskHandler } = await loader;
  for (const extension of ['cjs', 'mjs']) {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-'));
    t.after(() => fs.rmSync(root, { recursive: true, force: true }));
    const source = extension === 'cjs' ? "module.exports=async()=>({success:true,output:require('./dependency.cjs')});" : "export default async()=>({success:true,output:(await import('./dependency.mjs')).default});";
    const dependency = path.join(root, 'dependency.' + extension);
    const entry = path.join(root, 'task.' + extension);
    fs.writeFileSync(entry, source);
    fs.writeFileSync(dependency, extension === 'cjs' ? "module.exports='original';" : "export default 'original';");
    const pin = treeHash(root);
    const handler = await loadTaskHandler(entry, hash(source), root, pin);
    assert.equal((await handler({}, {})).output, 'original');
    fs.writeFileSync(dependency, 'changed');
    await assert.rejects(loadTaskHandler(entry, hash(source), root, pin), /文件树已变化/);
  }
});

test('dependency replacement after entry import is refused before execution', async t => {
  const { loadTaskHandler } = await loader;
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-change-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const source = "export default async()=>({success:true,output:(await import('./late.mjs')).default});";
  const entry = path.join(root, 'task.mjs');
  const dependency = path.join(root, 'late.mjs');
  fs.writeFileSync(entry, source);
  fs.writeFileSync(dependency, "export default 'original';");
  const handler = await loadTaskHandler(entry, hash(source), root, treeHash(root));
  fs.writeFileSync(dependency, "export default 'replacement';");
  await assert.rejects(handler({}, {}), /源码已变化/);
});

test('bundle modules cannot import files outside the pinned tree or newly added files', async t => {
  const { loadTaskHandler } = await loader;
  for (const outside of [true, false]) {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-boundary-'));
    const foreign = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-foreign-'));
    t.after(() => { fs.rmSync(root, { recursive: true, force: true }); fs.rmSync(foreign, { recursive: true, force: true }); });
    const dependency = path.join(outside ? foreign : root, 'extra.mjs');
    const source = `export default async()=>({success:true,output:(await import(${JSON.stringify(pathToFileURL(dependency).href)})).default});`;
    const entry = path.join(root, 'task.mjs');
    fs.writeFileSync(entry, source);
    const handler = await loadTaskHandler(entry, hash(source), root, treeHash(root));
    fs.writeFileSync(dependency, "export default 'foreign';");
    await assert.rejects(handler({}, {}), /超出固定插件目录|不在固定插件文件树/);
  }
});

test('dependency loader executes the same bytes it verifies during an import race', async t => {
  const { loadTaskHandler } = await loader;
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-load-race-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const entry = path.join(root, 'task.mjs');
  const dependency = path.join(root, 'dependency.mjs');
  const source = "export default async()=>({success:true,output:(await import('./dependency.mjs')).default});";
  fs.writeFileSync(entry, source);
  fs.writeFileSync(dependency, "export default 'original';");
  const replacementHook = registerHooks({load(url, context, next) {
    if (url === pathToFileURL(dependency).href) fs.writeFileSync(dependency, "export default 'replacement';");
    return next(url, context);
  }});
  t.after(() => replacementHook.deregister());
  const handler = await loadTaskHandler(entry, hash(source), root, treeHash(root));
  assert.equal((await handler({}, {})).output, 'original');
});

test('an already cached external module cannot bypass the bundle boundary', async t => {
  const { loadTaskHandler } = await loader;
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-cached-'));
  const foreign = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-cached-foreign-'));
  t.after(() => { fs.rmSync(root, { recursive: true, force: true }); fs.rmSync(foreign, { recursive: true, force: true }); });
  const dependency = path.join(foreign, 'cached.cjs');
  fs.writeFileSync(dependency, "module.exports='cached';");
  assert.equal(require(dependency), 'cached');
  const source = `module.exports=async()=>({success:true,output:require(${JSON.stringify(dependency)})});`;
  const entry = path.join(root, 'task.cjs');
  fs.writeFileSync(entry, source);
  const handler = await loadTaskHandler(entry, hash(source), root, treeHash(root));
  await assert.rejects(handler({}, {}), /超出固定插件目录/);
});

test('a directory link inserted after bundle verification cannot escape pinned modules', async t => {
  const { loadTaskHandler } = await loader;
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-link-'));
  const foreign = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-bundle-link-foreign-'));
  t.after(() => { fs.rmSync(root, { recursive: true, force: true }); fs.rmSync(foreign, { recursive: true, force: true }); });
  const modules = path.join(root, 'modules');
  fs.mkdirSync(modules);
  fs.writeFileSync(path.join(modules, 'dependency.mjs'), "export default 'original';");
  fs.writeFileSync(path.join(foreign, 'dependency.mjs'), "export default 'foreign';");
  const source = "export default async()=>({success:true,output:(await import('./modules/dependency.mjs')).default});";
  const entry = path.join(root, 'task.mjs');
  fs.writeFileSync(entry, source);
  const handler = await loadTaskHandler(entry, hash(source), root, treeHash(root));
  fs.renameSync(modules, path.join(root, 'original-modules'));
  fs.symlinkSync(foreign, modules, process.platform === 'win32' ? 'junction' : 'dir');
  await assert.rejects(handler({}, {}), /不允许符号链接|超出固定插件目录/);
});
