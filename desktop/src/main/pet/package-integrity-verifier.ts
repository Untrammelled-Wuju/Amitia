import { createHash } from "node:crypto";
import { readFile, stat, readdir } from "node:fs/promises";
import { dirname, extname, join, relative } from "node:path";
import { realpathSync } from "node:fs";
import type {
  NormalizedManifestData,
  RuntimeAction,
} from "../../shared/package-schema";
import { MANIFEST_PSEUDO_ENTRY_PATH } from "../../shared/package-schema";
import type {
  PackageValidationError,
  PackageErrorCode,
} from "../../shared/package-errors";

export interface IntegrityVerificationResult {
  valid: boolean;
  errors: PackageValidationError[];
}

export interface VerifyParams {
  manifestRawText: string;
  manifest: NormalizedManifestData;
  installPath: string;
  manifestPath: string;
  actions: Map<string, RuntimeAction>;
}

interface CacheEntry {
  fingerprint: string;
  result: IntegrityVerificationResult;
}

const MIME_MAP: Record<string, string> = {
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".bmp": "image/bmp",
  ".json": "application/json",
  ".txt": "text/plain",
  ".mp3": "audio/mpeg",
  ".wav": "audio/wav",
};

export class PackageIntegrityVerifier {
  private cache = new Map<string, CacheEntry>();

  async verify(params: VerifyParams): Promise<IntegrityVerificationResult> {
    const { manifest, installPath } = params;
    const cacheKey = `${manifest.releaseId}:${manifest.integrity.contentRootHash}`;
    const fingerprint = await this.computeFingerprint(installPath, params.manifestRawText);

    const cached = this.cache.get(cacheKey);
    if (cached && cached.fingerprint === fingerprint) {
      return cached.result;
    }

    const errors: PackageValidationError[] = [];

    this.verifyManifestHash(params, errors);
    this.verifyContentRootHash(params, errors);
    this.verifyFileCount(manifest, errors);

    await this.verifyFiles(params, errors);
    await this.verifyUndeclaredFiles(params, errors);
    await this.verifyActionConfigs(params, errors);
    await this.verifyPreview(params, errors);
    await this.verifyFrames(params, errors);
    this.verifyMediaTypes(manifest, errors);

    const result: IntegrityVerificationResult = {
      valid: errors.filter((e) => e.severity === "error").length === 0,
      errors,
    };

    this.cache.set(cacheKey, { fingerprint, result });
    return result;
  }

  clearCache(): void {
    this.cache.clear();
  }

  invalidate(releaseId: string, contentRootHash: string): void {
    const cacheKey = `${releaseId}:${contentRootHash}`;
    this.cache.delete(cacheKey);
  }

  private verifyManifestHash(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): void {
    const { manifestRawText, manifest } = params;
    const declaredHash = manifest.integrity.manifestHash;
    if (!declaredHash) {
      errors.push(this.makeError("PACKAGE_MANIFEST_HASH_MISSING", "manifestHash is missing", "error"));
      return;
    }
    let canonical: string;
    try {
      canonical = canonicalManifestJSON(manifestRawText);
    } catch {
      errors.push(this.makeError("PACKAGE_MANIFEST_INVALID", "manifest is not valid JSON", "error"));
      return;
    }
    const computed = sha256Hex(canonical);
    if (computed !== declaredHash) {
      errors.push(this.makeError(
        "PACKAGE_MANIFEST_HASH_MISMATCH",
        `manifestHash mismatch: expected ${declaredHash}, got ${computed}`,
        "error",
        { expected: declaredHash, actual: computed },
      ));
    }
  }

  private verifyContentRootHash(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): void {
    const { manifestRawText, manifest } = params;
    const declaredHash = manifest.integrity.contentRootHash;
    if (!declaredHash) {
      errors.push(this.makeError("PACKAGE_INTEGRITY_MISSING", "contentRootHash is missing", "error"));
      return;
    }
    let canonicalManifest: string;
    try {
      canonicalManifest = canonicalManifestJSON(manifestRawText);
    } catch {
      errors.push(this.makeError("PACKAGE_MANIFEST_INVALID", "manifest is not valid JSON", "error"));
      return;
    }
    const manifestBytes = Buffer.byteLength(canonicalManifest, "utf8");
    const entries: TreeHashEntry[] = manifest.integrity.files.map((f) => ({
      path: f.path,
      sha256: f.sha256,
      bytes: f.bytes,
    }));
    entries.push({
      path: MANIFEST_PSEUDO_ENTRY_PATH,
      sha256: manifest.integrity.manifestHash,
      bytes: manifestBytes,
    });
    const computed = computeTreeHash(entries);
    if (computed !== declaredHash) {
      errors.push(this.makeError(
        "PACKAGE_HASH_MISMATCH",
        `contentRootHash mismatch: expected ${declaredHash}, got ${computed}`,
        "error",
        { expected: declaredHash, actual: computed },
      ));
    }
  }

