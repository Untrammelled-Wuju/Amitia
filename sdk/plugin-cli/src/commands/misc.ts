import type { CliCommand, CliContext, CliCommandResult, CliOption } from "../types.js";
import { EXIT_CODES } from "../exit-codes.js";
import * as crypto from "node:crypto";
import * as fs from "node:fs";
import * as http from "node:http";
import * as https from "node:https";
import * as os from "node:os";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { buildPackage, inspectPackage, readZip, createZip } from "../archive.js";
import type { ArchiveEntry } from "../archive.js";

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
    const projectTsc = path.resolve(ctx.cwd, "node_modules", "typescript", "bin", "tsc");
    const tsc = fs.existsSync(projectTsc)
      ? projectTsc
      : fileURLToPath(new URL("../../node_modules/typescript/bin/tsc", import.meta.url));
    if (!fs.existsSync(tsc)) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: `typescript is not installed in ${ctx.cwd}` };
    }
    const sdkEntry = fileURLToPath(new URL("../../node_modules/@amitia/plugin-sdk/dist/index.d.ts", import.meta.url));
    const overlay = path.join(os.tmpdir(), `amitia-tsconfig-${process.pid}-${Date.now()}.json`);
    fs.writeFileSync(overlay, JSON.stringify({
      extends: config,
      compilerOptions: {
        baseUrl: ctx.cwd,
        paths: {
          "@amitia/plugin-sdk": [sdkEntry],
        },
      },
    }, null, 2));
    try {
      const result = spawnSync(process.execPath, [tsc, "-p", overlay], { cwd: ctx.cwd, stdio: "inherit" });
      if (result.error) return { exitCode: EXIT_CODES.ENVIRONMENT_ERROR, message: result.error.message };
      const exitCode = result.status ?? EXIT_CODES.ENVIRONMENT_ERROR;
      return { exitCode, message: exitCode === 0 ? "build succeeded" : "build failed" };
    } finally {
      fs.rmSync(overlay, { force: true });
    }
  },
};

export const packCommand: CliCommand = {
  name: "pack",
  description: "Pack the extension into a unified .amitiax/.gamex/.petx plugin package",
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
  description: "Sign a unified plugin package with a developer key (delegates to amitiax CLI)",
  usage: "amitia-ext sign [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path (.amitiax/.gamex/.petx)", takesValue: true, required: true },
    { name: "--private-key", shortName: "-k", description: "Private key file path (PEM ed25519 or hex)", takesValue: true, required: true },
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

    const resolvedPackagePath = path.resolve(ctx.cwd, packagePath);
    const resolvedKeyPath = path.resolve(ctx.cwd, privateKeyPath);

    const amitiaxBinary = findAmitiaxBinary();
    if (!amitiaxBinary) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "amitiax CLI binary not found. Please build it with: cd backend && go build -o amitiax ./cmd/amitia-ext",
      };
    }

    const hexKeyPath = await ensureHexKeyFile(resolvedKeyPath);
    if (!hexKeyPath) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "failed to convert private key to hex format",
      };
    }

    const publisherId = opts["--publisher-id"] ?? "";
    const keyId = opts["--key-id"] ?? "";

    const cliArgs: string[] = ["sign", resolvedPackagePath, "--key", hexKeyPath, "--publisher", publisherId, "--json"];
    if (keyId) {
      cliArgs.push("--key-id", keyId);
    }

    ctx.logger.info("delegating to amitiax CLI", { binary: amitiaxBinary, args: cliArgs });

    const result = spawnSync(amitiaxBinary, cliArgs, {
      cwd: ctx.cwd,
      encoding: "utf8",
      stdio: ["pipe", "pipe", "pipe"],
    });

    if (result.status !== 0) {
      return {
        exitCode: EXIT_CODES.SIGNATURE_OR_TRUST_ERROR,
        message: `amitiax sign failed: ${result.stderr || result.stdout || "unknown error"}`,
      };
    }

    let data: unknown = null;
    try {
      data = JSON.parse(result.stdout);
    } catch (error) {
  console.error("[original-error]", error);
data = { raw: result.stdout };
    }

    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: `signed ${path.relative(ctx.cwd, resolvedPackagePath)} (via amitiax-signature-v1)`,
      data,
    };
  },
};

