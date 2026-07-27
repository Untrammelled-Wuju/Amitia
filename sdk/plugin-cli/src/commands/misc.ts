import type { CliCommand, CliContext, CliCommandResult, CliOption } from "../types.js";
import { EXIT_CODES } from "../exit-codes.js";
import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as http from "node:http";
import * as https from "node:https";
import * as path from "node:path";
import { spawnSync } from "node:child_process";
import { buildPackage, inspectPackage, readZip, createZip } from "../archive.js";
import type { ArchiveEntry } from "../archive.js";
import type { AmitiaxManifestV2 } from "../manifest.js";

export const testCommand: CliCommand = {
  name: "test",
  description: "Run extension unit, contract, and platform tests",
  usage: "amitia-ext test [options]",
  options: [
    { name: "--filter", shortName: "-f", description: "Test filter pattern", takesValue: true },
    { name: "--host-version", description: "Host contract version", takesValue: true },
    { name: "--platform", description: "Target platform", takesValue: true, choices: ["windows", "macos", "linux", "web"] },
    { name: "--arch", description: "Target architecture", takesValue: true, choices: ["x64", "arm64"] },
    { name: "--watch", description: "Watch mode" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    return runLocalTool(ctx, "vitest", ["run", ...args], "tests");
  },
};

export const buildCommand: CliCommand = {
  name: "build",
  description: "Build the extension TypeScript sources",
  usage: "amitia-ext build [options]",
  options: [
    { name: "--config", shortName: "-c", description: "TypeScript config path", takesValue: true },
    { name: "--manifest", shortName: "-m", description: "Manifest path", takesValue: true },
    { name: "--output", shortName: "-o", description: "Output directory", takesValue: true },
    { name: "--sourcemap", description: "Generate source maps" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    const config = path.resolve(ctx.cwd, opts["--config"] ?? "tsconfig.json");
    return runLocalTool(ctx, "typescript", ["-p", config], "build", "typescript/bin/tsc");
  },
};

export const packCommand: CliCommand = {
  name: "pack",
  description: "Pack the extension into a .amitiax package",
  usage: "amitia-ext pack [options]",
  options: [
    { name: "--manifest", shortName: "-m", description: "Manifest path", takesValue: true },
    { name: "--output", shortName: "-o", description: "Output package", takesValue: true, defaultValue: "./package.amitiax" },
    { name: "--deterministic", description: "Produce deterministic content tree" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    const manifestPath = path.resolve(ctx.cwd, opts["--manifest"] ?? "manifest.json");
    const output = path.resolve(ctx.cwd, opts["--output"] ?? "package.amitiax");
    try {
      const inspection = buildPackage(ctx.cwd, manifestPath, output);
      return { exitCode: EXIT_CODES.SUCCESS, message: `packed ${path.relative(ctx.cwd, output)}`, data: inspection };
    } catch (cause) {
      return { exitCode: EXIT_CODES.VALIDATION_OR_BUILD_FAILURE, message: cause instanceof Error ? cause.message : String(cause) };
    }
  },
};

export const signCommand: CliCommand = {
  name: "sign",
  description: "Sign a .amitiax package with a developer key",
  usage: "amitia-ext sign [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path (.amitiax)", takesValue: true, required: true },
    { name: "--private-key", shortName: "-k", description: "Private key file path (PEM ed25519)", takesValue: true, required: true },
    { name: "--key-id", description: "Signing key identifier (default: derived from key fingerprint)", takesValue: true },
    { name: "--publisher-id", description: "Publisher ID (default: read from manifest.json)", takesValue: true },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args, signCommand.options);
    const packagePath = opts["--package"];
    const privateKeyPath = opts["--private-key"];
    if (!packagePath) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "missing required option: --package" };
    }
    if (!privateKeyPath) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "missing required option: --private-key" };
    }
    try {
      const resolvedPackagePath = path.resolve(ctx.cwd, packagePath);
      const resolvedKeyPath = path.resolve(ctx.cwd, privateKeyPath);

      ctx.logger.info("reading package", { package: resolvedPackagePath });
      const packageBuffer = fs.readFileSync(resolvedPackagePath);
      const entries = readZip(packageBuffer);

      const treeData = entries.get("integrity/content-tree.json");
      if (!treeData) {
        return { exitCode: EXIT_CODES.VALIDATION_OR_BUILD_FAILURE, message: "package is missing integrity/content-tree.json" };
      }
      const treeDocument = JSON.parse(treeData.toString("utf8")) as { treeHash: string };
      const treeHash = treeDocument.treeHash;

      const manifestData = entries.get("manifest.json");
      if (!manifestData) {
        return { exitCode: EXIT_CODES.VALIDATION_OR_BUILD_FAILURE, message: "package is missing manifest.json" };
      }
      const manifest = JSON.parse(manifestData.toString("utf8")) as AmitiaxManifestV2;
      const publisherId = opts["--publisher-id"] ?? manifest.publisher?.id;
      if (!publisherId) {
        return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "publisher ID is required (provide --publisher-id or ensure manifest.json has publisher.id)" };
      }

      ctx.logger.info("loading private key", { key: resolvedKeyPath });
      const keyPem = fs.readFileSync(resolvedKeyPath, "utf8");
      const privateKey = crypto.createPrivateKey(keyPem);
      if (privateKey.asymmetricKeyType !== "ed25519") {
        return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: `private key is not ed25519 (got ${privateKey.asymmetricKeyType ?? "unknown"})` };
      }

      let keyId = opts["--key-id"];
      if (!keyId) {
        const publicKey = crypto.createPublicKey(privateKey);
        const publicKeyDer = publicKey.export({ type: "spki", format: "der" });
        const fingerprint = crypto.createHash("sha256").update(publicKeyDer).digest("hex");
        keyId = fingerprint.slice(0, 16);
      }

      const message = Buffer.from(`${publisherId}:${treeHash}`, "utf8");
      const signature = crypto.sign(null, message, privateKey);
      const signedAt = new Date().toISOString();

      const signatureDocument = {
        algorithm: "ed25519",
        keyId,
        publisherId,
        treeHash,
        signature: signature.toString("base64"),
        signedAt,
      };

      const signatureJson = Buffer.from(JSON.stringify(signatureDocument, null, 2), "utf8");
      entries.set("signatures/signature.json", signatureJson);

      const archiveEntries: ArchiveEntry[] = [];
      for (const [entryPath, data] of entries) {
        archiveEntries.push({ path: entryPath, data });
      }
      archiveEntries.sort((a, b) => a.path.localeCompare(b.path));

      const newZip = createZip(archiveEntries);
      fs.writeFileSync(resolvedPackagePath, newZip);

      ctx.logger.info("package signed", { keyId, publisherId, treeHash });

      return {
        exitCode: EXIT_CODES.SUCCESS,
        message: `signed ${path.relative(ctx.cwd, resolvedPackagePath)}`,
        data: signatureDocument,
      };
    } catch (cause) {
      return { exitCode: EXIT_CODES.SIGNATURE_OR_TRUST_ERROR, message: cause instanceof Error ? cause.message : String(cause) };
    }
  },
};

