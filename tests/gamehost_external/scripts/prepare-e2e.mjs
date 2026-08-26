import { appendFileSync, mkdirSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const here = dirname(fileURLToPath(import.meta.url));
const workspace = resolve(process.env.GITHUB_WORKSPACE || resolve(here, '../../..'));
const runnerTemp = resolve(process.env.RUNNER_TEMP || join(workspace, '.gamehost-e2e-tmp'));
const e2eRoot = join(runnerTemp, 'gamehost-e2e');
const backendBin = join(runnerTemp, process.platform === 'win32' ? 'amitia-gamehost-e2e-server.exe' : 'amitia-gamehost-e2e-server');
const backendDir = join(workspace, 'backend');
const pluginDir = join(workspace, 'testplugins', 'mock-amitiax-game-plugin');
const nativePluginDir = join(workspace, 'testplugins', 'game-plugin-demo', 'go');
const restartScript = join(workspace, 'tests', 'gamehost_external', 'scripts', 'restart-backend.mjs');

for (const dir of [e2eRoot, join(e2eRoot, 'data'), join(e2eRoot, 'logs'), join(e2eRoot, 'cache'), join(e2eRoot, 'tmp')]) {
  mkdirSync(dir, { recursive: true });
}

const built = spawnSync('go', ['build', '-o', backendBin, './cmd/server'], {
  cwd: backendDir,
  env: process.env,
  stdio: 'inherit',
  shell: false,
});
if (built.error) throw built.error;
if (built.status !== 0) throw new Error(`go build failed with exit code ${built.status}`);

const values = {
  AMITIA_RUNTIME_ROOT: workspace,
  AMITIA_DATA_DIR: join(e2eRoot, 'data'),
  AMITIA_LOG_DIR: join(e2eRoot, 'logs'),
  AMITIA_CACHE_DIR: join(e2eRoot, 'cache'),
  AMITIA_TEMP_DIR: join(e2eRoot, 'tmp'),
  AMITIA_WORKSPACE_DIR: workspace,
  AMITIA_RUNTIME_PROFILE: 'local',
  AMITIA_JWT_SECRET: randomBytes(48).toString('hex'),
  AMITIA_LOCAL_TOKEN: randomBytes(32).toString('hex'),
  AMITIA_EXTENSION_DEV_MODE: 'true',
  AMITIA_VECTOR_STORE_ENABLED: 'false',
  AMITIA_GRAPH_STORE_ENABLED: 'false',
  AMITIA_TASK_HOST_ENABLED: 'false',
  AMITIA_WECHAT_SIDECAR_ENABLED: 'false',
  AMITIA_QQ_SIDECAR_ENABLED: 'false',
  AMITIA_DESKTOP_PET_RUNTIME_ENABLED: 'false',
  AMITIA_NODE_BIN: process.execPath,
  GAMEHOST_BACKEND_BIN: backendBin,
  GAMEHOST_BACKEND_PID_FILE: join(e2eRoot, 'backend.pid'),
  GAMEHOST_BACKEND_LOG: join(e2eRoot, 'backend.log'),
  GAMEHOST_SERVER_ROOT: 'http://127.0.0.1:18899',
  GAMEHOST_BASE_URL: 'http://127.0.0.1:18899/api',
  GAMEHOST_BACKEND_RESTART_SCRIPT: restartScript,
  GAMEHOST_BACKEND_CWD: workspace,
  GAMEHOST_DEV_WORKSPACE_PATH: pluginDir,
  GAMEHOST_NATIVE_DEV_WORKSPACE_PATH: nativePluginDir,
  MOCK_PLUGIN_ARCHIVE_PATH: join(pluginDir, 'dist-package', 'mock-amitiax-game-plugin-v1.amitiax'),
  MOCK_PLUGIN_ARCHIVE_PATH_V2: join(pluginDir, 'dist-package', 'mock-amitiax-game-plugin-v2.amitiax'),
  MOCK_PLUGIN_REQUIRED_ARTIFACT_ARCHIVE_PATH: join(pluginDir, 'dist-package', 'mock-amitiax-game-plugin-required.amitiax'),
  MOCK_PLUGIN_REQUIRED_ARTIFACT_ARCHIVE_PATH_V2: join(pluginDir, 'dist-package', 'mock-amitiax-game-plugin-required-v2.amitiax'),
  NATIVE_PLUGIN_ARCHIVE_PATH: join(nativePluginDir, 'dist-package', 'mock-game-plugin-go-v1.amitiax'),
  NATIVE_PLUGIN_ARCHIVE_PATH_V2: join(nativePluginDir, 'dist-package', 'mock-game-plugin-go-v2.amitiax'),
};

const githubEnv = process.env.GITHUB_ENV;
if (!githubEnv) throw new Error('GITHUB_ENV is required in CI');
for (const [key, value] of Object.entries(values)) appendFileSync(githubEnv, `${key}=${value}\n`);
process.stdout.write(`Prepared GameHost E2E for ${process.platform}/${process.arch}\n`);