export const verifyCommand: CliCommand = {
  name: "verify",
  description: "Verify a .amitiax package's signature and integrity",
  usage: "amitia-ext verify [options]",
  options: [
    { name: "--package", shortName: "-p", description: "Package path", takesValue: true, required: true },
    { name: "--public-key", description: "Public key path", takesValue: true },
    { name: "--publisher", description: "Publisher ID", takesValue: true },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    const publicKey = opts["--public-key"];
    const publisher = opts["--publisher"];
    if (!publicKey || !publisher) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "verify requires --public-key and --publisher" };
    }
    const binary = findAmitiaxBinary();
    if (!binary) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "amitiax CLI binary not found" };
    }
    const result = spawnSync(binary, [
      "verify",
      path.resolve(ctx.cwd, opts["--package"]),
      "--pubkey",
      path.resolve(ctx.cwd, publicKey),
      "--publisher",
      publisher,
      "--json",
    ], { cwd: ctx.cwd, encoding: "utf8" });
    if (result.status !== 0) {
      return {
        exitCode: EXIT_CODES.SIGNATURE_OR_TRUST_ERROR,
        message: `signature verification failed: ${result.stderr || result.stdout || "unknown error"}`,
      };
    }
    let data: unknown = { raw: result.stdout };
    try {
      data = JSON.parse(result.stdout);
    } catch {
      data = { raw: result.stdout };
    }
    return { exitCode: EXIT_CODES.SUCCESS, message: "package integrity and signature verified", data };
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
      return { exitCode: EXIT_CODES.SUCCESS, message: `${inspection.manifest.plugin.id} v${inspection.manifest.plugin.version}`, data: inspection };
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
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "" },
    { name: "--token", description: "Local runtime token", takesValue: true },
    { name: "--token-file", description: "Local runtime token file", takesValue: true },
    { name: "--scope", description: "Installation scope", takesValue: true, defaultValue: "global" },
    { name: "--scope-id", description: "Installation scope id", takesValue: true, defaultValue: "" },
    { name: "--no-enable", description: "Do not enable after installation" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args, installCommand.options);
    if (!opts["--package"]) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "missing required option: --package",
      };
    }
    try {
      const host = resolveHost(opts);
      const token = resolveLocalToken(ctx, opts);
      const packagePath = path.resolve(ctx.cwd, opts["--package"]);
      const payload = await installPackage(host, token, packagePath, opts["--scope"] ?? "global", opts["--scope-id"] ?? "");
      if (opts["--no-enable"] === undefined) {
        await hostJSON(host, token, "POST", "/api/extensions/kernel/extensions/enable", { id: payload.extensionId });
      }
      return {
        exitCode: EXIT_CODES.SUCCESS,
        message: `installed ${payload.extensionId} ${payload.version}${opts["--no-enable"] === undefined ? " and enabled" : ""}`,
        data: payload,
      };
    } catch (cause) {
      return {
        exitCode: EXIT_CODES.HOST_CONNECTION_ERROR,
        message: cause instanceof Error ? cause.message : String(cause),
      };
    }
  },
};

export const uninstallCommand: CliCommand = {
  name: "uninstall",
  description: "Uninstall an extension by ID via the Developer Host",
  usage: "amitia-ext uninstall <extensionId>",
  options: [
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "" },
    { name: "--token", description: "Local runtime token", takesValue: true },
    { name: "--token-file", description: "Local runtime token file", takesValue: true },
    { name: "--scope", description: "Installation scope", takesValue: true, defaultValue: "global" },
    { name: "--scope-id", description: "Installation scope id", takesValue: true, defaultValue: "" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args, uninstallCommand.options);
    const extensionId = positionalArgs(args, uninstallCommand.options)[0];
    if (!extensionId) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: "extensionId is required",
      };
    }
    try {
      const host = resolveHost(opts);
      const token = resolveLocalToken(ctx, opts);
      const scope = opts["--scope"] ?? "global";
      const scopeId = opts["--scope-id"] ?? "";
      const preview = await hostJSON(host, token, "POST", "/api/extensions/kernel/extensions/uninstall/preview", {
        extensionId,
        scopeType: scope,
        scopeId,
      });
      const confirmations = stringSetMap(preview.requiredConfirmations);
      const confirmation = await hostJSON(host, token, "POST", "/api/extensions/kernel/extensions/uninstall/confirm", {
        extensionId,
        scopeType: scope,
        scopeId,
        confirmations,
      });
      const operation = await hostJSON(host, token, "POST", "/api/extensions/kernel/extensions/uninstall", {
        extensionId,
        scopeType: scope,
        scopeId,
        confirmationToken: String(confirmation.confirmationToken ?? ""),
        idempotencyKey: `cli-uninstall-${crypto.randomUUID()}`,
      });
      return {
        exitCode: EXIT_CODES.SUCCESS,
        message: `uninstalled ${extensionId}`,
        data: operation,
      };
    } catch (cause) {
      return {
        exitCode: EXIT_CODES.HOST_CONNECTION_ERROR,
        message: cause instanceof Error ? cause.message : String(cause),
      };
    }
  },
};

