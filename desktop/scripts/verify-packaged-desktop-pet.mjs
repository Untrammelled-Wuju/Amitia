import { existsSync, readFileSync } from "node:fs";
import { resolve, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const projectRoot = resolve(__dirname, "..");
const unpacked = process.argv[2]
  ? resolve(projectRoot, process.argv[2])
  : resolve(projectRoot, "release-ci", "win-unpacked");
const appAsar = resolve(unpacked, "resources", "app.asar");
const errors = [];

function requireFile(path, label) {
  if (!existsSync(path)) {
    errors.push(`MISSING: ${label} (${relative(projectRoot, path)})`);
    return false;
  }
  return true;
}

requireFile(resolve(unpacked, "Amitia.exe"), "packaged Amitia.exe");
if (requireFile(appAsar, "packaged resources/app.asar")) {
  const asar = readFileSync(appAsar);
  const requiredNames = [
    "index.cjs",
    "animation-preload.cjs",
    "pet.html",
  ];
  for (const name of requiredNames) {
    if (!asar.includes(Buffer.from(name, "utf8"))) {
      errors.push(`ASAR MISSING: ${name}`);
    }
  }
  for (const forbidden of ["pet-preload.cjs", "pet-combined-preload.cjs"]) {
    if (asar.includes(Buffer.from(forbidden, "utf8"))) {
      errors.push(`ASAR FORBIDDEN: ${forbidden}`);
    }
  }
}

if (errors.length > 0) {
  console.error("[verify-packaged-desktop-pet] FAILED:");
  for (const error of errors) console.error(`  ERROR: ${error}`);
  process.exit(1);
}

console.log("[verify-packaged-desktop-pet] PASSED");