  private verifyFileCount(
    manifest: NormalizedManifestData,
    errors: PackageValidationError[],
  ): void {
    const declared = manifest.integrity.fileCount;
    const actual = manifest.integrity.files.length;
    if (declared !== actual) {
      errors.push(this.makeError(
        "PACKAGE_MANIFEST_INVALID",
        `fileCount mismatch: expected ${declared}, got ${actual}`,
        "error",
        { expected: String(declared), actual: String(actual) },
      ));
    }
    let totalBytes = 0;
    for (const f of manifest.integrity.files) {
      totalBytes += f.bytes;
    }
    if (manifest.integrity.totalBytes !== totalBytes) {
      errors.push(this.makeError(
        "PACKAGE_MANIFEST_INVALID",
        `totalBytes mismatch: expected ${manifest.integrity.totalBytes}, got ${totalBytes}`,
        "error",
        { expected: String(manifest.integrity.totalBytes), actual: String(totalBytes) },
      ));
    }
  }

  private async verifyFiles(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): Promise<void> {
    const { manifest, installPath } = params;
    for (const f of manifest.integrity.files) {
      const fullPath = join(installPath, f.path);
      let content: Buffer;
      try {
        content = await readFile(fullPath);
      } catch {
        errors.push(this.makeError(
          "PACKAGE_FILE_MISSING",
          `file missing: ${f.path}`,
          "error",
          { path: f.path },
        ));
        continue;
      }
      if (content.length !== f.bytes) {
        errors.push(this.makeError(
          "PACKAGE_HASH_MISMATCH",
          `size mismatch for ${f.path}: expected ${f.bytes}, got ${content.length}`,
          "error",
          { path: f.path, expected: String(f.bytes), actual: String(content.length) },
        ));
      }
      const computed = sha256Hex(content);
      if (computed !== f.sha256) {
        errors.push(this.makeError(
          "PACKAGE_HASH_MISMATCH",
          `hash mismatch for ${f.path}: expected ${f.sha256}, got ${computed}`,
          "error",
          { path: f.path, expected: f.sha256, actual: computed },
        ));
      }
    }
  }

  private async verifyUndeclaredFiles(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): Promise<void> {
    const { manifest, installPath } = params;
    const declared = new Set(
      manifest.integrity.files.map((f) => normalizePathKey(f.path)),
    );
    let allFiles: string[];
    try {
      allFiles = await listAllFiles(installPath);
    } catch {
      return;
    }
    const manifestRelativePath = normalizePathKey(relative(installPath, params.manifestPath));
    if (manifestRelativePath !== "manifest.json") {
      errors.push(this.makeError(
        "PACKAGE_MANIFEST_INVALID",
        `manifest must be located at package root as manifest.json, got ${manifestRelativePath}`,
        "error",
        { path: manifestRelativePath },
      ));
    }
    for (const relPath of allFiles) {
      const key = normalizePathKey(relPath);
      if (key === "manifest.json") continue;
      if (!declared.has(key)) {
        errors.push(this.makeError(
          "PACKAGE_FILE_UNDECLARED",
          `undeclared file: ${relPath}`,
          "error",
          { path: relPath },
        ));
      }
    }
  }

  private async verifyActionConfigs(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): Promise<void> {
    const { manifest, installPath } = params;
    for (const entry of manifest.actionEntries) {
      const fullPath = join(installPath, entry.config);
      let content: Buffer;
      try {
        content = await readFile(fullPath);
      } catch {
        errors.push(this.makeError(
          "ACTION_CONFIG_MISSING",
          `action config missing: ${entry.config}`,
          "error",
          { path: entry.config, actionKey: entry.key },
        ));
        continue;
      }
      const computed = sha256Hex(content);
      const declared = manifest.integrity.files.find(
        (f) => normalizePathKey(f.path) === normalizePathKey(entry.config),
      );
      if (declared && computed !== declared.sha256) {
        errors.push(this.makeError(
          "PACKAGE_HASH_MISMATCH",
          `action config hash mismatch for ${entry.config}`,
          "error",
          { path: entry.config, actionKey: entry.key, expected: declared.sha256, actual: computed },
        ));
      }
    }
  }