export const enableCommand = lifecycleCommand("enable", "Enable an installed extension");
export const disableCommand = lifecycleCommand("disable", "Disable an installed extension");

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
  description: "Inspect SDK and current protocol versions",
  usage: "amitia-ext sdk [info|version]",
  async run(_ctx: CliContext, _args: string[]): Promise<CliCommandResult> {
    return {
      exitCode: EXIT_CODES.SUCCESS,
      message: "sdk info",
      data: {
        sdkVersion: "1.0.0",
        supportedManifestVersion: 1,
        hostApiVersion: "1.0.0",
        runtimeRpcVersion: "1.0.0",
      },
    };
  },
};

export const exportDiagnosticsCommand: CliCommand = {
  name: "export-diagnostics",
  description: "Export diagnostics data from the backend API",
  usage: "amitia-ext export-diagnostics [options]",
  options: [
    { name: "--host", shortName: "-h", description: "Backend API host", takesValue: true, defaultValue: "" },
    { name: "--extension", shortName: "-e", description: "Filter by extension ID", takesValue: true },
    { name: "--severity", description: "Filter by severity level", takesValue: true },
    { name: "--output", shortName: "-o", description: "Output file path (default: stdout)", takesValue: true },
    { name: "--format", shortName: "-f", description: "Output format", takesValue: true, choices: ["json", "text"], defaultValue: "json" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args, exportDiagnosticsCommand.options);
    const host = opts["--host"] ?? "";
    if (!host.trim()) {
      return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "missing required option: --host" };
    }
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

function lifecycleCommand(name: "enable" | "disable", description: string): CliCommand {
  const options: CliOption[] = [
    { name: "--host", description: "Developer Host endpoint", takesValue: true, defaultValue: "" },
    { name: "--token", description: "Local runtime token", takesValue: true },
    { name: "--token-file", description: "Local runtime token file", takesValue: true },
  ];
  return {
    name,
    description,
    usage: `amitia-ext ${name} <extensionId>`,
    options,
    async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
      const opts = parseOptions(args, options);
      const extensionId = positionalArgs(args, options)[0];
      if (!extensionId) {
        return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: "extensionId is required" };
      }
      try {
        const host = resolveHost(opts);
        const token = resolveLocalToken(ctx, opts);
        const data = await hostJSON(host, token, "POST", `/api/extensions/kernel/extensions/${name}`, { id: extensionId });
        return {
          exitCode: EXIT_CODES.SUCCESS,
          message: `${name === "enable" ? "enabled" : "disabled"} ${extensionId}`,
          data,
        };
      } catch (cause) {
        return {
          exitCode: EXIT_CODES.HOST_CONNECTION_ERROR,
          message: cause instanceof Error ? cause.message : String(cause),
        };
      }
    },
  };
}

function positionalArgs(args: string[], definitions: ReadonlyArray<CliOption> = []): string[] {
  const takesValue = new Set<string>();
  for (const definition of definitions) {
    if (definition.takesValue) {
      takesValue.add(definition.name);
      if (definition.shortName) takesValue.add(definition.shortName);
    }
  }
  const result: string[] = [];
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (!arg?.startsWith("-")) {
      result.push(arg);
      continue;
    }
    if (takesValue.has(arg) && index + 1 < args.length) index++;
  }
  return result;
}

function resolveHost(opts: Record<string, string | undefined>): string {
  const host = (opts["--host"] ?? process.env.AMITIA_DEV_HOST ?? "").trim().replace(/\/+$/, "");
  if (!host) throw new Error("missing required option: --host");
  const parsed = new URL(host);
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("Developer Host endpoint must use http or https");
  }
  return host;
}