export const verifyCommand: CliCommand = {
  name: "verify",
  description: "Verify a .amitiax package's signature and integrity",
  usage: "amitia-ext verify [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--public-key", description: "Public key path", takesValue: true },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    try {
      const inspection = inspectPackage(path.resolve(ctx.cwd, opts["--package"]));
      return { exitCode: EXIT_CODES.SUCCESS, message: "package integrity verified", data: inspection };
    } catch (cause) {
      return { exitCode: EXIT_CODES.SIGNATURE_OR_TRUST_ERROR, message: cause instanceof Error ? cause.message : String(cause) };
    }
  },
};

export const inspectCommand: CliCommand = {
  name: "inspect",
  description: "Inspect a .amitiax package's contents and metadata",
  usage: "amitia-ext inspect [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--format", description: "Output format", takesValue: true, choices: ["human", "json"], defaultValue: "human" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    try {
      const inspection = inspectPackage(path.resolve(ctx.cwd, opts["--package"]));
      return { exitCode: EXIT_CODES.SUCCESS, message: `${inspection.manifest.extension.id} v${inspection.manifest.extension.version}`, data: inspection };
    } catch (cause) {
      return { exitCode: EXIT_CODES.VALIDATION_OR_BUILD_FAILURE, message: cause instanceof Error ? cause.message : String(cause) };
    }
  },
};

