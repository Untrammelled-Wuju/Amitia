import { createHash } from "node:crypto";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { computeFreezeSourceGateHash } from "../../scripts/lib/freeze-scope.mjs";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const buildStampPath = resolve(desktopRoot, "resources/core/.amitiacore-build.json");

function sha256File(filePath) {
  return createHash("sha256").update(readFileSync(filePath)).digest("hex");
}

async function main() {
  const unpackedDir = process.argv[2] || resolve(desktopRoot, "release/win-unpacked");
  if (!existsSync(buildStampPath)) {
    throw new Error("AmitiaCore build stamp missing; run build-amitiacore.mjs first");
  }

  const stamp = JSON.parse(readFileSync(buildStampPath, "utf8"));
  const expectedSha = stamp.CORE_BUILD_SHA256;
  const expectedSourceGate = stamp.CORE_SOURCE_GATE_SHA256;
  if (!/^[0-9a-f]{64}$/.test(expectedSha ?? "")) {
    throw new Error("build stamp has an invalid CORE_BUILD_SHA256");
  }
  if (!/^[0-9a-f]{64}$/.test(expectedSourceGate ?? "")) {
    throw new Error("build stamp has an invalid CORE_SOURCE_GATE_SHA256");
  }

  const currentSourceGate = await computeFreezeSourceGateHash(repositoryRoot);
  if (currentSourceGate !== expectedSourceGate) {
    throw new Error("frozen source changed after AmitiaCore was built");
  }

  const candidatePaths = [
    resolve(unpackedDir, "resources/core/AmitiaCore.exe"),
    resolve(unpackedDir, "app.asar.unpacked/resources/core/AmitiaCore.exe"),
  ];
  const corePath = candidatePaths.find((candidate) => existsSync(candidate));
  if (!corePath) {
    throw new Error(`AmitiaCore.exe not found in package. Searched:\n${candidatePaths.map((p) => `  ${p}`).join("\n")}`);
  }

  const actualSha = sha256File(corePath);
  if (actualSha !== expectedSha) {
    throw new Error(`packaged Core SHA mismatch: expected ${expectedSha}, got ${actualSha}`);
  }

  console.log(`[verify-packaged-core] PASS: ${actualSha}`);
  console.log(`[verify-packaged-core] SourceGate: ${currentSourceGate}`);
}

main().catch((error) => {
  console.error(`[verify-packaged-core] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