function resolveLocalToken(ctx: CliContext, opts: Record<string, string | undefined>): string {
  const explicit = (opts["--token"] ?? process.env.AMITIA_LOCAL_TOKEN ?? "").trim();
  if (explicit) return explicit;
  const dataDir = (process.env.AMITIA_DATA_DIR ?? path.resolve(ctx.cwd, "AmitiaData")).trim();
  const tokenFile = path.resolve(ctx.cwd, opts["--token-file"] ?? path.join(dataDir, "security", "local-token"));
  if (!fs.existsSync(tokenFile)) throw new Error(`local runtime token not found: ${tokenFile}`);
  const token = fs.readFileSync(tokenFile, "utf8").trim();
  if (!token) throw new Error(`local runtime token is empty: ${tokenFile}`);
  return token;
}

async function hostJSON(
  host: string,
  token: string,
  method: string,
  requestPath: string,
  payload?: unknown,
  acceptedStatuses: number[] = [200],
): Promise<Record<string, any>> {
  const body = payload === undefined ? undefined : Buffer.from(JSON.stringify(payload));
  const response = await rawHostRequest(host, token, method, requestPath, body, payload === undefined ? undefined : "application/json");
  if (!acceptedStatuses.includes(response.statusCode)) {
    throw new Error(`Developer Host ${method} ${requestPath} returned ${response.statusCode}: ${response.body}`);
  }
  if (!response.body.trim()) return {};
  const value = JSON.parse(response.body);
  return value && typeof value === "object" ? value : { value };
}

function rawHostRequest(
  host: string,
  token: string,
  method: string,
  requestPath: string,
  body?: Buffer,
  contentType?: string,
): Promise<{ statusCode: number; body: string }> {
  return new Promise((resolve, reject) => {
    const base = new URL(host);
    const url = new URL(requestPath, base);
    const lib = url.protocol === "https:" ? https : http;
    const headers: Record<string, string | number> = { Accept: "application/json", "X-Amitia-Local-Token": token };
    if (body) {
      headers["Content-Type"] = contentType ?? "application/octet-stream";
      headers["Content-Length"] = body.length;
    }
    const request = lib.request(url, { method, headers }, (response) => {
      const chunks: Buffer[] = [];
      response.on("data", (chunk: Buffer) => chunks.push(chunk));
      response.on("end", () => resolve({
        statusCode: response.statusCode ?? 0,
        body: Buffer.concat(chunks).toString("utf8"),
      }));
    });
    request.on("error", reject);
    request.setTimeout(125000, () => request.destroy(new Error("Developer Host request timed out")));
    if (body) request.write(body);
    request.end();
  });
}

async function installPackage(
  host: string,
  token: string,
  packagePath: string,
  scopeType: string,
  scopeId: string,
): Promise<Record<string, unknown>> {
  if (!fs.existsSync(packagePath)) throw new Error(`package not found: ${packagePath}`);
  const inspection = inspectPackage(packagePath);
  const boundary = `----amitia-ext-${crypto.randomUUID()}`;
  const fields = new Map<string, string>([
    ["scopeType", scopeType],
    ["scopeId", scopeId],
    ["allowUnsignedDev", "true"],
  ]);
  const chunks: Buffer[] = [];
  for (const [name, value] of fields) {
    chunks.push(Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="${name}"\r\n\r\n${value}\r\n`));
  }
  chunks.push(Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${path.basename(packagePath)}"\r\nContent-Type: application/octet-stream\r\n\r\n`));
  chunks.push(fs.readFileSync(packagePath));
  chunks.push(Buffer.from(`\r\n--${boundary}--\r\n`));
  const previewResponse = await rawHostRequest(
    host,
    token,
    "POST",
    "/api/extensions/packages/artifacts",
    Buffer.concat(chunks),
    `multipart/form-data; boundary=${boundary}`,
  );
  if (previewResponse.statusCode !== 201) {
    throw new Error(`package preview failed with ${previewResponse.statusCode}: ${previewResponse.body}`);
  }
  const previewEnvelope = JSON.parse(previewResponse.body) as Record<string, any>;
  const preview = previewEnvelope.preview as Record<string, any> | undefined;
  const sessionId = String(preview?.sessionId ?? "");
  if (!sessionId) throw new Error("package preview did not return a sessionId");
  const confirmationMap = stringSetMap(preview?.capabilityConfirmations);
  for (const item of toList(preview?.requiredConfirmations)) {
    const key = String(item);
    if (key) confirmationMap[key] = true;
  }
  const confirmation = await hostJSON(host, token, "POST", `/api/extensions/packages/previews/${encodeURIComponent(sessionId)}/confirm`, {
    scopeType,
    scopeId,
    confirmations: confirmationMap,
  });
  const confirmationToken = String(confirmation.confirmationToken ?? "");
  if (!confirmationToken) throw new Error("package confirmation did not return a token");
  const operation = await hostJSON(host, token, "POST", "/api/extensions/packages/operations/install", {
    sessionId,
    scopeType,
    scopeId,
    confirmationToken,
    idempotencyKey: `cli-install-${crypto.randomUUID()}`,
  }, [200, 202]);
  const operationId = String(operation.operationId ?? "");
  const completed = operationId ? await waitHostOperation(host, token, operationId) : operation;
  const extensionId = String(completed.extensionId ?? preview?.extensionId ?? inspection.manifest.plugin.id);
  const version = String(completed.version ?? preview?.version ?? inspection.manifest.plugin.version);
  return { extensionId, version, operationId, operation: completed };
}

