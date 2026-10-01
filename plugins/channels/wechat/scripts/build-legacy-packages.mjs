import path from "node:path";
import { fileURLToPath } from "node:url";
import { buildChannelPackage } from "../../scripts/build-channel-package.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const outputDir = path.resolve(scriptDir, "..", "..", "..", "..", "plugin");
const sourceDir = path.resolve(scriptDir, "..", "package-sources", "1.0.13");
const result = buildChannelPackage(sourceDir, outputDir);

console.log(JSON.stringify(result));
