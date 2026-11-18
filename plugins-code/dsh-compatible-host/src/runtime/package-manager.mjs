import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { delimiter, dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

function text(value) {
  return value == null ? "" : String(value).trim();
}

function jsonRecord(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function packageEntry(exportsValue, subpath) {
  const exportsMap = jsonRecord(exportsValue);
  const raw = subpath ? exportsMap[subpath] : exportsMap["."];
  if (typeof raw === "string") return raw;
  const record = jsonRecord(raw);
  for (const key of ["import", "default", "node", "require"]) {
    if (typeof record[key] === "string") return record[key];
  }
  return "";
}

function packageEntries(exportsValue, packageJson) {
  const exportsMap = jsonRecord(exportsValue);
  const raw = exportsMap["."];
  const rawRecord = jsonRecord(raw);
  const entry = typeof raw === "string"
    ? raw
    : text(rawRecord.import || rawRecord.default || rawRecord.node || rawRecord.require);
  if (/\.(?:mjs|cjs|js)$/i.test(entry)) return [{ id: "root", entry }];
  const fallback = text(packageJson.module || packageJson.main);
  return fallback ? [{ id: "root", entry: fallback }] : [];
}

function packageClientEntries(exportsValue) {
  const exportsMap = jsonRecord(exportsValue);
  const entries = [];
  for (const [subpath, raw] of Object.entries(exportsMap)) {
    if (subpath !== "./client" && !subpath.endsWith("/client")) continue;
    const rawRecord = jsonRecord(raw);
    const entry = typeof raw === "string"
      ? raw
      : text(rawRecord.import || rawRecord.default || rawRecord.node || rawRecord.require);
    if (!/\.(?:mjs|cjs|js)$/i.test(entry)) continue;
    entries.push({
      id: subpath === "./client" ? "root" : subpath.slice(2, -"/client".length),
      entry,
    });
  }
  return entries;
}

export function resolvePluginDataRoot(environment = process.env) {
  const encoded = text(environment.AMITIA_HOST_FILESYSTEM_ROOTS_JSON);
  if (encoded) {
    const roots = JSON.parse(encoded);
    const root = text(jsonRecord(roots).plugin_data);
    if (root) return resolve(root);
  }
  const direct = text(environment.AMITIA_PLUGIN_DATA_DIR);
  if (direct) return resolve(direct);
  throw new Error("DSH 兼容宿主无法解析 plugin_data 目录");
}

export function resolveNpmCli(nodeExecutable = process.execPath, platform = process.platform) {
  const root = dirname(nodeExecutable);
  const candidates = platform === "win32"
    ? [
        join(root, "node_modules", "npm", "bin", "npm-cli.js"),
        join(root, "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"),
      ]
    : [
        join(root, "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"),
        join(root, "node_modules", "npm", "bin", "npm-cli.js"),
      ];
  const found = candidates.find((candidate) => existsSync(candidate));
  if (!found) throw new Error(`无法定位项目 npm CLI，已检查: ${candidates.join(", ")}`);
  return found;
}

function npmChildPermissionArgs(
  execArgv = process.execArgv,
  environment = process.env,
) {
  if (!execArgv.includes("--permission")) return [];
  return [
    "--permission",
    "--allow-fs-read=*",
    "--allow-fs-write=*",
    "--allow-child-process",
    ...(text(environment.AMITIA_HOST_NETWORK_MODE) === "host"
      ? ["--allow-net"]
      : []),
  ];
}

const HOST_PROVIDED_PACKAGES = new Set(["@deepseek-ai/cordis"]);
const PEER_INSTALL_PASSES = 4;

function childEnvironment(environment) {
  const nodeDirectory = dirname(process.execPath);
  const pathKey = Object.keys(environment).find((key) => key.toUpperCase() === "path") || "PATH";
  const current = text(environment[pathKey]);
  return { ...environment, [pathKey]: current ? `${nodeDirectory}${delimiter}${current}` : nodeDirectory };
}

async function runNpm(args, workspace, environment, paths) {
  const output = [];
  const exitCode = await new Promise((resolveExit, reject) => {
    const child = spawn(process.execPath, [
      ...npmChildPermissionArgs(process.execArgv, environment),
      paths.npmCli,
      ...args,
    ], {
      cwd: workspace,
      env: {
        ...childEnvironment(environment),
        HOME: workspace,
        USERPROFILE: workspace,
        TMPDIR: paths.npmTmp,
        TMP: paths.npmTmp,
        TEMP: paths.npmTmp,
        npm_config_cache: paths.npmCache,
        npm_config_logs_dir: paths.npmLogs,
        npm_config_tmp: paths.npmTmp,
        npm_config_update_notifier: "false",
      },
      windowsHide: true,
      stdio: ["ignore", "pipe", "pipe"],
    });
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (chunk) => output.push(String(chunk)));
    child.stderr.on("data", (chunk) => output.push(String(chunk)));
    child.once("error", (error) => reject(new Error(`启动 npm 失败: ${error.message}`, { cause: error })));
    child.once("close", (code, signal) => resolveExit({ code, signal }));
  });
  return { log: output.join(""), code: exitCode.code, signal: exitCode.signal };
}

async function installedPackageNames(nodeModules) {
  const names = [];
  let entries;
  try {
    entries = await readdir(nodeModules, { withFileTypes: true });
  } catch (error) {
    if (error?.code === "ENOENT") return names;
    throw error;
  }
  for (const entry of entries) {
    if (!entry.isDirectory() || entry.name.startsWith(".")) continue;
    if (!entry.name.startsWith("@")) {
      names.push(entry.name);
      continue;
    }
    for (const child of await readdir(join(nodeModules, entry.name), { withFileTypes: true })) {
      if (child.isDirectory()) names.push(`${entry.name}/${child.name}`);
    }
  }
  return names;
}

async function missingPeerSpecs(nodeModules) {
  const installed = new Set(await installedPackageNames(nodeModules));
  const missing = new Map();
  for (const name of installed) {
    if (HOST_PROVIDED_PACKAGES.has(name)) continue;
    const manifestPath = join(nodeModules, ...name.split("/"), "package.json");
    let manifest;
    try {
      manifest = JSON.parse(await readFile(manifestPath, "utf8"));
    } catch (error) {
      if (error?.code === "ENOENT") continue;
      throw error;
    }
    for (const [peer, range] of Object.entries(jsonRecord(manifest.peerDependencies))) {
      if (HOST_PROVIDED_PACKAGES.has(peer) || installed.has(peer) || missing.has(peer)) continue;
      missing.set(peer, `${peer}@${text(range) || "latest"}`);
    }
  }
  return Array.from(missing.values());
}

export const TREE_DIRECTORY_NAME = "dsh-tree";

const CLIENT_INJECT_INSTALLED = new Set();

function isPathSpec(raw) {
  return /^(?:\.|\/|file:)/.test(raw) || /^[A-Za-z]:[\\/]/.test(raw);
}

export async function resolveDshInstallSpec(spec) {
  const raw = text(spec);
  if (!raw) throw new Error("DSH 包安装源不能为空");
  if (isPathSpec(raw)) {
    const location = resolve(raw.startsWith("file:") ? raw.slice(5) : raw);
    const manifest = JSON.parse(await readFile(join(location, "package.json"), "utf8"));
    const name = text(manifest.name);
    if (!name) throw new Error(`本地 DSH 包 ${location} 缺少 name`);
    return { name, requirement: `file:${location}` };
  }
  const body = raw.startsWith("npm:") ? raw.slice(4) : raw;
  const at = body.lastIndexOf("@");
  const name = text(at > 0 ? body.slice(0, at) : body);
  const range = text(at > 0 ? body.slice(at + 1) : "");
  if (!name) throw new Error(`无法从安装源 ${raw} 解析包名`);
  return { name, requirement: range || "latest" };
}

export function dshTreeRoot(pluginDataRoot) {
  return join(resolve(pluginDataRoot), TREE_DIRECTORY_NAME);
}

export async function runNpmInstall(spec, workspace, environment = process.env) {
  const npmCli = resolveNpmCli(process.execPath, process.platform);
  const npmRoot = join(workspace, ".amitia-npm");
  const npmCache = join(npmRoot, "cache");
  const npmLogs = join(npmRoot, "logs");
  const npmTmp = join(npmRoot, "tmp");
  await Promise.all([
    mkdir(npmCache, { recursive: true }),
    mkdir(npmLogs, { recursive: true }),
    mkdir(npmTmp, { recursive: true }),
  ]);
  await mkdir(workspace, { recursive: true });
  const { name, requirement } = await resolveDshInstallSpec(spec);
  const manifestPath = join(workspace, "package.json");
  let previous = null;
  try {
    previous = await readFile(manifestPath, "utf8");
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
  const manifest = previous
    ? JSON.parse(previous)
    : { name: "amitia-dsh-tree", private: true, version: "0.0.0", type: "module" };
  manifest.dependencies = { ...jsonRecord(manifest.dependencies), [name]: requirement };
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  const paths = { npmCli, npmCache, npmLogs, npmTmp };
  const installArgs = ["install", "--install-links", "--legacy-peer-deps", "--no-audit", "--no-fund", "--loglevel=error", "--ignore-scripts"];
  const logs = [];
  try {
    const install = await runNpm(installArgs, workspace, environment, paths);
    logs.push(install.log);
    if (install.code !== 0) {
      const error = new Error(`DSH 包安装失败，exit=${install.code ?? "null"} signal=${install.signal ?? ""}\n${install.log}`, {
        cause: install,
      });
      error.log = install.log;
      throw error;
    }
    for (let pass = 0; pass < PEER_INSTALL_PASSES; pass += 1) {
      const peers = await missingPeerSpecs(join(workspace, "node_modules"));
      if (peers.length === 0) break;
      const peerInstall = await runNpm(["install", ...peers, ...installArgs.slice(1)], workspace, environment, paths);
      logs.push(peerInstall.log);
      if (peerInstall.code !== 0) {
        const error = new Error(`DSH 包依赖安装失败，exit=${peerInstall.code ?? "null"} signal=${peerInstall.signal ?? ""} peers=${peers.join(" ")}\n${peerInstall.log}`, {
          cause: peerInstall,
        });
        error.log = peerInstall.log;
        throw error;
      }
    }
  } catch (error) {
    if (previous === null) await rm(manifestPath, { force: true });
    else await writeFile(manifestPath, previous);
    throw error;
  }
  const installedManifest = JSON.parse(await readFile(join(workspace, "node_modules", name, "package.json"), "utf8"));
  const injectList = Array.isArray(installedManifest?.dsh?.client?.inject) ? installedManifest.dsh.client.inject : [];
  const injectLogs = [];
  for (const injectName of injectList) {
    const dependency = text(injectName);
    if (!dependency || CLIENT_INJECT_INSTALLED.has(dependency) || dependency === name) continue;
    CLIENT_INJECT_INSTALLED.add(dependency);
    const nested = await runNpmInstall(`${dependency}@${text(installedManifest.version) || "latest"}`, workspace, environment);
    injectLogs.push(nested.log);
  }
  return { log: logs.join("") + injectLogs.join(""), packageName: name };
}

export async function runNpmCi(workspace, environment = process.env) {
  const npmCli = resolveNpmCli(process.execPath, process.platform);
  const npmRoot = join(workspace, ".amitia-npm");
  const paths = {
    npmCli,
    npmCache: join(npmRoot, "cache"),
    npmLogs: join(npmRoot, "logs"),
    npmTmp: join(npmRoot, "tmp"),
  };
  await Promise.all([
    mkdir(paths.npmCache, { recursive: true }),
    mkdir(paths.npmLogs, { recursive: true }),
    mkdir(paths.npmTmp, { recursive: true }),
  ]);
  const args = ["ci", "--install-links", "--legacy-peer-deps", "--no-audit", "--no-fund", "--loglevel=error", "--ignore-scripts"];
  const result = await runNpm(args, workspace, environment, paths);
  if (result.code !== 0) {
    const error = new Error(`DSH 子包按快照恢复失败，exit=${result.code ?? "null"} signal=${result.signal ?? ""}\n${result.log}`, {
      cause: result,
    });
    error.log = result.log;
    throw error;
  }
  return { log: result.log };
}

export async function runNpmInstallPinned(pins, workspace, environment = process.env) {
  const npmCli = resolveNpmCli(process.execPath, process.platform);
  const npmRoot = join(workspace, ".amitia-npm");
  const npmCache = join(npmRoot, "cache");
  const npmLogs = join(npmRoot, "logs");
  const npmTmp = join(npmRoot, "tmp");
  await Promise.all([
    mkdir(npmCache, { recursive: true }),
    mkdir(npmLogs, { recursive: true }),
    mkdir(npmTmp, { recursive: true }),
  ]);
  await mkdir(workspace, { recursive: true });
  const manifestPath = join(workspace, "package.json");
  let previous = null;
  try {
    previous = await readFile(manifestPath, "utf8");
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
  const manifest = previous
    ? JSON.parse(previous)
    : { name: "amitia-dsh-tree", private: true, version: "0.0.0", type: "module" };
  manifest.dependencies = { ...pins };
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  const paths = { npmCli, npmCache, npmLogs, npmTmp };
  const args = ["install", "--install-links", "--legacy-peer-deps", "--no-audit", "--no-fund", "--loglevel=error", "--ignore-scripts"];
  try {
    const install = await runNpm(args, workspace, environment, paths);
    if (install.code !== 0) {
      const error = new Error(`DSH 子包按锁恢复失败，exit=${install.code ?? "null"} signal=${install.signal ?? ""}\n${install.log}`, {
        cause: install,
      });
      error.log = install.log;
      throw error;
    }
    return { log: install.log };
  } catch (error) {
    if (previous === null) await rm(manifestPath, { force: true });
    else await writeFile(manifestPath, previous);
    throw error;
  }
}

export async function describeDshPackage(packageDir, source = "") {
  const root = resolve(packageDir);
  const packageJsonPath = join(root, "package.json");
  const packageJson = JSON.parse(await readFile(packageJsonPath, "utf8"));
  const name = text(packageJson.name);
  const version = text(packageJson.version);
  if (!name) throw new Error(`DSH 包 ${packageJsonPath} 缺少 name`);
  if (!version) throw new Error(`DSH 包 ${name} 缺少 version`);
  let dsh = jsonRecord(packageJson.dsh);
  if (Object.keys(dsh).length === 0 && name.startsWith("@deepseek-ai/dsh-")) {
    dsh = { manifestVersion: 1, official: true };
  }
  if (Object.keys(dsh).length === 0) throw new Error(`包 ${name} 不是 DSH 插件，缺少 package.json.dsh`);
  const exportsValue = packageJson.exports;
  const entries = packageEntries(exportsValue, packageJson);
  const entry = text(entries[0]?.entry);
  const clientEntries = packageClientEntries(exportsValue);
  const clientEntry = text(clientEntries[0]?.entry || packageEntry(exportsValue, "./client"));
  if (!entry && !clientEntry) throw new Error(`DSH 包 ${name} 缺少主入口`);
  return {
    pluginId: name,
    name: text(packageJson.displayName) || name,
    version,
    entry,
    entries,
    clientEntry,
    clientEntries,
    source,
    enabled: false,
    installedAt: new Date().toISOString(),
    mappedContributions: [],
    packageDir: root,
    dsh,
  };
}

export async function writeDshTreeLock(workspace, pluginDataRoot) {
  const dataRoot = resolve(pluginDataRoot, "dsh-data");
  const lockPath = join(dataRoot, "dsh-tree-lock.json");
  const lockSource = join(workspace, "package-lock.json");
  let lock = null;
  try {
    lock = JSON.parse(await readFile(lockSource, "utf8"));
  } catch (error) {
    if (error?.code !== "ENOENT") throw error;
  }
  const packages = [];
  const entries = jsonRecord(lock?.packages);
  for (const [key, value] of Object.entries(entries)) {
    const record = jsonRecord(value);
    const version = text(record.version);
    if (!key.startsWith("node_modules/") || !version) continue;
    const entry = { name: key.slice("node_modules/".length), version };
    if (record.optional === true) entry.optional = true;
    if (Array.isArray(record.os) && record.os.length > 0) entry.os = record.os.slice();
    if (Array.isArray(record.cpu) && record.cpu.length > 0) entry.cpu = record.cpu.slice();
    packages.push(entry);
  }
  const manifest = jsonRecord(lock?.dependencies);
  if (packages.length === 0) {
    for (const [name, value] of Object.entries(manifest)) {
      const record = jsonRecord(value);
      const version = text(record.version);
      if (version) packages.push({ name, version });
    }
  }
  packages.sort((left, right) => left.name.localeCompare(right.name));
  await mkdir(dataRoot, { recursive: true });
  if (lock) {
    await writeFile(join(dataRoot, "dsh-tree-package-lock.json"), `${JSON.stringify(lock, null, 2)}\n`, "utf8");
  }
  await writeFile(lockPath, `${JSON.stringify({ generatedAt: new Date().toISOString(), packages }, null, 2)}\n`, "utf8");
  return lockPath;
}

export async function installDshPackage(spec, options = {}) {
  const source = text(spec);
  if (!source) throw new Error("DSH 包安装源不能为空");
  const pluginDataRoot = resolve(options.pluginDataRoot || resolvePluginDataRoot(options.environment));
  const workspace = dshTreeRoot(pluginDataRoot);
  const install = typeof options.runInstall === "function" ? options.runInstall : runNpmInstall;
  await mkdir(workspace, { recursive: true });
  const installResult = await install(source, workspace, options.environment || process.env);
  const expectedName = text(installResult?.packageName);
  if (!expectedName) throw new Error(`DSH 包安装未返回包名: ${source}`);
  const packageDir = join(workspace, "node_modules", ...expectedName.split("/"));
  const described = await describeDshPackage(packageDir, source);
  if (described.pluginId !== expectedName) {
    throw new Error(`DSH 包安装结果与目标包名不一致: 期望 ${expectedName}，实际 ${described.pluginId}`);
  }
  const boundary = relative(workspace, described.packageDir);
  if (!boundary || boundary === ".." || boundary.startsWith(`..${sep}`)) {
    throw new Error(`DSH 包安装目录越过共享安装树边界: ${described.packageDir}`);
  }
  await writeDshTreeLock(workspace, pluginDataRoot);
  return { ...described, installRoot: workspace, lockPath: join(resolve(pluginDataRoot, "dsh-data"), "dsh-tree-lock.json") };
}

export async function restoreDshTreeLock(options = {}) {
  const pluginDataRoot = resolve(options.pluginDataRoot || resolvePluginDataRoot(options.environment));
  const workspace = dshTreeRoot(pluginDataRoot);
  const lockPath = join(resolve(pluginDataRoot, "dsh-data"), "dsh-tree-lock.json");
  const expected = JSON.parse(await readFile(lockPath, "utf8"));
  const manifestPath = join(workspace, "package.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  const direct = Object.keys(jsonRecord(manifest.dependencies));
  if (direct.length === 0) {
    throw new Error(`DSH 安装树缺少直接依赖，无法按锁恢复: ${manifestPath}`);
  }
  const versions = new Map();
  const optionalNames = new Set();
  for (const entry of Array.isArray(expected?.packages) ? expected.packages : []) {
    const name = text(entry?.name);
    const version = text(entry?.version);
    if (!name || !version) continue;
    versions.set(name, version);
    if (entry?.optional === true) optionalNames.add(name);
  }
  const pins = {};
  for (const name of direct) {
    const version = versions.get(name);
    if (!version) throw new Error(`子包锁缺少直接依赖 ${name} 的版本记录: ${lockPath}`);
    pins[name] = version;
  }
  const lockSnapshots = join(resolve(pluginDataRoot, "dsh-data"), "dsh-tree-package-lock.json");
  const install = typeof options.runInstall === "function"
    ? options.runInstall
    : (async (pinned) => {
      let snapshot = null;
      try {
        snapshot = await readFile(lockSnapshots, "utf8");
      } catch (error) {
        if (error?.code !== "ENOENT") throw error;
      }
      if (snapshot === null) {
        await runNpmInstallPinned(pinned, workspace, options.environment || process.env);
        return;
      }
      const targetLock = join(workspace, "package-lock.json");
      let previousLock = null;
      try {
        previousLock = await readFile(targetLock, "utf8");
      } catch (error) {
        if (error?.code !== "ENOENT") throw error;
      }
      await writeFile(targetLock, snapshot);
      try {
        await runNpmCi(workspace, options.environment || process.env);
      } catch (error) {
        if (previousLock === null) await rm(targetLock, { force: true });
        else await writeFile(targetLock, previousLock);
        throw error;
      }
    });
  await install(pins, workspace, options.environment || process.env);
  const mismatched = [];
  for (const [name, version] of versions) {
    const packageJsonPath = join(workspace, "node_modules", ...name.split("/"), "package.json");
    let actual = "";
    try {
      actual = text(JSON.parse(await readFile(packageJsonPath, "utf8")).version);
    } catch (error) {
      if (error?.code !== "ENOENT") throw error;
    }
    if (actual === version) continue;
    if (actual === "" && optionalNames.has(name)) continue;
    mismatched.push({ name, expected: version, actual });
  }
  const lockPathAfter = await writeDshTreeLock(workspace, pluginDataRoot);
  return { pins, mismatched, lockPath: lockPathAfter };
}

export async function readDshClientSource(record, entryId = "root") {
  const packageDir = text(record?.packageDir);
  const requested = text(entryId) || "root";
  const clientEntries = Array.isArray(record?.clientEntries) && record.clientEntries.length
    ? record.clientEntries
    : [{ id: "root", entry: record?.clientEntry }];
  const selected = clientEntries.find((item) => text(item.id) === requested);
  const clientEntry = text(selected?.entry);
  if (!packageDir) throw new Error(`DSH 插件 ${text(record?.pluginId)} 缺少 packageDir`);
  if (!clientEntry) throw new Error(`DSH 插件 ${text(record?.pluginId)} 没有 client 入口`);
  const root = resolve(packageDir);
  const file = resolve(root, clientEntry);
  const boundary = relative(root, file);
  if (boundary === ".." || boundary.startsWith(`..${sep}`) || boundary.includes(`..${sep}`)) {
    throw new Error(`DSH client 入口越界: ${clientEntry}`);
  }
  const code = await readFile(file, "utf8");
  if (record?.plain === true && !/^\s*(?:import|export)\b/m.test(code)) {
    throw new Error(
      `共享依赖 ${text(record.pluginId)} 的 ${clientEntry} 不是 ESM，浏览器无法直接加载；请为该依赖提供打包期 ESM vendor（参考 src/runtime/vendor/react-browser.mjs）`,
    );
  }
  return {
    pluginId: text(record.pluginId),
    path: file,
    code,
  };
}

export async function removeDshPackageInstallation(record, environment = process.env) {
  const installRoot = text(record?.installRoot);
  const pluginId = text(record?.pluginId);
  if (!installRoot || !pluginId) return { removed: false };
  const pluginDataRoot = resolvePluginDataRoot(environment);
  const workspace = dshTreeRoot(pluginDataRoot);
  const target = resolve(installRoot);
  if (target !== workspace) {
    throw new Error(`DSH 插件 ${pluginId} 的安装树不是共享安装树，拒绝删除: ${target}`);
  }
  const nodeModules = join(workspace, "node_modules");
  const packageDir = text(record?.packageDir) ? resolve(record.packageDir) : join(nodeModules, ...pluginId.split("/"));
  const boundary = relative(nodeModules, packageDir);
  if (!boundary || boundary === ".." || boundary.startsWith(`..${sep}`)) {
    throw new Error(`拒绝删除共享安装树之外的目录: ${packageDir}`);
  }
  const manifestPath = join(workspace, "package.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  const dependencies = jsonRecord(manifest.dependencies);
  if (Object.prototype.hasOwnProperty.call(dependencies, pluginId)) {
    delete dependencies[pluginId];
    await writeFile(manifestPath, `${JSON.stringify({ ...manifest, dependencies }, null, 2)}\n`);
  }
  await rm(packageDir, { recursive: true, force: true });
  return { removed: true, path: packageDir };
}

async function resolveInstalledPackageDir(fromDir, name, boundary) {
  let current = resolve(fromDir);
  const limit = resolve(boundary);
  while (true) {
    const candidate = join(current, "node_modules", ...name.split("/"));
    if (existsSync(join(candidate, "package.json"))) return candidate;
    const parent = dirname(current);
    if (parent === current) return "";
    if (relative(limit, parent).startsWith("..")) return "";
    current = parent;
  }
}

function clientRequireSpecifiers(code) {
  const specifiers = new Set();
  const pattern = /require\(\s*["'`]([^"'`]+)["'`]\s*\)/g;
  let match;
  while ((match = pattern.exec(code)) !== null) {
    const specifier = text(match[1]);
    if (!specifier || specifier.startsWith(".") || specifier.startsWith("/")) continue;
    specifiers.add(specifier);
  }
  return Array.from(specifiers);
}

function packageNameOfSpecifier(specifier) {
  const value = text(specifier);
  const segments = value.split("/");
  if (segments[0].startsWith("@")) return segments.slice(0, 2).join("/");
  return segments[0];
}

export async function listClientDependencyPackages(record, options = {}) {
  const packageDir = text(record?.packageDir);
  const clientEntry = text(record?.clientEntry);
  if (!packageDir || !clientEntry) return [];
  const root = resolve(packageDir);
  const boundary = resolve(text(options.installRoot) || text(record?.installRoot) || join(root, "..", ".."));
  const entryPath = resolve(root, clientEntry);
  const entryBoundary = relative(root, entryPath);
  if (entryBoundary === ".." || entryBoundary.startsWith(`..${sep}`)) {
    throw new Error(`DSH client 入口越界: ${clientEntry}`);
  }
  const code = await readFile(entryPath, "utf8");
  const entryManifest = JSON.parse(await readFile(join(root, "package.json"), "utf8"));
  const clientInjectSpecifiers = Array.isArray(entryManifest?.dsh?.client?.inject) ? entryManifest.dsh.client.inject : [];
  const result = [];
  for (const specifier of [...clientRequireSpecifiers(code), ...clientInjectSpecifiers]) {
    const name = packageNameOfSpecifier(specifier);
    if (!name || name === text(record?.pluginId)) continue;
    const dir = await resolveInstalledPackageDir(root, name, boundary);
    if (!dir) continue;
    const depManifest = JSON.parse(await readFile(join(dir, "package.json"), "utf8"));
    const clientEntries = packageClientEntries(depManifest.exports);
    const dependencyEntry = text(clientEntries[0]?.entry || packageEntry(depManifest.exports, "./client"));
    if (!dependencyEntry) continue;
    result.push({
      pluginId: text(depManifest.name) || name,
      clientEntry: dependencyEntry,
      clientEntries: clientEntries.length > 0 ? clientEntries : [{ id: "root", entry: dependencyEntry }],
      typeOnly: dependencyEntry.includes("/types/"),
    });
  }
  return result;
}

async function describePlainClientPackage(packageDir, source) {
  const root = resolve(packageDir);
  const packageJson = JSON.parse(await readFile(join(root, "package.json"), "utf8"));
  const name = text(packageJson.name);
  if (!name) throw new Error(`共享依赖 ${root} 缺少 name`);
  const entries = packageEntries(packageJson.exports, packageJson);
  if (entries.length === 0) throw new Error(`共享依赖 ${name} 没有可加载的入口`);
  return {
    pluginId: name,
    version: text(packageJson.version),
    packageDir: root,
    entry: entries[0].entry,
    clientEntry: entries[0].entry,
    entries,
    clientEntries: entries,
    enabled: true,
    plain: true,
    source: text(source),
  };
}

const VENDOR_REACT_PACKAGES = new Set(["react", "react-dom", "react-dom/client", "react/jsx-runtime", "react/jsx-dev-runtime"]);

export async function readInstalledClientSource(pluginId, entryId, pluginDataRoot) {
  const requested = text(pluginId);
  if (VENDOR_REACT_PACKAGES.has(requested)) {
    const vendorPath = join(dirname(fileURLToPath(import.meta.url)), "vendor", "react-browser.mjs");
    return {
      pluginId: requested,
      path: vendorPath,
      code: await readFile(vendorPath, "utf8"),
      installedOnly: true,
      vendor: true,
    };
  }
  const tree = dshTreeRoot(pluginDataRoot);
  const dir = await resolveInstalledPackageDir(tree, pluginId, tree);
  if (!dir) throw new Error(`DSH 包 ${pluginId} 不在共享依赖树中`);
  let record;
  try {
    record = await describeDshPackage(dir, pluginId);
  } catch (error) {
    if (!String(error?.message ?? "").includes("不是 DSH 插件")) throw error;
    record = await describePlainClientPackage(dir, pluginId);
  }
  const source = await readDshClientSource(record, entryId);
  return { ...source, installedOnly: true };
}