  private async verifyPreview(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): Promise<void> {
    const { manifest, installPath } = params;
    if (!manifest.preview) return;
    const fullPath = join(installPath, manifest.preview);
    let content: Buffer;
    try {
      content = await readFile(fullPath);
    } catch {
      errors.push(this.makeError(
        "PACKAGE_FILE_MISSING",
        `preview missing: ${manifest.preview}`,
        "error",
        { path: manifest.preview },
      ));
      return;
    }
    const computed = sha256Hex(content);
    const declared = manifest.integrity.files.find(
      (f) => normalizePathKey(f.path) === normalizePathKey(manifest.preview!),
    );
    if (declared && computed !== declared.sha256) {
      errors.push(this.makeError(
        "PACKAGE_HASH_MISMATCH",
        `preview hash mismatch for ${manifest.preview}`,
        "error",
        { path: manifest.preview, expected: declared.sha256, actual: computed },
      ));
    }
    if (!declared) {
      errors.push(this.makeError(
        "PACKAGE_PREVIEW_UNDECLARED",
        `preview not declared in integrity files: ${manifest.preview}`,
        "error",
        { path: manifest.preview },
      ));
    }
  }

  private async verifyFrames(
    params: VerifyParams,
    errors: PackageValidationError[],
  ): Promise<void> {
    const { manifest, installPath, actions } = params;
    for (const [actionKey, action] of actions) {
      const actionBase = dirname(action.configPath);
      for (const frame of action.frames) {
        const packageRelativeFramePath = normalizePathKey(join(actionBase, frame.file));
        const fullPath = join(installPath, packageRelativeFramePath);
        let content: Buffer;
        try {
          content = await readFile(fullPath);
        } catch {
          errors.push(this.makeError(
            "FRAME_MISSING",
            `frame missing: ${packageRelativeFramePath} (action: ${actionKey})`,
            "error",
            { path: packageRelativeFramePath, actionKey },
          ));
          continue;
        }
        const computed = sha256Hex(content);
        if (computed !== frame.contentHash) {
          errors.push(this.makeError(
            "FRAME_HASH_MISMATCH",
            `frame hash mismatch for ${packageRelativeFramePath} (action: ${actionKey})`,
            "error",
            { path: packageRelativeFramePath, actionKey, expected: frame.contentHash, actual: computed },
          ));
        }
        const declared = manifest.integrity.files.find(
          (f) => normalizePathKey(f.path) === packageRelativeFramePath,
        );
        if (declared && computed !== declared.sha256) {
          errors.push(this.makeError(
            "PACKAGE_RESOURCE_HASH_MISMATCH",
            `frame resource hash mismatch for ${packageRelativeFramePath} (action: ${actionKey})`,
            "error",
            { path: packageRelativeFramePath, actionKey, expected: declared.sha256, actual: computed },
          ));
        }
        if (!declared) {
          errors.push(this.makeError(
            "PACKAGE_RESOURCE_NOT_DECLARED",
            `frame resource not declared: ${packageRelativeFramePath} (action: ${actionKey})`,
            "error",
            { path: packageRelativeFramePath, actionKey },
          ));
        }
      }
    }
  }

  private verifyMediaTypes(
    manifest: NormalizedManifestData,
    errors: PackageValidationError[],
  ): void {
    for (const f of manifest.integrity.files) {
      const ext = extname(f.path).toLowerCase();
      const expected = MIME_MAP[ext];
      if (expected && expected !== f.mediaType) {
        errors.push(this.makeError(
          "PACKAGE_MEDIA_TYPE_MISMATCH",
          `mediaType mismatch for ${f.path}: expected ${expected}, got ${f.mediaType}`,
          "warning",
          { path: f.path, expected, actual: f.mediaType },
        ));
      }
    }
  }

