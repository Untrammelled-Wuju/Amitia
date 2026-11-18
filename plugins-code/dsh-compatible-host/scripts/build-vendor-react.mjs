import { createRequire } from "node:module";
import { writeFileSync, mkdirSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";

const packageRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const vendorPath = join(packageRoot, "src", "runtime", "vendor", "react-browser.mjs");
const workRoot = join(packageRoot, ".vendor-work");
mkdirSync(workRoot, { recursive: true });
const entryPath = join(workRoot, "entry.mjs");
const bundlePath = join(workRoot, "bundle.cjs");

writeFileSync(
  entryPath,
  [
    'import * as React from "react";',
    'import * as ReactDOM from "react-dom";',
    'import * as ReactDOMClient from "react-dom/client";',
    'import * as JsxRuntime from "react/jsx-runtime";',
    "export const __react = React;",
    "export const __reactDom = ReactDOM;",
    "export const __reactDomClient = ReactDOMClient;",
    "export const __jsxRuntime = JsxRuntime;",
    "export default React;",
    "",
  ].join("\n"),
);

await build({
  entryPoints: [entryPath],
  outfile: bundlePath,
  bundle: true,
  format: "cjs",
  platform: "browser",
  target: ["chrome110"],
  define: { "process.env.NODE_ENV": '"production"' },
  logLevel: "silent",
});

const require = createRequire(import.meta.url);
const loaded = require(bundlePath);
const names = new Set();
for (const group of [loaded.__react, loaded.__reactDom, loaded.__reactDomClient, loaded.__jsxRuntime]) {
  if (!group || typeof group !== "object") continue;
  for (const key of Object.keys(group)) {
    if (key === "default" || key === "__esModule") continue;
    if (/^[A-Za-z_$][A-Za-z0-9_$]*$/.test(key)) names.add(key);
  }
}
const exportNames = [...names].sort();
if (!exportNames.includes("createElement") || !exportNames.includes("useState") || !exportNames.includes("createRoot")) {
  throw new Error(`react vendor bundle is missing required exports: ${exportNames.join(", ")}`);
}

const body = require("node:fs").readFileSync(bundlePath, "utf8");
const lines = [
  "const factory = () => {",
  "  const module = { exports: {} };",
  "  const exports = module.exports;",
  body
    .split("\n")
    .map((line) => (line.trim() === "" ? "" : `  ${line}`))
    .join("\n"),
  "  return module.exports;",
  "};",
  "const NS = globalThis.__AMITIA_REACT_VENDOR__ || (globalThis.__AMITIA_REACT_VENDOR__ = factory());",
  "",
];
for (const name of exportNames) lines.push(`export const ${name} = NS.${name};`);
lines.push("export default NS.default ?? NS;", "");
writeFileSync(vendorPath, `${lines.join("\n")}\n`);

rmSync(workRoot, { recursive: true, force: true });
console.log(`vendor=${vendorPath} exports=${exportNames.length}`);