async function waitHostOperation(host: string, token: string, operationId: string): Promise<Record<string, any>> {
  const deadline = Date.now() + 125000;
  let latest: Record<string, any> = {};
  while (Date.now() < deadline) {
    latest = await hostJSON(host, token, "GET", `/api/extensions/packages/operations/${encodeURIComponent(operationId)}`);
    const status = String(latest.status ?? "");
    if (status === "completed") return latest;
    if (status === "failed" || status === "requires_recovery" || status === "rolled_back") {
      throw new Error(`package operation ${operationId} ended with ${status}: ${latest.errorDetail ?? latest.error ?? ""}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  throw new Error(`package operation ${operationId} did not complete before timeout`);
}

function stringSetMap(value: unknown): Record<string, boolean> {
  const result: Record<string, boolean> = {};
  for (const item of toList(value)) {
    const key = String(item);
    if (key) result[key] = true;
  }
  return result;
}

function toList(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

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
  } catch (error) {
  console.error("[original-error]", error);
return body;
  }
}

function runLocalTool(ctx: CliContext, packageName: string, args: string[], label: string, relativeBinary = `${packageName}/vitest.mjs`, fallbackBinary = ""): CliCommandResult {
  const projectBinary = path.resolve(ctx.cwd, "node_modules", relativeBinary);
  const binary = fs.existsSync(projectBinary) ? projectBinary : fallbackBinary;
  if (!fs.existsSync(binary)) {
    return { exitCode: EXIT_CODES.CONFIGURATION_ERROR, message: `${packageName} is not installed in ${ctx.cwd}` };
  }
  const result = spawnSync(process.execPath, [binary, ...args], { cwd: ctx.cwd, stdio: "inherit" });
  if (result.error) return { exitCode: EXIT_CODES.ENVIRONMENT_ERROR, message: result.error.message };
  const exitCode = result.status ?? EXIT_CODES.ENVIRONMENT_ERROR;
  return { exitCode, message: exitCode === 0 ? `${label} succeeded` : `${label} failed` };
}

function findAmitiaxBinary(): string | null {
  const candidates: string[] = [];
  const ext = process.platform === "win32" ? ".exe" : "";
  candidates.push(path.resolve(process.cwd(), "backend", `amitiax${ext}`));
  candidates.push(path.resolve(process.cwd(), "backend", `amitia-ext${ext}`));
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  const pathDirs = (process.env.PATH ?? "").split(path.delimiter);
  for (const dir of pathDirs) {
    const amitiaxPath = path.join(dir, `amitiax${ext}`);
    if (fs.existsSync(amitiaxPath)) return amitiaxPath;
    const amitiaExtPath = path.join(dir, `amitia-ext${ext}`);
    if (fs.existsSync(amitiaExtPath)) return amitiaExtPath;
  }
  return null;
}

async function ensureHexKeyFile(keyPath: string): Promise<string | null> {
  const raw = fs.readFileSync(keyPath, "utf8").trim();
  if (/^[0-9a-fA-F]+$/.test(raw) && raw.length === 128) {
    return keyPath;
  }
  try {
    const privateKey = crypto.createPrivateKey(raw);
    if (privateKey.asymmetricKeyType !== "ed25519") {
      return null;
    }
    const pkcs8 = privateKey.export({ type: "pkcs8", format: "der" });
    const rawKeyBytes = pkcs8.slice(pkcs8.length - 32);
    const hexKey = Buffer.from(rawKeyBytes).toString("hex");
    const tmpPath = path.join(os.tmpdir(), `amitiax-key-${Date.now()}.hex`);
    fs.writeFileSync(tmpPath, hexKey, { mode: 0o600 });
    return tmpPath;
  } catch (error) {
  console.error("[original-error]", error);
return null;
  }
}
