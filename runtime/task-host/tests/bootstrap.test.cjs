const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = fs.readFileSync(path.join(__dirname, '../src/bootstrap.ts'), 'utf8');
const code = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;

function config(generation) {
  const exportsObject = {};
  vm.runInNewContext(code, {
    exports: exportsObject,
    require: () => ({}),
    process: { env: generation === undefined ? {} : { AMITIA_GENERATION: generation } },
    Number, Error, JSON,
  });
  return exportsObject.readRuntimeConfig();
}

test('task runtime preserves the host generation on resumed processes', () => {
  assert.equal(config('7').generation, 7);
  assert.equal(config().generation, 1);
});

test('invalid generation refuses runtime startup', () => {
  for (const generation of ['0', '-1', '1.2', 'invalid', '', '9007199254740992']) {
    assert.throws(() => config(generation));
  }
});