export const installCommand: CliCommand = {
  name: "install",
  description: "Install a .amitiax package via the local Developer Host",
  usage: "amitia-ext install [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "http://127.0.0.1:18899" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    return {
      exitCode: EXIT_CODES.HOST_CONNECTION_ERROR,
      message: "CLI host installation is not available in the MVP; use the extension center",
    };
  },
};

export const uninstallCommand: CliCommand = {
  name: "uninstall",
  description: "Uninstall an extension by ID via the Developer Host",
  usage: "amitia-ext uninstall <extensionId>",
  options: [
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "http://127.0.0.1:18899" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const extensionId = args[0];
    if (!extensionId) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "extensionId is required",
      };
    }
    return {
      exitCode: EXIT_CODES.HOST_CONNECTION_ERROR,
      message: "CLI host uninstall is not available in the MVP",
    };
  },
};

export const publishCheckCommand: CliCommand = {
  name: "publish-check",
  description: "Pre-publish verification for release readiness",
  usage: "amitia-ext publish-check [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true },
    { name: "--strict", description: "Treat warnings as errors" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "missing required option: --package" };
    }
    try {
      const inspection = inspectPackage(path.resolve(ctx.cwd, opts["--package"]));
      return { exitCode: EXIT_CODES.SUCCESS, message: "publish-check passed", data: inspection };
    } catch (cause) {
      return { exitCode: EXIT_CODES.VALIDATION_OR_BUILD_FAILURE, message: cause instanceof Error ? cause.message : String(cause) };
    }
  },
};

export const migrateCommand: CliCommand = {
  name: "migrate",
  description: "Assist in migrating legacy extension projects to v2",
  usage: "amitia-ext migrate [options]",
  options: [
    { name: "--source", shortName: "-s", description: "Legacy source directory", takesValue: true },
    { name: "--report", description: "Write migration report to file", takesValue: true },
  ],
  async run(_ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    return {
      exitCode: EXIT_CODES.CONFIGURATION_ERROR,
      message: "legacy migration is not available in the MVP",
    };
  },
};

export const doctorCommand: CliCommand = {
  name: "doctor",
  description: "Diagnose environment, toolchain, and SDK installation",
  usage: "amitia-ext doctor",
  async run(ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    const checks: Array<{ name: string; ok: boolean; detail?: string }> = [];
    checks.push({ name: "node", ok: typeof process !== "undefined" && !!process.versions.node, detail: process.versions.node });
    checks.push({ name: "cli", ok: true, detail: "1.0.0" });
    const failed = checks.filter((c) => !c.ok);
    return {
      exitCode: failed.length > 0 ? EXIT_CODES.ENVIRONMENT_ERROR : EXIT_CODES.SUCCESS,
      message: failed.length > 0 ? `doctor failed: ${failed.map((c) => c.name).join(", ")}` : "doctor passed",
      data: { checks },
    };
  },
};

export const sdkCommand: CliCommand = {
  name: "sdk",
  description: "Inspect SDK version and compatibility matrix",
  usage: "amitia-ext sdk [info|version]",
  async run(_ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "sdk info",
      data: {
        sdkVersion: "1.0.0",
        supportedManifestVersion: 2,
        supportedHostApiRange: ">=1.0.0 <2.0.0",
        supportedRuntimeRpcRange: ">=1.0.0 <2.0.0",
        minAmitiaVersion: "0.1.0",
      },
    };
  },
};

