import { readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const catalogRoot = join(packageRoot, "catalog");

function argument(name, fallback) {
  const index = process.argv.indexOf(`--${name}`);
  if (index < 0) return fallback;
  const value = process.argv[index + 1];
  if (!value) throw new Error(`--${name} 缺少取值`);
  return value;
}

const modulesRoot = resolve(argument("modules", join(packageRoot, ".official-scan", "node_modules")));
const outputPath = resolve(argument("output", join(catalogRoot, "official-references.v1.json")));
const sourceLabel = argument("source", "");
const scanExtensions = [".js", ".mjs", ".cjs", ".ts", ".tsx", ".d.ts"];

const compatibility = JSON.parse(readFileSync(join(catalogRoot, "compatibility.v1.json"), "utf8"));
const catalogServices = compatibility.hostServices
  .map((entry) => entry.capability)
  .filter((capability) => typeof capability === "string" && capability.length > 0)
  .map((capability) => capability.replace(/^ctx\./, ""));
const catalogSet = new Set(catalogServices);

function walk(directory, files) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const target = join(directory, entry.name);
    if (entry.isSymbolicLink()) {
      const stats = statSync(target);
      if (stats.isDirectory()) walk(target, files);
      else if (isScannable(entry.name)) files.push(target);
      continue;
    }
    if (entry.isDirectory()) {
      if (entry.name === "node_modules") continue;
      walk(target, files);
      continue;
    }
    if (isScannable(entry.name)) files.push(target);
  }
  return files;
}

function isScannable(name) {
  return scanExtensions.some((extension) => name.endsWith(extension)) && !name.endsWith(".map");
}

function packageDirectories(root) {
  const directories = [];
  for (const scope of readdirSync(root, { withFileTypes: true })) {
    if (!scope.isDirectory() || !scope.name.startsWith("@")) continue;
    const scopePath = join(root, scope.name);
    for (const entry of readdirSync(scopePath, { withFileTypes: true })) {
      directories.push(join(scopePath, entry.name));
    }
  }
  return directories;
}

function readPackage(directory) {
  const manifest = JSON.parse(readFileSync(join(directory, "package.json"), "utf8"));
  return { name: manifest.name, version: manifest.version, directory };
}

const literalPattern = /["'`]([A-Za-z_$][\w$]*)["'`]/g;
const definitionPattern = /(?:super\(\s*[A-Za-z_$][\w$]*\s*,|\.(?:set|provide)\()\s*["'`]([A-Za-z_$][\w$]*)["'`]/g;
const propertyPattern = /\b(?:ctx|this\.ctx|context|root)\s*\.\s*([A-Za-z_$][\w$]*)/g;

const packages = packageDirectories(modulesRoot).map(readPackage);
const references = new Map();
for (const name of catalogServices) references.set(name, { literal: new Set(), property: new Set(), definition: new Set() });

function collect(service, packageName, kind) {
  references.get(service)?.[kind].add(packageName);
}

for (const entry of packages) {
  let files;
  try {
    files = walk(entry.directory, []);
  } catch (cause) {
    throw new Error(`扫描 ${entry.name} 目录失败: ${entry.directory}`, { cause });
  }
  for (const file of files) {
    const source = readFileSync(file, "utf8");
    for (const match of source.matchAll(literalPattern)) {
      if (catalogSet.has(match[1])) collect(match[1], entry.name, "literal");
    }
    for (const match of source.matchAll(propertyPattern)) {
      if (catalogSet.has(match[1])) collect(match[1], entry.name, "property");
    }
    for (const match of source.matchAll(definitionPattern)) {
      if (catalogSet.has(match[1])) collect(match[1], entry.name, "definition");
    }
  }
}

const services = {};
for (const name of catalogServices) {
  const entry = references.get(name);
  const literalReferences = Array.from(entry.literal).sort();
  const propertyReferences = Array.from(entry.property).sort();
  const definitionPackages = Array.from(entry.definition).sort();
  const consumers = Array.from(new Set([...literalReferences, ...propertyReferences])).sort();
  services[name] = {
    literalReferences,
    propertyReferences,
    definitionPackages,
    consumerCount: consumers.length,
    consumers,
  };
}

const zeroReferenceServices = Object.entries(services)
  .filter(([, value]) => value.consumerCount === 0)
  .map(([name]) => name)
  .sort();

const payload = {
  schemaVersion: 1,
  targetDshVersion: compatibility.targetDshVersion,
  source: sourceLabel || `@deepseek-ai 依赖闭包（${packages.length} 个包）`,
  methods: {
    literal: "包内代码中出现带引号的服务名字面量（覆盖 inject 数组、ctx.get\\(name\\)、super\\(ctx, name\\) 与服务名字符串）",
    property: "包内代码中出现 ctx.<name> / this.ctx.<name> / context.<name> 属性访问",
    definition: "包内代码中出现 super(<owned context>, \"<name>\") / set(\"<name>\") / provide(\"<name>\") 的定义点",
  },
  scannedPackageCount: packages.length,
  scannedPackages: packages.map((entry) => `${entry.name}@${entry.version}`).sort(),
  services,
  zeroReferenceServices,
};

writeFileSync(outputPath, `${JSON.stringify(payload, null, 2)}\n`);
console.log(`output=${outputPath}`);
console.log(`packages=${packages.length}`);
console.log(`services=${catalogServices.length} referenced=${catalogServices.length - zeroReferenceServices.length} zero=${zeroReferenceServices.length}`);
if (zeroReferenceServices.length) console.log(`zeroReferenceServices=${zeroReferenceServices.join(",")}`);
