import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const buildStampPath = resolve(desktopRoot, "resources/core/.amitiacore-build.json");

function sha256File(filePath) {
  const data = readFileSync(filePath);
  return createHash("sha256").update(data).digest("hex");
}

function main() {
  const args = process.argv.slice(2);
  const unpackedDir = args[0] || resolve(desktopRoot, "release/win-unpacked");

  if (!existsSync(buildStampPath)) {
    throw new Error("[verify-packaged-core] Build stamp missing. Run build-amitiacore.mjs first.");
  }

  const stamp = JSON.parse(readFileSync(buildStampPath, "utf8"));
  const expectedSha = stamp.CORE_BUILD_SHA256;

  if (!expectedSha) {
    throw new Error("[verify-packaged-core] Build stamp does not contain CORE_BUILD_SHA256");
  }

  const candidatePaths = [
    resolve(unpackedDir, "resources/core/AmitiaCore.exe"),
    resolve(unpackedDir, "app.asar.unpacked/resources/core/AmitiaCore.exe"),
  ];

  let corePath = null;
  for (const p of candidatePaths) {
    if (existsSync(p)) {
      corePath = p;
      break;
    }
  }

  if (!corePath) {
    throw new Error(
      `[verify-packaged-core] AmitiaCore.exe not found in package. Searched:\n${candidatePaths.map((p) => "  " + p).join("\n")}`,
    );
  }

  const actualSha = sha256File(corePath);

  console.log(`[verify-packaged-core] Core path: ${corePath}`);
  console.log(`[verify-packaged-core] Expected SHA256: ${expectedSha}`);
  console.log(`[verify-packaged-core] Actual SHA256:   ${actualSha}`);

  if (actualSha !== expectedSha) {
    throw new Error(
      `[verify-packaged-core] SHA256 mismatch! Built Core and packaged Core differ.`,
    );
  }

  console.log("[verify-packaged-core] PASS: Built Core SHA matches packaged Core SHA");
}

try {
  main();
} catch (error) {
  console.error(`[verify-packaged-core] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
