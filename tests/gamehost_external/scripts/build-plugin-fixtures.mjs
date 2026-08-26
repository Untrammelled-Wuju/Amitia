import { existsSync } from 'node:fs';
import { join, resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const here = dirname(fileURLToPath(import.meta.url));
const workspace = resolve(process.env.GITHUB_WORKSPACE || resolve(here, '../../..'));
const pluginDir = join(workspace, 'testplugins', 'mock-amitiax-game-plugin');
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';

function run(args, extraEnv = {}) {
  const result = spawnSync(npm, args, {
    cwd: pluginDir,
    env: { ...process.env, ...extraEnv },
    stdio: 'inherit',
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${npm} ${args.join(' ')} failed with exit code ${result.status}`);
}

run(['ci']);
run(['run', 'build']);
for (const fixture of [
  { output: 'mock-amitiax-game-plugin-v1.amitiax', version: '' },
  { output: 'mock-amitiax-game-plugin-v2.amitiax', version: '1.1.0' },
]) {
  const env = { MOCK_PLUGIN_OUTPUT: fixture.output };
  if (fixture.version) env.MOCK_PLUGIN_VERSION = fixture.version;
  run(['run', 'package'], env);
  run(['run', 'verify-package'], env);
  const outputPath = join(pluginDir, 'dist-package', fixture.output);
  if (!existsSync(outputPath)) throw new Error(`fixture package missing: ${outputPath}`);
}
