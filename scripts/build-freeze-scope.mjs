import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  collectFreezeEntries,
  serializeFreezeEntries,
  verifyFreezeManifest,
  writeFreezeManifest,
} from "./lib/freeze-scope.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const workspaceRoot = path.resolve(scriptDir, "..");
const args = new Set(process.argv.slice(2));

async function main() {
  if (args.has("--write") || args.has("--manifest")) {
    const { entries, manifestPath } = await writeFreezeManifest(workspaceRoot);
    console.log(`[freeze-scope] wrote ${entries.length} entries -> ${manifestPath}`);
    return;
  }

  if (args.has("--verify")) {
    const result = await verifyFreezeManifest(workspaceRoot);
    console.log(`[freeze-scope] PASS: ${result.count} canonical freeze entries verified`);
    return;
  }

  const entries = await collectFreezeEntries(workspaceRoot);
  if (args.has("--list")) {
    for (const entry of entries) console.log(entry.relativePath);
    return;
  }

  if (args.has("--sha")) {
    process.stdout.write(serializeFreezeEntries(entries));
    return;
  }

  console.log(`Freeze scope files: ${entries.length}`);
  for (const entry of entries) console.log(`  ${entry.hash}  ${entry.relativePath}`);
}

main().catch((error) => {
  console.error(`[freeze-scope] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
});
