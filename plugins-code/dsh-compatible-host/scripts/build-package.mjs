import { createHash } from "node:crypto";
import {
  copyFileSync,
  cpSync,
  mkdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";
import { buildPackage, inspectPackage } from "../../../sdk/plugin-cli/dist/archive.js";
import { applyMobileContributionActions } from "./mobile-contributions.mjs";
import { execFileSync } from "node:child_process";

const packageRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const repositoryRoot = resolve(packageRoot, "..", "..");
const stagingRoot = join(packageRoot, ".package-staging");
const outputRoot = join(repositoryRoot, "plugin");
const manifest = JSON.parse(readFileSync(join(packageRoot, "amitia-extension.json"), "utf8"));
const outputPath = join(outputRoot, `amitia-dsh-compatible-host-${manifest.plugin.version}.amitiax`);
const temporaryPath = `${outputPath}.tmp`;

function browserHash(path) {
  return `sha256-${createHash("sha256").update(readFileSync(path)).digest("base64")}`;
}

execFileSync(process.execPath, [join(packageRoot, "scripts", "build-vendor-react.mjs")], { stdio: "inherit" });
rmSync(stagingRoot, { recursive: true, force: true });
rmSync(temporaryPath, { force: true });
mkdirSync(join(stagingRoot, "modules", "dsh-compatible-runtime", "dist"), { recursive: true });
mkdirSync(join(stagingRoot, "modules", "dsh-compatible-ui", "desktop"), { recursive: true });
mkdirSync(join(stagingRoot, "modules", "dsh-compatible-ui", "mobile"), { recursive: true });
mkdirSync(join(stagingRoot, "modules", "dsh-compatible-mobile-client"), { recursive: true });
mkdirSync(outputRoot, { recursive: true });

copyFileSync(join(packageRoot, "src", "runtime", "index.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "index.js"));
copyFileSync(join(packageRoot, "src", "runtime", "abi.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "abi.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "state.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "state.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "package-manager.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "package-manager.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "host-abi.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "host-abi.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "errors.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "errors.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "tool-mapping.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "tool-mapping.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "host-services.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "host-services.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "host-service-bridges.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "host-service-bridges.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "schedule-adapter.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "schedule-adapter.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "catalog-summary.generated.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "catalog-summary.generated.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "lifecycle-events.generated.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "lifecycle-events.generated.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "session-events.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "session-events.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "provider-wiring.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "provider-wiring.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "model-adapter.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "model-adapter.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "diagnose.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "diagnose.mjs"));
copyFileSync(join(packageRoot, "src", "runtime", "approval-bridge.mjs"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "approval-bridge.mjs"));
cpSync(join(packageRoot, "src", "runtime", "compat"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "compat"), { recursive: true });
cpSync(join(packageRoot, "src", "runtime", "vendor"), join(stagingRoot, "modules", "dsh-compatible-runtime", "dist", "vendor"), { recursive: true });
cpSync(join(packageRoot, "catalog"), join(stagingRoot, "modules", "dsh-compatible-runtime", "catalog"), { recursive: true });
const shellBuild = await build({
  entryPoints: [join(packageRoot, "src", "client-ui", "shell.mjs")],
  outfile: join(stagingRoot, "modules", "dsh-compatible-ui", "desktop", "shell.mjs"),
  bundle: true,
  format: "esm",
  platform: "browser",
  target: ["chrome110"],
  sourcemap: false,
  minify: false,
  write: false,
  define: {
    "process.env.NODE_ENV": "\"production\"",
  },
  loader: {
    ".woff": "dataurl",
    ".woff2": "dataurl",
    ".ttf": "dataurl",
    ".png": "dataurl",
    ".svg": "dataurl",
    ".css": "css",
  },
});
const shellJavaScript = shellBuild.outputFiles.find((file) => file.path.endsWith(".mjs") || file.path.endsWith(".js"));
const shellCss = shellBuild.outputFiles.find((file) => file.path.endsWith(".css"));
if (!shellJavaScript) throw new Error("DSH client shell JavaScript output is missing");
const shellCssInjection = shellCss
  ? `const __dshShellStyle=document.createElement("style");__dshShellStyle.dataset.plugin="com.amitia/dsh-compatible-host";__dshShellStyle.textContent=${JSON.stringify(shellCss.text)};document.head.append(__dshShellStyle);\n`
  : "";
writeFileSync(join(stagingRoot, "modules", "dsh-compatible-ui", "desktop", "shell.mjs"), `${shellCssInjection}${shellJavaScript.text}`);
const clientBuild = await build({
  entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
  outfile: join(stagingRoot, "modules", "dsh-compatible-ui", "desktop", "index.mjs"),
  bundle: true,
  format: "esm",
  platform: "browser",
  target: ["chrome110"],
  sourcemap: false,
  minify: false,
  write: false,
  define: {
    "process.env.NODE_ENV": "\"production\"",
  },
  loader: {
    ".woff": "dataurl",
    ".woff2": "dataurl",
    ".ttf": "dataurl",
    ".png": "dataurl",
    ".svg": "dataurl",
    ".css": "css",
  },
});
const clientJavaScript = clientBuild.outputFiles.find((file) => file.path.endsWith(".mjs") || file.path.endsWith(".js"));
const clientCss = clientBuild.outputFiles.find((file) => file.path.endsWith(".css"));
if (!clientJavaScript) throw new Error("DSH client UI JavaScript output is missing");
if (clientCss) {
  const cssInjection = `const __dshStyle=document.createElement("style");__dshStyle.dataset.plugin="com.amitia/dsh-compatible-host";__dshStyle.textContent=${JSON.stringify(clientCss.text)};document.head.append(__dshStyle);\n`;
  writeFileSync(join(stagingRoot, "modules", "dsh-compatible-ui", "desktop", "index.mjs"), `${cssInjection}${clientJavaScript.text}`);
} else {
  writeFileSync(join(stagingRoot, "modules", "dsh-compatible-ui", "desktop", "index.mjs"), clientJavaScript.text);
}
const mobileBuild = await build({
  entryPoints: [join(packageRoot, "src", "mobile-ui", "index.mjs")],
  outfile: join(stagingRoot, "modules", "dsh-compatible-ui", "mobile", "index.mjs"),
  bundle: true,
  format: "esm",
  platform: "browser",
  target: ["chrome110"],
  sourcemap: false,
  minify: false,
  write: false,
});
const mobileJavaScript = mobileBuild.outputFiles.find((file) => file.path.endsWith(".mjs") || file.path.endsWith(".js"));
if (!mobileJavaScript) throw new Error("DSH mobile UI JavaScript output is missing");
writeFileSync(join(stagingRoot, "modules", "dsh-compatible-ui", "mobile", "index.mjs"), mobileJavaScript.text);
copyFileSync(join(packageRoot, "src", "mobile-ui", "mobile.css"), join(stagingRoot, "modules", "dsh-compatible-ui", "mobile", "index.css"));
const mobileClientBuild = await build({
  entryPoints: [join(packageRoot, "src", "mobile-client-ui", "index.mjs")],
  outfile: join(stagingRoot, "modules", "dsh-compatible-mobile-client", "index.mjs"),
  bundle: true,
  format: "esm",
  platform: "browser",
  target: ["chrome110"],
  sourcemap: false,
  minify: false,
  write: false,
  define: {
    "process.env.NODE_ENV": "\"production\"",
  },
  loader: {
    ".woff": "dataurl",
    ".woff2": "dataurl",
    ".ttf": "dataurl",
    ".png": "dataurl",
    ".svg": "dataurl",
    ".css": "css",
  },
});
const mobileClientJavaScript = mobileClientBuild.outputFiles.find((file) => file.path.endsWith(".mjs") || file.path.endsWith(".js"));
if (!mobileClientJavaScript) throw new Error("DSH mobile client UI JavaScript output is missing");
writeFileSync(join(stagingRoot, "modules", "dsh-compatible-mobile-client", "index.mjs"), mobileClientJavaScript.text);
copyFileSync(join(packageRoot, "src", "mobile-client-ui", "mobile-client.css"), join(stagingRoot, "modules", "dsh-compatible-mobile-client", "index.css"));
writeFileSync(join(stagingRoot, "modules", "dsh-compatible-runtime", "package.json"), `${JSON.stringify({ type: "module" }, null, 2)}\n`);
writeFileSync(join(stagingRoot, "modules", "dsh-compatible-ui", "package.json"), `${JSON.stringify({ type: "module" }, null, 2)}\n`);

for (const module of manifest.modules || []) {
  for (const contribution of module.contributions || []) {
    const entryPath = contribution.spec?.entry?.path;
    if (entryPath) {
      contribution.spec.entry.content_hash = browserHash(join(stagingRoot, entryPath));
      const stylePath = contribution.spec?.entry?.style_path;
      if (stylePath) contribution.spec.entry.style_hash = browserHash(join(stagingRoot, stylePath));
    }
    const schemaPath = contribution.spec?.entry?.schema_path;
    if (schemaPath) contribution.spec.entry.content_hash = browserHash(join(stagingRoot, schemaPath));
    const entries = contribution.spec?.entries;
    if (entries && typeof entries === "object") {
      for (const entry of Object.values(entries)) {
        if (!entry || typeof entry !== "object") continue;
        if (entry.path) entry.contentHash = browserHash(join(stagingRoot, entry.path));
        if (entry.stylePath) entry.styleHash = browserHash(join(stagingRoot, entry.stylePath));
      }
    }
  }
}
manifest.integrity.algorithm = "sha256";
manifest.integrity.contentTreeHash = "";
applyMobileContributionActions(manifest);
writeFileSync(join(stagingRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);

const inspection = buildPackage(stagingRoot, join(stagingRoot, "manifest.json"), temporaryPath);
inspectPackage(temporaryPath);
renameSync(temporaryPath, outputPath);
rmSync(stagingRoot, { recursive: true, force: true });

console.log(`package=${outputPath}`);
console.log(`treeHash=${inspection.treeHash}`);
