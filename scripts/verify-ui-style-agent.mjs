import { readFile } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const contractDir = join(root, "contracts", "ui-style-agent", "v1");
const fixtureDir = join(contractDir, "fixtures");
const schemaPath = join(contractDir, "style-patch.schema.json");
const targetsPath = join(contractDir, "targets.json");
const propertiesPath = join(contractDir, "properties.json");
const fixtures = [
  "apply-chat-input.json",
  "reset-chat-input-colors.json",
  "inspect-chat.json",
  "apply-component-states.json",
];
const forbidden = ["javascript:", "expression(", "url(", "\u0000", "\r", "\n"];
const states = new Set([
  "default",
  "hover",
  "focus",
  "focus-visible",
  "active",
  "pressed",
  "disabled",
  "dark",
  "light",
]);
const platforms = new Set(["desktop", "mobile", "web"]);

function fail(message) {
  throw new Error(message);
}

function walk(value, path = "$") {
  if (typeof value === "string") {
    const lower = value.toLowerCase();
    for (const token of forbidden) {
      if (token && lower.includes(token)) fail(`${path} contains forbidden token ${token}`);
    }
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((item, index) => walk(item, `${path}[${index}]`));
    return;
  }
  if (value && typeof value === "object") {
    for (const [key, item] of Object.entries(value)) walk(item, `${path}.${key}`);
  }
}

function validateScope(scope, path) {
  if (scope === undefined) return;
  if (!scope || typeof scope !== "object" || Array.isArray(scope)) fail(`${path} must be an object`);
  if (scope.platform !== undefined && !platforms.has(scope.platform)) fail(`${path}.platform is invalid`);
  if (scope.page !== undefined && (typeof scope.page !== "string" || scope.page.length > 512)) fail(`${path}.page is invalid`);
  if (scope.surface !== undefined && (typeof scope.surface !== "string" || scope.surface.length > 512)) fail(`${path}.surface is invalid`);
}

function validateStates(value, path) {
  if (value === undefined) return;
  if (!Array.isArray(value)) fail(`${path} must be an array`);
  for (const state of value) {
    if (typeof state !== "string" || !states.has(state)) fail(`${path} contains invalid state ${state}`);
  }
}

function validateProperties(value, path) {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).length === 0) {
    fail(`${path} must be a nonempty object`);
  }
  walk(value, path);
}

function validateRule(rule, path) {
  if (!rule || typeof rule !== "object" || Array.isArray(rule)) fail(`${path} must be an object`);
  if (typeof rule.target !== "string" || !rule.target.trim()) fail(`${path}.target is required`);
  if (rule.target.length > 512) fail(`${path}.target is too long`);
  validateScope(rule.scope, `${path}.scope`);
  validateStates(rule.states, `${path}.states`);
  const properties = rule.properties ?? rule.set;
  validateProperties(properties, `${path}.properties`);
  if (rule.source !== undefined && rule.source !== "ai" && rule.source !== "user") fail(`${path}.source is invalid`);
}

function validateReset(filter, path) {
  if (filter === undefined) return;
  if (!filter || typeof filter !== "object" || Array.isArray(filter)) fail(`${path} must be an object`);
  validateScope(filter.scope, `${path}.scope`);
  validateStates(filter.states, `${path}.states`);
  if (filter.target !== undefined && (typeof filter.target !== "string" || filter.target.length > 512)) fail(`${path}.target is invalid`);
  if (filter.properties !== undefined && !Array.isArray(filter.properties)) fail(`${path}.properties must be an array`);
  if (filter.source !== undefined && filter.source !== "ai" && filter.source !== "user") fail(`${path}.source is invalid`);
}

async function main() {
  const schema = JSON.parse(await readFile(schemaPath, "utf8"));
  if (schema.$schema !== "https://json-schema.org/draft/2020-12/schema") fail("schema draft is invalid");
  const targets = JSON.parse(await readFile(targetsPath, "utf8"));
  if (!Array.isArray(targets.targets) || targets.targets.length === 0) fail("targets.json must declare targets");
  const seenTargets = new Set();
  for (const target of targets.targets) {
    if (!target || typeof target.id !== "string" || !target.id.trim()) fail("targets.json contains an invalid target id");
    if (seenTargets.has(target.id)) fail(`targets.json contains duplicate target ${target.id}`);
    seenTargets.add(target.id);
    if (typeof target.kind !== "string" || !target.kind.trim()) fail(`target ${target.id} is missing kind`);
    if (typeof target.scope !== "string" || !target.scope.trim()) fail(`target ${target.id} is missing scope`);
  }
  const properties = JSON.parse(await readFile(propertiesPath, "utf8"));
  if (!properties.properties || typeof properties.properties !== "object" || Array.isArray(properties.properties)) {
    fail("properties.json must declare a property map");
  }
  if (!properties.stateSelectors || typeof properties.stateSelectors !== "object" || Array.isArray(properties.stateSelectors)) {
    fail("properties.json must declare state selectors");
  }
  for (const state of states) {
    if (typeof properties.stateSelectors[state] !== "string") fail(`properties.json is missing state ${state}`);
  }
  for (const fixture of fixtures) {
    const value = JSON.parse(await readFile(join(fixtureDir, fixture), "utf8"));
    if (value.targets !== undefined) {
      if (!Array.isArray(value.targets)) fail(`${fixture}.targets must be an array`);
      value.targets.forEach((rule, index) => validateRule(rule, `${fixture}.targets[${index}]`));
    } else if (value.target !== undefined && (value.properties !== undefined || value.set !== undefined)) {
      validateRule(value, fixture);
    } else if (value.target !== undefined && (typeof value.target !== "string" || value.target.length > 512)) {
      fail(`${fixture}.target is invalid`);
    }
    validateReset(value.reset, `${fixture}.reset`);
    walk(value, fixture);
  }
  process.stdout.write("ui style agent contract verification passed\n");
}

main().catch((error) => {
  process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
});
