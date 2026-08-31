import { mkdirSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const desktopRoot = resolve(__dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const stampPath = resolve(desktopRoot, "release/.desktop-pet-release-gate.json");
const latestPath = resolve(repositoryRoot, "LATEST_RELEASE_GATE_REPORT.md");

function main() {
  if (!existsSync(stampPath)) {
    throw new Error("release gate stamp missing; generate the report only after a successful installer build");
  }
  const stamp = JSON.parse(readFileSync(stampPath, "utf8"));
  const createdAt = new Date(stamp.createdAt);
  const directoryName = createdAt.toISOString().replace(/[:.]/g, "-");
  const reportDir = resolve(repositoryRoot, "release-reports", directoryName);
  mkdirSync(reportDir, { recursive: true });

  const artifactRows = (stamp.artifacts ?? [])
    .map((artifact) => `| ${artifact.name} | ${artifact.bytes} | \`${artifact.sha256}\` |`)
    .join("\n");

  const report = `# AMITIA Release Gate Report\n\n` +
    `Generated: ${stamp.createdAt}\n\n` +
    `Status: **PASS**\n\n` +
    `| Gate | Result |\n|---|---|\n` +
    `| Canonical freeze verification | PASS (${stamp.freezeShaVerified} files) |\n` +
    `| Source Gate SHA256 | \`${stamp.sourceGateSha256}\` |\n` +
    `| AmitiaCore provenance | PASS |\n` +
    `| Runtime asset provenance | PASS (${stamp.runtimeAssets?.entries?.length ?? 0} files) |\n` +
    `| Packaged artifact integrity | PASS |\n\n` +
    `## Core provenance\n\n` +
    `- Core SHA256: \`${stamp.core?.sha256 ?? "unknown"}\`\n` +
    `- Core SourceGate SHA256: \`${stamp.core?.sourceGateSha256 ?? "unknown"}\`\n` +
    `- Go: ${stamp.core?.goVersion ?? "unknown"}\n` +
    `- Commit: ${stamp.core?.commit ?? "unknown"}\n\n` +
    `## Runtime assets\n\n` +
    `- Runtime asset SourceGate SHA256: \`${stamp.runtimeAssets?.sourceGateSha256 ?? "unknown"}\`\n` +
    `- Verified packaged runtime files: ${stamp.runtimeAssets?.entries?.length ?? 0}\n\n` +
    `## Release artifacts\n\n` +
    `| Artifact | Bytes | SHA256 |\n|---|---:|---|\n${artifactRows}\n`;

  writeFileSync(resolve(reportDir, "release-gate-report.md"), report, "utf8");
  writeFileSync(latestPath, report, "utf8");
  console.log(`[release-report] PASS: ${latestPath}`);
}

try {
  main();
} catch (error) {
  console.error(`[release-report] FAILED: ${error instanceof Error ? error.message : String(error)}`);
  process.exit(1);
}
