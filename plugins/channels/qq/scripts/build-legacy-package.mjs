import path from "node:path";
import { fileURLToPath } from "node:url";
import { buildChannelPackage } from "../../scripts/build-channel-package.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const sourceDir = path.resolve(scriptDir, "..", "package-sources", "1.0.12");
const outputDir = path.resolve(scriptDir, "..", "..", "..", "..", "plugin");
const result = buildChannelPackage(sourceDir, outputDir);

console.log(JSON.stringify(result));
