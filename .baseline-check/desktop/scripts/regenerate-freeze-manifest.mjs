import path from "node:path";
import { fileURLToPath } from "node:url";
import { writeFreezeManifest } from "../../scripts/lib/freeze-scope.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(scriptDir, "../..");

try {
  const { entries, manifestPath } = await writeFreezeManifest(repositoryRoot);
  console.log(`[freeze-manifest] generated ${entries.length} unique entries -> ${manifestPath}`);
} catch (error) {
  console.error(`[freeze-manifest] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