  private async computeFingerprint(
    installPath: string,
    manifestRawText: string,
  ): Promise<string> {
    const h = createHash("sha256");
    h.update(sha256Hex(manifestRawText));
    h.update("\0");
    let files: string[] = [];
    try {
      files = await listAllFiles(installPath);
      files.sort();
    } catch {
      h.update("list-failed");
      return h.digest("hex");
    }
    for (const relPath of files) {
      const fullPath = join(installPath, relPath);
      try {
        const s = await stat(fullPath);
        h.update(relPath);
        h.update("\0");
        h.update(String(s.size));
        h.update("\0");
        h.update(String(s.mtimeMs));
        h.update("\0");
        h.update(String(s.ctimeMs));
        h.update("\0");
      } catch {
        h.update(relPath);
        h.update("\0missing\0");
      }
    }
    return h.digest("hex");
  }

  private makeError(
    code: PackageErrorCode,
    message: string,
    severity: string,
    details?: { path?: string; actionKey?: string; expected?: string; actual?: string },
  ): PackageValidationError {
    return {
      code,
      severity,
      message,
      path: details?.path,
      actionKey: details?.actionKey,
      expected: details?.expected,
      actual: details?.actual,
    };
  }
}

interface TreeHashEntry {
  path: string;
  sha256: string;
  bytes: number;
}

function computeTreeHash(entries: TreeHashEntry[]): string {
  const sorted = [...entries].sort((a, b) =>
    a.path < b.path ? -1 : a.path > b.path ? 1 : 0,
  );
  const h = createHash("sha256");
  for (const e of sorted) {
    h.update("file");
    h.update("\0");
    h.update(e.path);
    h.update("\0");
    h.update(String(e.bytes));
    h.update("\0");
    const isDecodableHex =
      e.sha256.length % 2 === 0 && /^[0-9a-fA-F]*$/.test(e.sha256);
    const rawHash = isDecodableHex
      ? Buffer.from(e.sha256, "hex")
      : Buffer.from(e.sha256, "utf8");
    h.update(rawHash);
    h.update("\0");
  }
  return h.digest("hex");
}

function sha256Hex(data: string | Buffer): string {
  return createHash("sha256").update(data).digest("hex");
}

function canonicalJSON(value: unknown): string {
  const canonical = canonicalizeValue(value);
  // Go's encoding/json escapes HTML-significant characters and U+2028/U+2029
  // by default. Package v2 hashes are authored by the Go backend, so the
  // Electron implementation must reproduce those bytes exactly rather than
  // relying on JavaScript's slightly different JSON.stringify escaping.
  return JSON.stringify(canonical)
    .replace(/&/g, "\\u0026")
    .replace(/</g, "\\u003c")
    .replace(/>/g, "\\u003e")
    .replace(/\u2028/g, "\\u2028")
    .replace(/\u2029/g, "\\u2029");
}

function canonicalizeValue(v: unknown): unknown {
  if (v !== null && typeof v === "object") {
    if (Array.isArray(v)) {
      return v.map(canonicalizeValue);
    }
    const obj = v as Record<string, unknown>;
    const keys = Object.keys(obj).sort();
    return keys.map((k) => ({ k, v: canonicalizeValue(obj[k]) }));
  }
  return v;
}

function canonicalManifestJSON(manifestRawText: string): string {
  const parsed = JSON.parse(manifestRawText) as Record<string, unknown>;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("manifest must be an object");
  }
  return canonicalJSON(projectGoManifestForHash(parsed));
}

/**
 * Reproduce the JSON shape produced by Go's packageformat.Manifest before
 * CanonicalJSON runs. The backend hashes the strongly typed Go struct, not the
 * original arbitrary JSON object. That distinction matters for zero-value
 * struct fields and the few `omitempty` fields.
 */