export const exportDiagnosticsCommand: CliCommand = {
  name: "export-diagnostics",
  description: "Export diagnostics data from the backend API",
  usage: "amitia-ext export-diagnostics [options]",
  options: [
    { name: "--host", shortName: "-h", description: "Backend API host", takesValue: true, defaultValue: "http://127.0.0.1:18899" },
    { name: "--extension", shortName: "-e", description: "Filter by extension ID", takesValue: true },
    { name: "--severity", description: "Filter by severity level", takesValue: true },
    { name: "--output", shortName: "-o", description: "Output file path (default: stdout)", takesValue: true },
    { name: "--format", shortName: "-f", description: "Output format", takesValue: true, choices: ["json", "text"], defaultValue: "json" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args, exportDiagnosticsCommand.options);
    const host = opts["--host"] ?? "http://127.0.0.1:18899";
    const extension = opts["--extension"];
    const severity = opts["--severity"];
    const output = opts["--output"];
    const format = opts["--format"] ?? "json";

    try {
      const params = new URLSearchParams();
      if (extension) params.set("extension", extension);
      if (severity) params.set("severity", severity);
      const query = params.toString();
      const url = `${host.replace(/\/+$/, "")}/api/dev-console/export-diagnostics${query ? `?${query}` : ""}`;

      ctx.logger.info("requesting diagnostics", { url });
      const response = await httpRequest(url);

      if (response.statusCode !== 200) {
        return { exitCode: EXIT_CODES.HOST_CONNECTION_ERROR, message: `backend returned ${response.statusCode}: ${response.body}` };
      }

      const result = format === "text" ? formatDiagnosticsText(response.body) : response.body;

      if (output) {
        const outputPath = path.resolve(ctx.cwd, output);
        fs.writeFileSync(outputPath, result, "utf8");
        ctx.logger.info("diagnostics written to file", { output: outputPath });
        return { exitCode: EXIT_CODES.SUCCESS, message: `diagnostics exported to ${path.relative(ctx.cwd, outputPath)}` };
      }

      return { exitCode: EXIT_CODES.SUCCESS, message: result };
    } catch (cause) {
      return { exitCode: EXIT_CODES.HOST_CONNECTION_ERROR, message: cause instanceof Error ? cause.message : String(cause) };
    }
  },
};

function parseOptions(args: string[], definitions?: ReadonlyArray<CliOption>): Record<string, string | undefined> {
  const opts: Record<string, string | undefined> = {};
  const shortToLong = new Map<string, string>();
  if (definitions) {
    for (const def of definitions) {
      if (def.shortName) shortToLong.set(def.shortName, def.name);
    }
  }
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg === undefined || !arg.startsWith("-")) continue;
    const key = arg.startsWith("--") ? arg : (shortToLong.get(arg) ?? arg);
    const next = args[i + 1];
    if (next && !next.startsWith("-")) {
      opts[key] = next;
      i++;
    } else {
      opts[key] = "true";
    }
  }
  return opts;
}

function httpRequest(url: string): Promise<{ statusCode: number; body: string }> {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const lib = parsed.protocol === "https:" ? https : http;
    const req = lib.request(parsed, (res) => {
      const chunks: Buffer[] = [];
      res.on("data", (chunk: Buffer) => chunks.push(chunk));
      res.on("end", () => {
        resolve({
          statusCode: res.statusCode ?? 0,
          body: Buffer.concat(chunks).toString("utf8"),
        });
      });
    });
    req.on("error", reject);
    req.setTimeout(15000, () => {
      req.destroy(new Error("request timed out"));
    });
    req.end();
  });
}

function formatDiagnosticsText(body: string): string {
  try {
    const data = JSON.parse(body);
    const lines: string[] = [];
    const items = Array.isArray(data) ? data : (data.items ?? data.diagnostics ?? [data]);
    for (const item of items) {
      const ext = item.extension ?? item.extensionId ?? "-";
      const sev = item.severity ?? "-";
      const msg = item.message ?? item.description ?? JSON.stringify(item);
      lines.push(`[${sev}] ${ext}: ${msg}`);
    }
    return lines.length > 0 ? lines.join("\n") : "no diagnostics found";
  } catch {
    return body;
  }
}

function runLocalTool(ctx: CliContext, packageName: string, args: string[], label: string, relativeBinary = `${packageName}/vitest.mjs`): CliCommandResult {
  const binary = path.resolve(ctx.cwd, "node_modules", relativeBinary);
  if (!fs.existsSync(binary)) {
    return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: `${packageName} is not installed in ${ctx.cwd}` };
  }
  const result = spawnSync(process.execPath, [binary, ...args], { cwd: ctx.cwd, stdio: "inherit" });
  if (result.error) return { exitCode: EXIT_CODES.ENVIRONMENT_ERROR, message: result.error.message };
  const exitCode = result.status ?? EXIT_CODES.ENVIRONMENT_ERROR;
  return { exitCode, message: exitCode === 0 ? `${label} succeeded` : `${label} failed` };
}
