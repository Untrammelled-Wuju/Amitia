import fs from "node:fs";
import path from "node:path";
import { buildPackage, inspectPackage } from "../../../sdk/plugin-cli/dist/archive.js";

const desktopPlatforms = new Set(["windows", "linux", "macos"]);
const desktopEntryKeys = new Set([
  "electron_windows",
  "electron_macos",
  "electron_linux",
  "web",
]);

function isMobileValue(value) {
  return /(?:^|[-_.:/])(mobile|android|ios)(?:$|[-_.:/])/i.test(String(value));
}

function sanitizeNavigationItems(items) {
  if (!Array.isArray(items)) return items;
  return items
    .map((item) => {
      if (!Array.isArray(item.platforms)) return item;
      const platforms = item.platforms.filter((platform) =>
        desktopPlatforms.has(platform),
      );
      return { ...item, platforms };
    })
    .filter((item) => !Array.isArray(item.platforms) || item.platforms.length > 0);
}

function sanitizeContribution(contribution) {
  const spec = contribution.spec ?? {};
  const contributionId = spec.contribution_id ?? contribution.id ?? "";
  const entryPath = spec.entry?.path ?? spec.schema_path ?? spec.path ?? "";
  if (isMobileValue(contributionId) || isMobileValue(entryPath)) return null;

  if (spec.entries && typeof spec.entries === "object") {
    spec.entries = Object.fromEntries(
      Object.entries(spec.entries).filter(([key]) => desktopEntryKeys.has(key)),
    );
  }

  if (spec.metadata && typeof spec.metadata === "object") {
    spec.metadata.navigationItems = sanitizeNavigationItems(
      spec.metadata.navigationItems,
    );
  }

  if (contribution.kind === "ui_page") {
    spec.visibility = {
      ...(spec.visibility ?? {}),
      platforms: ["windows", "linux", "macos"],
    };
  }

  contribution.spec = spec;
  return contribution;
}

function sanitizeManifest(manifest) {
  manifest.compatibility = {
    ...(manifest.compatibility ?? {}),
    platforms: ["windows", "linux", "macos"],
  };
  manifest.integrity = {
    ...(manifest.integrity ?? {}),
    algorithm: "sha256",
    contentTreeHash: "",
  };

  for (const module of manifest.modules ?? []) {
    const contributions = module.contributions ?? [];
    module.contributions = contributions
      .map((contribution) => sanitizeContribution(contribution))
      .filter(Boolean);
  }

  return manifest;
}

function removeMobilePayload(sourceDir) {
  const modulesDir = path.join(sourceDir, "modules");
  if (!fs.existsSync(modulesDir)) return;
  for (const module of fs.readdirSync(modulesDir, { withFileTypes: true })) {
    if (!module.isDirectory()) continue;
    const mobileDir = path.join(modulesDir, module.name, "mobile");
    if (fs.existsSync(mobileDir)) {
      fs.rmSync(mobileDir, { recursive: true, force: true });
    }
  }
}

export function buildLegacyDesktopPackage(sourceDir, outputDir) {
  const manifestPath = path.join(sourceDir, "manifest.json");
  const manifest = sanitizeManifest(
    JSON.parse(fs.readFileSync(manifestPath, "utf8")),
  );
  fs.writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  removeMobilePayload(sourceDir);

  const packageName = `${manifest.extension.id.replaceAll("/", ".")}-${manifest.extension.version}.amitiax`;
  const outputPath = path.join(outputDir, packageName);
  const temporaryPath = `${outputPath}.tmp`;
  fs.mkdirSync(outputDir, { recursive: true });
  fs.rmSync(temporaryPath, { force: true });

  const inspection = buildPackage(sourceDir, manifestPath, temporaryPath);
  inspectPackage(temporaryPath);
  fs.rmSync(outputPath, { force: true });
  fs.renameSync(temporaryPath, outputPath);

  return {
    packagePath: outputPath,
    treeHash: inspection.treeHash,
    files: inspection.files,
  };
}