function projectGoManifestForHash(raw: Record<string, unknown>): Record<string, unknown> {
  const author = asRecord(raw.author);
  const license = asRecord(raw.license);
  const compatibility = asRecord(raw.compatibility);
  const binding = asRecord(raw.binding);
  const canvas = asRecord(raw.canvas);
  const capabilities = asRecord(raw.capabilities);
  const integrity = asRecord(raw.integrity);
  const provenance = asRecord(raw.provenance);

  const projectedCompatibility: Record<string, unknown> = {
    minRuntimeVersion: asString(compatibility.minRuntimeVersion),
    renderMode: asString(compatibility.renderMode),
  };
  if (typeof compatibility.maxRuntimeVersion === "string") {
    projectedCompatibility.maxRuntimeVersion = compatibility.maxRuntimeVersion;
  }

  const rawActions = Array.isArray(raw.actions) ? raw.actions : null;
  const projectedActions = rawActions === null
    ? null
    : rawActions.map((item) => {
        const action = asRecord(item);
        return {
          key: asString(action.key),
          name: asString(action.name),
          config: asString(action.config),
          revisionId: asString(action.revisionId),
          qualityEvaluationId: asString(action.qualityEvaluationId),
          qualityVerdict: asString(action.qualityVerdict),
          playbackMode: asString(action.playbackMode),
          fps: asInteger(action.fps),
          frameCount: asInteger(action.frameCount),
          supportsDefaultIdle: asBoolean(action.supportsDefaultIdle),
          isStableStateCandidate: asBoolean(action.isStableStateCandidate),
          isTransitionOnly: asBoolean(action.isTransitionOnly),
        };
      });

  const rawFiles = Array.isArray(integrity.files) ? integrity.files : null;
  const projectedFiles = rawFiles === null
    ? null
    : rawFiles.map((item) => {
        const file = asRecord(item);
        const projected: Record<string, unknown> = {
          path: asString(file.path),
          sha256: asString(file.sha256),
          bytes: asInteger(file.bytes),
          mediaType: asString(file.mediaType),
          role: asString(file.role),
        };
        const actionKey = asString(file.actionKey);
        const frameId = asString(file.frameId);
        if (actionKey) projected.actionKey = actionKey;
        if (frameId) projected.frameId = frameId;
        return projected;
      });

  return {
    schemaVersion: asInteger(raw.schemaVersion),
    manifestFormat: asString(raw.manifestFormat),
    petId: asString(raw.petId),
    releaseId: asString(raw.releaseId),
    version: asString(raw.version),
    name: asString(raw.name),
    description: asString(raw.description),
    author: {
      name: asString(author.name),
      id: asString(author.id),
    },
    license: {
      spdx: asString(license.spdx),
      noticePath: asString(license.noticePath),
    },
    compatibility: projectedCompatibility,
    binding: {
      policy: asString(binding.policy),
      sourceCharacterId: asString(binding.sourceCharacterId),
    },
    canvas: {
      width: asInteger(canvas.width),
      height: asInteger(canvas.height),
      coordinateSystem: asString(canvas.coordinateSystem),
    },
    defaultAction: asString(raw.defaultAction),
    preview: asString(raw.preview),
    actions: projectedActions,
    capabilities: {
      transparentBackground: asBoolean(capabilities.transparentBackground),
      frameSequence: asBoolean(capabilities.frameSequence),
      perFrameDuration: asBoolean(capabilities.perFrameDuration),
      audio: asBoolean(capabilities.audio),
    },
    integrity: {
      algorithm: asString(integrity.algorithm),
      manifestHash: "",
      contentRootHash: "",
      fileCount: asInteger(integrity.fileCount),
      totalBytes: asInteger(integrity.totalBytes),
      files: projectedFiles,
    },
    provenance: {
      sourceType: asString(provenance.sourceType),
      generationTaskId: asString(provenance.generationTaskId),
      processingTaskId: asString(provenance.processingTaskId),
      builtAt: asString(provenance.builtAt),
      builder: asString(provenance.builder),
    },
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asInteger(value: unknown): number {
  return typeof value === "number" && Number.isInteger(value) ? value : 0;
}

function asBoolean(value: unknown): boolean {
  return typeof value === "boolean" ? value : false;
}

function normalizePathKey(path: string): string {
  return path.replace(/\\/g, "/").replace(/^\.\/+/, "");
}

async function listAllFiles(root: string): Promise<string[]> {
  const results: string[] = [];
  const realRoot = safeRealpath(root);
  await walkDir(realRoot, realRoot, results);
  return results;
}

function safeRealpath(p: string): string {
  try {
    return realpathSync(p);
  } catch {
    return p;
  }
}

async function walkDir(root: string, current: string, results: string[]): Promise<void> {
  let entries: import("node:fs").Dirent[];
  try {
    entries = await readdir(current, { withFileTypes: true });
  } catch {
    return;
  }
  for (const entry of entries) {
    const fullPath = join(current, entry.name);
    if (entry.isDirectory()) {
      await walkDir(root, fullPath, results);
    } else if (entry.isFile()) {
      const rel = relative(root, fullPath).replace(/\\/g, "/");
      results.push(rel);
    }
  }
}

export { computeTreeHash, sha256Hex, canonicalJSON, canonicalManifestJSON };
