import {
  createPackageError,
  type PackageErrorCode,
  type PackageValidationError,
} from "./package-errors";

export type PlaybackMode = "loop" | "once" | "hold" | "ping_pong";

export type ReturnToRule =
  | { type: "action"; actionKey: string }
  | { type: "default" }
  | { type: "previous" }
  | { type: "current_activity" }
  | { type: "none" };

export type QualityVerdict =
  | "accepted"
  | "accepted_with_warning"
  | "needs_review"
  | "rejected";

export type LegacyQualityVerdict = "pass" | "warning" | "fail" | "skipped";

export const INTEGRITY_ALGORITHM_V2 = "amitia-package-sha256-v2";
export const INTEGRITY_ALGORITHM_V1_LEGACY = "amitia-tree-sha256-v1";
export const MANIFEST_FORMAT_CANONICAL = "amitia-desktop-pet";
export const MANIFEST_SCHEMA_VERSION = 2;
export const MANIFEST_PSEUDO_ENTRY_PATH = "@manifest";

export interface RuntimeAnchor {
  x: number;
  y: number;
  coordinateSpace: "normalized_canvas";
}

export interface RuntimeFrame {
  frameId: string;
  index: number;
  file: string;
  durationMs: number;
  assetId: string;
  contentHash: string;
}

export interface RuntimeAction {
  actionKey: string;
  displayName: string;
  fps: number;
  playbackMode: PlaybackMode;
  interruptible: boolean;
  priority: number;
  cooldownMs: number;
  minimumPlayMs: number;
  maximumPlayMs: number;
  mutexGroup: string;
  returnTo: ReturnToRule;
  anchor: RuntimeAnchor;
  frames: RuntimeFrame[];
  configPath: string;
  version: number;
  supportsDefaultIdle: boolean;
  isStableStateCandidate: boolean;
  isTransitionOnly: boolean;
}

export interface RuntimeIntegrityFile {
  path: string;
  sha256: string;
  bytes: number;
  mediaType: string;
  role: string;
  actionKey?: string;
  frameId?: string;
}

export interface RuntimePetPackage {
  schemaVersion: 2;
  petId: string;
  releaseId: string;
  packageRoot: string;
  displayName: string;
  defaultActionKey: string;
  preview: string | null;
  canvas: { width: number; height: number; coordinateSystem: "top-left" };
  compatibility: {
    minRuntimeVersion: string;
    maxRuntimeVersion: string | null;
    renderMode: "sprite";
  };
  actions: Map<string, RuntimeAction>;
  integrity: {
    algorithm: string;
    manifestHash: string;
    contentRootHash: string;
    fileCount: number;
    totalBytes: number;
    files: RuntimeIntegrityFile[];
  };
  sourceSchemaVersion: 1 | 2;
}

export interface LegacyWarning {
  code: string;
  message: string;
  path?: string;
  actionKey?: string;
}

export interface RuntimeResourceIndexEntry {
  path: string;
  sha256: string;
  bytes: number;
  mediaType: string;
  role: string;
  actionKey?: string;
  frameId?: string;
}

export interface LoadInstallationRequest {
  installationId: string;
  petId: string;
  releaseId: string;
  installPath: string;
  manifestPath: string;
  expectedContentRootHash: string;
}

export interface ManifestActionEntry {
  key: string;
  name: string;
  config: string;
  revisionId?: string;
  qualityEvaluationId?: string;
  qualityVerdict?: QualityVerdict;
  playbackMode: PlaybackMode;
  fps: number;
  frameCount: number;
  supportsDefaultIdle: boolean;
  isStableStateCandidate: boolean;
  isTransitionOnly: boolean;
}

export interface NormalizedIntegrity {
  algorithm: string;
  manifestHash: string;
  contentRootHash: string;
  fileCount: number;
  totalBytes: number;
  files: RuntimeIntegrityFile[];
}

export interface NormalizedManifestData {
  schemaVersion: number;
  manifestFormat: string;
  petId: string;
  releaseId: string;
  version: string;
  displayName: string;
  description: string;
  defaultActionKey: string;
  preview: string | null;
  canvas: { width: number; height: number; coordinateSystem: "top-left" };
  actionEntries: ManifestActionEntry[];
  compatibility: {
    minRuntimeVersion: string;
    maxRuntimeVersion: string | null;
    renderMode: "sprite";
  };
  binding: { policy: string; sourceCharacterId?: string };
  integrity: NormalizedIntegrity;
}

export interface ManifestReadResult {
  data: NormalizedManifestData;
  warnings: LegacyWarning[];
}

export interface ActionReadResult {
  action: RuntimeAction;
  warnings: LegacyWarning[];
}

export interface PackageReader {
  readManifest(raw: unknown): ManifestReadResult;
  readAction(raw: unknown, actionKey: string, configPath: string): ActionReadResult;
}

export interface StrictPackageContractReader extends PackageReader {}

const SEMVER_RE =
  /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$/;

const SHA256_RE = /^[0-9a-f]{64}$/;

const VALID_PLAYBACK_MODES: readonly PlaybackMode[] = [
  "loop",
  "once",
  "hold",
  "ping_pong",
];

const VALID_RETURN_TO_TYPES: readonly string[] = [
  "action",
  "default",
  "previous",
  "current_activity",
  "none",
];

const VALID_QUALITY_VERDICTS: readonly QualityVerdict[] = [
  "accepted",
  "accepted_with_warning",
  "needs_review",
  "rejected",
];

export function isValidSemVer(v: string): boolean {
  return typeof v === "string" && SEMVER_RE.test(v);
}

export function isValidSha256(v: string): boolean {
  return typeof v === "string" && SHA256_RE.test(v);
}

export function compareVersions(minVersion: string, currentVersion: string): boolean {
  if (!isValidSemVer(minVersion)) {
    throw createPackageError(
      "PACKAGE_RUNTIME_VERSION_INVALID" as PackageErrorCode,
      `invalid semver for minVersion: ${minVersion}`,
      { actual: minVersion },
    );
  }
  if (!isValidSemVer(currentVersion)) {
    throw createPackageError(
      "PACKAGE_RUNTIME_VERSION_INVALID" as PackageErrorCode,
      `invalid semver for currentVersion: ${currentVersion}`,
      { actual: currentVersion },
    );
  }
  const minParsed = parseSemVer(minVersion);
  const curParsed = parseSemVer(currentVersion);
  const cmp = compareSemVer(curParsed, minParsed);
  return cmp >= 0;
}

interface ParsedSemVer {
  major: number;
  minor: number;
  patch: number;
  prerelease: string | null;
  build: string | null;
}

function parseSemVer(v: string): ParsedSemVer {
  const match = v.match(SEMVER_RE);
  if (!match) {
    throw createPackageError(
      "PACKAGE_RUNTIME_VERSION_INVALID" as PackageErrorCode,
      `invalid semver: ${v}`,
      { actual: v },
    );
  }
  return {
    major: parseInt(match[1], 10),
    minor: parseInt(match[2], 10),
    patch: parseInt(match[3], 10),
    prerelease: match[4] ?? null,
    build: match[5] ?? null,
  };
}

function compareSemVer(a: ParsedSemVer, b: ParsedSemVer): number {
  if (a.major !== b.major) return a.major - b.major;
  if (a.minor !== b.minor) return a.minor - b.minor;
  if (a.patch !== b.patch) return a.patch - b.patch;
  if (a.prerelease === null && b.prerelease === null) return 0;
  if (a.prerelease === null) return 1;
  if (b.prerelease === null) return -1;
  return comparePrerelease(a.prerelease, b.prerelease);
}

function comparePrerelease(a: string, b: string): number {
  const partsA = a.split(".");
  const partsB = b.split(".");
  const len = Math.min(partsA.length, partsB.length);
  for (let i = 0; i < len; i++) {
    const pa = partsA[i];
    const pb = partsB[i];
    const na = parseInt(pa, 10);
    const nb = parseInt(pb, 10);
    const aIsNum = !Number.isNaN(na) && String(na) === pa;
    const bIsNum = !Number.isNaN(nb) && String(nb) === pb;
    if (aIsNum && bIsNum) {
      if (na !== nb) return na - nb;
      continue;
    }
    if (aIsNum) return -1;
    if (bIsNum) return 1;
    if (pa !== pb) return pa < pb ? -1 : 1;
  }
  return partsA.length - partsB.length;
}

function assertCondition(
  condition: boolean,
  code: PackageErrorCode,
  message: string,
  details?: { path?: string; actionKey?: string; expected?: string; actual?: string },
): asserts condition {
  if (!condition) {
    throw createPackageError(code, message, details);
  }
}

function requireString(value: unknown, field: string, code: PackageErrorCode): string {
  assertCondition(
    typeof value === "string" && value.length > 0,
    code,
    `${field} is required and must be a non-empty string`,
  );
  return value as string;
}

function requireNumber(value: unknown, field: string, code: PackageErrorCode): number {
  assertCondition(
    typeof value === "number" && Number.isFinite(value),
    code,
    `${field} is required and must be a finite number`,
  );
  return value as number;
}

function requireBoolean(value: unknown, field: string, code: PackageErrorCode): boolean {
  assertCondition(
    typeof value === "boolean",
    code,
    `${field} is required and must be a boolean`,
  );
  return value as boolean;
}

function normalizePlaybackModeValue(
  raw: unknown,
  actionKey: string,
): PlaybackMode {
  if (typeof raw !== "string" || raw.length === 0) {
    throw createPackageError(
      "PACKAGE_MANIFEST_INVALID",
      `playbackMode is required (action: ${actionKey})`,
      { actionKey },
    );
  }
  const lt = raw.toLowerCase().trim();
  if (lt === "ping-pong" || lt === "pingpong") {
    return "ping_pong";
  }
  if ((VALID_PLAYBACK_MODES as readonly string[]).includes(lt)) {
    return lt as PlaybackMode;
  }
  throw createPackageError(
    "PACKAGE_MANIFEST_INVALID",
    `UNKNOWN_PLAYBACK_MODE: ${raw} (action: ${actionKey})`,
    { actionKey, actual: raw },
  );
}

function normalizeReturnToRule(
  returnTo: unknown,
  actionKey: string,
): ReturnToRule {
  if (!returnTo || typeof returnTo !== "object") {
    throw createPackageError(
      "PACKAGE_MANIFEST_INVALID",
      `returnTo is required (action: ${actionKey})`,
      { actionKey },
    );
  }
  const rt = returnTo as { type?: string; actionKey?: string };
  const type = rt.type;
  if (typeof type !== "string" || type.length === 0) {
    throw createPackageError(
      "PACKAGE_MANIFEST_INVALID",
      `returnTo.type is required (action: ${actionKey})`,
      { actionKey },
    );
  }
  if (!(VALID_RETURN_TO_TYPES as readonly string[]).includes(type)) {
    throw createPackageError(
      "PACKAGE_MANIFEST_INVALID",
      `UNKNOWN_RETURN_TO_TYPE: ${type} (action: ${actionKey})`,
      { actionKey, actual: type },
    );
  }
  switch (type) {
    case "action": {
      if (typeof rt.actionKey !== "string" || rt.actionKey.length === 0) {
        throw createPackageError(
          "PACKAGE_MANIFEST_INVALID",
          `INVALID_RETURN_TO_ACTION_KEY: ${actionKey}`,
          { actionKey },
        );
      }
      return { type: "action", actionKey: rt.actionKey };
    }
    case "default":
      return { type: "default" };
    case "previous":
      return { type: "previous" };
    case "current_activity":
      return { type: "current_activity" };
    case "none":
      return { type: "none" };
    default:
      throw createPackageError(
        "PACKAGE_MANIFEST_INVALID",
        `UNKNOWN_RETURN_TO_TYPE: ${type} (action: ${actionKey})`,
        { actionKey, actual: type },
      );
  }
}

function normalizeAnchor(anchor: unknown, actionKey: string): RuntimeAnchor {
  if (!anchor || typeof anchor !== "object") {
    throw createPackageError(
      "PACKAGE_MANIFEST_INVALID",
      `anchor is required (action: ${actionKey})`,
      { actionKey },
    );
  }
  const a = anchor as { x?: number; y?: number; coordinateSpace?: string };
  const x = a.x;
  const y = a.y;
  assertCondition(
    typeof x === "number" && Number.isFinite(x) && x >= 0 && x <= 1,
    "PACKAGE_MANIFEST_INVALID",
    `anchor.x must be between 0 and 1 (action: ${actionKey})`,
    { actionKey },
  );
  assertCondition(
    typeof y === "number" && Number.isFinite(y) && y >= 0 && y <= 1,
    "PACKAGE_MANIFEST_INVALID",
    `anchor.y must be between 0 and 1 (action: ${actionKey})`,
    { actionKey },
  );
  assertCondition(
    a.coordinateSpace === "normalized_canvas",
    "PACKAGE_MANIFEST_INVALID",
    `anchor.coordinateSpace must be normalized_canvas (action: ${actionKey})`,
    { actionKey, actual: a.coordinateSpace },
  );
  return { x, y, coordinateSpace: "normalized_canvas" };
}

function normalizeFramesStrict(
  frames: unknown,
  actionKey: string,
): RuntimeFrame[] {
  assertCondition(
    Array.isArray(frames) && frames.length > 0,
    "PACKAGE_MANIFEST_INVALID",
    `frames is required and must be a non-empty array (action: ${actionKey})`,
    { actionKey },
  );
  const result: RuntimeFrame[] = [];
  const seenFrameIds = new Set<string>();
  for (let i = 0; i < frames.length; i++) {
    const item = frames[i];
    assertCondition(
      item !== null && typeof item === "object",
      "PACKAGE_MANIFEST_INVALID",
      `frame[${i}] must be an object (action: ${actionKey})`,
      { actionKey },
    );
    const f = item as {
      frameId?: string;
      index?: number;
      file?: string;
      durationMs?: number;
      assetId?: string;
      contentHash?: string;
    };
    const frameId = requireString(f.frameId, `frame[${i}].frameId`, "PACKAGE_MANIFEST_INVALID");
    if (seenFrameIds.has(frameId)) {
      throw createPackageError(
        "PACKAGE_FRAME_ID_DUPLICATE",
        `duplicate frameId: ${frameId} (action: ${actionKey})`,
        { actionKey, actual: frameId },
      );
    }
    seenFrameIds.add(frameId);
    const index = requireNumber(f.index, `frame[${i}].index`, "PACKAGE_MANIFEST_INVALID");
    const file = requireString(f.file, `frame[${i}].file`, "PACKAGE_MANIFEST_INVALID");
    const durationMs = requireNumber(f.durationMs, `frame[${i}].durationMs`, "PACKAGE_MANIFEST_INVALID");
    assertCondition(
      durationMs >= 8 && durationMs <= 60000,
      "PACKAGE_MANIFEST_INVALID",
      `frame[${i}].durationMs must be between 8 and 60000 (action: ${actionKey})`,
      { actionKey },
    );
    const assetId = requireString(f.assetId, `frame[${i}].assetId`, "PACKAGE_FRAME_ASSET_ID_MISSING");
    const contentHash = requireString(f.contentHash, `frame[${i}].contentHash`, "PACKAGE_MANIFEST_INVALID");
    assertCondition(
      SHA256_RE.test(contentHash),
      "PACKAGE_MANIFEST_INVALID",
      `frame[${i}].contentHash must be a valid sha256 (action: ${actionKey})`,
      { actionKey, actual: contentHash },
    );
    result.push({ frameId, index, file, durationMs, assetId, contentHash });
  }
  return result;
}

function mapLegacyQualityVerdict(verdict: string): QualityVerdict {
  switch (verdict) {
    case "pass":
      return "accepted";
    case "warning":
      return "accepted_with_warning";
    case "fail":
      return "rejected";
    case "skipped":
      return "needs_review";
    default:
      return verdict as QualityVerdict;
  }
}

export class Schema2PackageReader implements StrictPackageContractReader {
  readManifest(raw: unknown): ManifestReadResult {
    assertCondition(
      raw !== null && typeof raw === "object",
      "PACKAGE_SCHEMA_MISSING",
      "manifest is missing or not an object",
    );
    const m = raw as Record<string, unknown>;

    const schemaVersion = m.schemaVersion;
    assertCondition(
      typeof schemaVersion === "number" && schemaVersion === 2,
      "PACKAGE_SCHEMA_UNSUPPORTED",
      `schemaVersion must be 2, got ${schemaVersion}`,
      { expected: "2", actual: String(schemaVersion) },
    );

    const manifestFormat = m.manifestFormat;
    assertCondition(
      typeof manifestFormat === "string" && manifestFormat === MANIFEST_FORMAT_CANONICAL,
      "PACKAGE_MANIFEST_INVALID",
      `manifestFormat must be ${MANIFEST_FORMAT_CANONICAL}`,
      { expected: MANIFEST_FORMAT_CANONICAL, actual: String(manifestFormat) },
    );

    const petId = requireString(m.petId, "petId", "PACKAGE_MANIFEST_INVALID");
    const releaseId = requireString(m.releaseId, "releaseId", "PACKAGE_MANIFEST_INVALID");
    const version = requireString(m.version, "version", "PACKAGE_MANIFEST_INVALID");
    assertCondition(
      isValidSemVer(version),
      "PACKAGE_RUNTIME_VERSION_INVALID",
      `version must be a valid semver: ${version}`,
      { actual: version },
    );
    const displayName = requireString(m.name, "name", "PACKAGE_MANIFEST_INVALID");

    const description = typeof m.description === "string" ? m.description : "";

    const defaultActionKey = requireString(m.defaultAction, "defaultAction", "PACKAGE_MANIFEST_INVALID");

    const previewRaw = m.preview;
    assertCondition(
      previewRaw === null || typeof previewRaw === "string",
      "PACKAGE_MANIFEST_INVALID",
      "preview must be a string or null",
    );
    const preview = previewRaw === null ? null : previewRaw;

    const canvasRaw = m.canvas;
    assertCondition(
      canvasRaw !== null && typeof canvasRaw === "object",
      "PACKAGE_MANIFEST_INVALID",
      "canvas is required",
    );
    const cv = canvasRaw as { width?: number; height?: number; coordinateSystem?: string };
    const canvasWidth = requireNumber(cv.width, "canvas.width", "PACKAGE_MANIFEST_INVALID");
    const canvasHeight = requireNumber(cv.height, "canvas.height", "PACKAGE_MANIFEST_INVALID");
    assertCondition(
      canvasWidth >= 1 && canvasWidth <= 4096,
      "PACKAGE_MANIFEST_INVALID",
      "canvas.width must be between 1 and 4096",
    );
    assertCondition(
      canvasHeight >= 1 && canvasHeight <= 4096,
      "PACKAGE_MANIFEST_INVALID",
      "canvas.height must be between 1 and 4096",
    );
    assertCondition(
      cv.coordinateSystem === "top-left",
      "PACKAGE_MANIFEST_INVALID",
      "canvas.coordinateSystem must be top-left",
      { actual: String(cv.coordinateSystem) },
    );

    const actionsRaw = m.actions;
    assertCondition(
      Array.isArray(actionsRaw) && actionsRaw.length > 0,
      "PACKAGE_MANIFEST_INVALID",
      "actions is required and must be a non-empty array",
    );
    const actionEntries: ManifestActionEntry[] = [];
    for (let i = 0; i < actionsRaw.length; i++) {
      const entry = actionsRaw[i];
      assertCondition(
        entry !== null && typeof entry === "object",
        "PACKAGE_MANIFEST_INVALID",
        `actions[${i}] must be an object`,
      );
      const ae = entry as Record<string, unknown>;
      const key = requireString(ae.key, `actions[${i}].key`, "PACKAGE_MANIFEST_INVALID");
      const name = requireString(ae.name, `actions[${i}].name`, "PACKAGE_MANIFEST_INVALID");
      const config = requireString(ae.config, `actions[${i}].config`, "PACKAGE_MANIFEST_INVALID");
      const playbackMode = normalizePlaybackModeValue(ae.playbackMode, key);
      const fps = requireNumber(ae.fps, `actions[${i}].fps`, "PACKAGE_MANIFEST_INVALID");
      assertCondition(fps >= 1 && fps <= 120, "PACKAGE_MANIFEST_INVALID", `actions[${i}].fps must be between 1 and 120`);
      const frameCount = requireNumber(ae.frameCount, `actions[${i}].frameCount`, "PACKAGE_MANIFEST_INVALID");
      assertCondition(frameCount >= 1, "PACKAGE_MANIFEST_INVALID", `actions[${i}].frameCount must be >= 1`);
      const supportsDefaultIdle = requireBoolean(ae.supportsDefaultIdle, `actions[${i}].supportsDefaultIdle`, "PACKAGE_MANIFEST_INVALID");
      const isStableStateCandidate = requireBoolean(ae.isStableStateCandidate, `actions[${i}].isStableStateCandidate`, "PACKAGE_MANIFEST_INVALID");
      const isTransitionOnly = requireBoolean(ae.isTransitionOnly, `actions[${i}].isTransitionOnly`, "PACKAGE_MANIFEST_INVALID");

      let qualityVerdict: QualityVerdict | undefined;
      if (ae.qualityVerdict !== undefined && ae.qualityVerdict !== null) {
        const qv = ae.qualityVerdict as string;
        if ((VALID_QUALITY_VERDICTS as readonly string[]).includes(qv)) {
          qualityVerdict = qv as QualityVerdict;
        } else {
          qualityVerdict = mapLegacyQualityVerdict(qv);
        }
      }

      actionEntries.push({
        key,
        name,
        config,
        revisionId: typeof ae.revisionId === "string" ? ae.revisionId : undefined,
        qualityEvaluationId: typeof ae.qualityEvaluationId === "string" ? ae.qualityEvaluationId : undefined,
        qualityVerdict,
        playbackMode,
        fps,
        frameCount,
        supportsDefaultIdle,
        isStableStateCandidate,
        isTransitionOnly,
      });
    }

    const compatRaw = m.compatibility;
    assertCondition(
      compatRaw !== null && typeof compatRaw === "object",
      "PACKAGE_MANIFEST_INVALID",
      "compatibility is required",
    );
    const compat = compatRaw as { minRuntimeVersion?: string; maxRuntimeVersion?: string; renderMode?: string };
    const minRuntimeVersion = requireString(compat.minRuntimeVersion, "compatibility.minRuntimeVersion", "PACKAGE_MANIFEST_INVALID");
    assertCondition(
      isValidSemVer(minRuntimeVersion),
      "PACKAGE_RUNTIME_VERSION_INVALID",
      `compatibility.minRuntimeVersion must be a valid semver: ${minRuntimeVersion}`,
      { actual: minRuntimeVersion },
    );
    let maxRuntimeVersion: string | null = null;
    if (compat.maxRuntimeVersion !== undefined && compat.maxRuntimeVersion !== null) {
      assertCondition(
        typeof compat.maxRuntimeVersion === "string",
        "PACKAGE_MANIFEST_INVALID",
        "compatibility.maxRuntimeVersion must be a string or null",
      );
      assertCondition(
        isValidSemVer(compat.maxRuntimeVersion),
        "PACKAGE_RUNTIME_VERSION_INVALID",
        `compatibility.maxRuntimeVersion must be a valid semver: ${compat.maxRuntimeVersion}`,
        { actual: compat.maxRuntimeVersion },
      );
      maxRuntimeVersion = compat.maxRuntimeVersion;
    }
    assertCondition(
      compat.renderMode === "sprite",
      "PACKAGE_MANIFEST_INVALID",
      "compatibility.renderMode must be sprite",
      { actual: String(compat.renderMode) },
    );

    const bindingRaw = m.binding;
    assertCondition(
      bindingRaw !== null && typeof bindingRaw === "object",
      "PACKAGE_MANIFEST_INVALID",
      "binding is required",
    );
    const binding = bindingRaw as { policy?: string; sourceCharacterId?: string };
    assertCondition(
      typeof binding.policy === "string" && ["bound", "unbound", "legacy_inferred"].includes(binding.policy),
      "PACKAGE_MANIFEST_INVALID",
      "binding.policy must be one of bound, unbound, legacy_inferred",
      { actual: String(binding.policy) },
    );

    const integrityRaw = m.integrity;
    assertCondition(
      integrityRaw !== null && typeof integrityRaw === "object",
      "PACKAGE_INTEGRITY_MISSING",
      "integrity is required",
    );
    const ig = integrityRaw as Record<string, unknown>;
    const algorithm = requireString(ig.algorithm, "integrity.algorithm", "PACKAGE_INTEGRITY_ALGORITHM_UNSUPPORTED");
    assertCondition(
      algorithm === INTEGRITY_ALGORITHM_V2,
      "PACKAGE_INTEGRITY_ALGORITHM_UNSUPPORTED",
      `integrity.algorithm must be ${INTEGRITY_ALGORITHM_V2}`,
      { expected: INTEGRITY_ALGORITHM_V2, actual: algorithm },
    );
    const manifestHash = requireString(ig.manifestHash, "integrity.manifestHash", "PACKAGE_MANIFEST_HASH_MISSING");
    assertCondition(
      SHA256_RE.test(manifestHash),
      "PACKAGE_MANIFEST_HASH_MISSING",
      "integrity.manifestHash must be a valid sha256",
      { actual: manifestHash },
    );
    const contentRootHash = requireString(ig.contentRootHash, "integrity.contentRootHash", "PACKAGE_INTEGRITY_MISSING");
    assertCondition(
      SHA256_RE.test(contentRootHash),
      "PACKAGE_INTEGRITY_MISSING",
      "integrity.contentRootHash must be a valid sha256",
      { actual: contentRootHash },
    );
    const fileCount = requireNumber(ig.fileCount, "integrity.fileCount", "PACKAGE_MANIFEST_INVALID");
    assertCondition(fileCount >= 1, "PACKAGE_MANIFEST_INVALID", "integrity.fileCount must be >= 1");
    const totalBytes = requireNumber(ig.totalBytes, "integrity.totalBytes", "PACKAGE_MANIFEST_INVALID");
    assertCondition(totalBytes >= 0, "PACKAGE_MANIFEST_INVALID", "integrity.totalBytes must be >= 0");

    const filesRaw = ig.files;
    assertCondition(
      Array.isArray(filesRaw) && filesRaw.length > 0,
      "PACKAGE_INTEGRITY_MISSING",
      "integrity.files is required and must be a non-empty array",
    );
    const files: RuntimeIntegrityFile[] = [];
    for (let i = 0; i < filesRaw.length; i++) {
      const f = filesRaw[i];
      assertCondition(
        f !== null && typeof f === "object",
        "PACKAGE_INTEGRITY_MISSING",
        `integrity.files[${i}] must be an object`,
      );
      const fe = f as Record<string, unknown>;
      const fpath = requireString(fe.path, `integrity.files[${i}].path`, "PACKAGE_MANIFEST_INVALID");
      let sha256: string | undefined;
      if (typeof fe.sha256 === "string") {
        sha256 = fe.sha256;
      } else if (typeof fe.hash === "string") {
        sha256 = fe.hash;
      }
      assertCondition(
        typeof sha256 === "string" && SHA256_RE.test(sha256),
        "PACKAGE_MANIFEST_INVALID",
        `integrity.files[${i}].sha256 is required and must be a valid sha256`,
        { path: fpath },
      );
      const fbytes = requireNumber(fe.bytes, `integrity.files[${i}].bytes`, "PACKAGE_MANIFEST_INVALID");
      const fmediaType = requireString(fe.mediaType, `integrity.files[${i}].mediaType`, "PACKAGE_MANIFEST_INVALID");
      const frole = requireString(fe.role, `integrity.files[${i}].role`, "PACKAGE_MANIFEST_INVALID");
      files.push({
        path: fpath,
        sha256,
        bytes: fbytes,
        mediaType: fmediaType,
        role: frole,
        actionKey: typeof fe.actionKey === "string" ? fe.actionKey : undefined,
        frameId: typeof fe.frameId === "string" ? fe.frameId : undefined,
      });
    }

    const data: NormalizedManifestData = {
      schemaVersion: 2,
      manifestFormat,
      petId,
      releaseId,
      version,
      displayName,
      description,
      defaultActionKey,
      preview,
      canvas: { width: canvasWidth, height: canvasHeight, coordinateSystem: "top-left" },
      actionEntries,
      compatibility: { minRuntimeVersion, maxRuntimeVersion, renderMode: "sprite" },
      binding: {
        policy: binding.policy,
        sourceCharacterId: typeof binding.sourceCharacterId === "string" ? binding.sourceCharacterId : undefined,
      },
      integrity: {
        algorithm,
        manifestHash,
        contentRootHash,
        fileCount,
        totalBytes,
        files,
      },
    };

    return { data, warnings: [] };
  }

  readAction(raw: unknown, actionKey: string, configPath: string): ActionReadResult {
    assertCondition(
      raw !== null && typeof raw === "object",
      "ACTION_CONFIG_INVALID",
      `action config is missing or not an object (action: ${actionKey})`,
      { actionKey },
    );
    const a = raw as Record<string, unknown>;

    const sv = a.schemaVersion;
    assertCondition(
      typeof sv === "number" && sv === 2,
      "PACKAGE_ACTION_CONFIG_SCHEMA_UNSUPPORTED",
      `action.schemaVersion must be 2 (action: ${actionKey})`,
      { actionKey, expected: "2", actual: String(sv) },
    );

    const key = requireString(a.actionKey, "actionKey", "PACKAGE_ACTION_KEY_MISMATCH");
    assertCondition(
      key === actionKey,
      "PACKAGE_ACTION_KEY_MISMATCH",
      `actionKey mismatch: manifest says ${actionKey}, config says ${key}`,
      { actionKey, expected: actionKey, actual: key },
    );

    const displayName = requireString(a.displayName, "displayName", "PACKAGE_MANIFEST_INVALID");
    const version = requireNumber(a.version, "version", "PACKAGE_MANIFEST_INVALID");
    assertCondition(version >= 1, "PACKAGE_MANIFEST_INVALID", `version must be >= 1 (action: ${actionKey})`);
    const playbackMode = normalizePlaybackModeValue(a.playbackMode, actionKey);
    const fps = requireNumber(a.fps, "fps", "PACKAGE_MANIFEST_INVALID");
    assertCondition(fps >= 1 && fps <= 120, "PACKAGE_MANIFEST_INVALID", `fps must be between 1 and 120 (action: ${actionKey})`);
    const interruptible = requireBoolean(a.interruptible, "interruptible", "PACKAGE_MANIFEST_INVALID");
    const priority = requireNumber(a.priority, "priority", "PACKAGE_MANIFEST_INVALID");
    assertCondition(priority >= 0 && priority <= 100, "PACKAGE_MANIFEST_INVALID", `priority must be between 0 and 100 (action: ${actionKey})`);
    const cooldownMs = requireNumber(a.cooldownMs, "cooldownMs", "PACKAGE_MANIFEST_INVALID");
    assertCondition(cooldownMs >= 0, "PACKAGE_MANIFEST_INVALID", `cooldownMs must be >= 0 (action: ${actionKey})`);
    const minimumPlayMs = requireNumber(a.minimumPlayMs, "minimumPlayMs", "PACKAGE_MANIFEST_INVALID");
    assertCondition(minimumPlayMs >= 0, "PACKAGE_MANIFEST_INVALID", `minimumPlayMs must be >= 0 (action: ${actionKey})`);

    let maximumPlayMs: number;
    if (a.maximumPlayMs === null) {
      maximumPlayMs = 0;
    } else {
      maximumPlayMs = requireNumber(a.maximumPlayMs, "maximumPlayMs", "PACKAGE_MANIFEST_INVALID");
      assertCondition(maximumPlayMs >= 0, "PACKAGE_MANIFEST_INVALID", `maximumPlayMs must be >= 0 (action: ${actionKey})`);
    }

    let mutexGroup: string;
    if (a.mutexGroup === null || a.mutexGroup === undefined) {
      mutexGroup = "";
    } else {
      assertCondition(typeof a.mutexGroup === "string", "PACKAGE_MANIFEST_INVALID", `mutexGroup must be a string or null (action: ${actionKey})`);
      mutexGroup = a.mutexGroup as string;
    }

    const supportsDefaultIdle = requireBoolean(a.supportsDefaultIdle, "supportsDefaultIdle", "PACKAGE_MANIFEST_INVALID");
    const isStableStateCandidate = requireBoolean(a.isStableStateCandidate, "isStableStateCandidate", "PACKAGE_MANIFEST_INVALID");
    const isTransitionOnly = requireBoolean(a.isTransitionOnly, "isTransitionOnly", "PACKAGE_MANIFEST_INVALID");
    const returnTo = normalizeReturnToRule(a.returnTo, actionKey);
    const anchor = normalizeAnchor(a.anchor, actionKey);
    const frames = normalizeFramesStrict(a.frames, actionKey);

    const action: RuntimeAction = {
      actionKey: key,
      displayName,
      fps,
      playbackMode,
      interruptible,
      priority,
      cooldownMs,
      minimumPlayMs,
      maximumPlayMs,
      mutexGroup,
      returnTo,
      anchor,
      frames,
      configPath,
      version,
      supportsDefaultIdle,
      isStableStateCandidate,
      isTransitionOnly,
    };

    return { action, warnings: [] };
  }
}

export class Schema1PackageReader implements PackageReader {
  readManifest(raw: unknown): ManifestReadResult {
    const warnings: LegacyWarning[] = [];
    const m = (raw ?? {}) as Record<string, unknown>;

    const schemaVersion = typeof m.schemaVersion === "number" ? m.schemaVersion : 1;
    if (typeof m.schemaVersion !== "number") {
      warnings.push({
        code: "LEGACY_SCHEMA_VERSION_MISSING",
        message: "schemaVersion missing, defaulting to 1",
      });
    }

    let petId = "";
    if (typeof m.petId === "string" && m.petId) {
      petId = m.petId;
    } else if (typeof m.packageId === "string" && m.packageId) {
      petId = m.packageId;
      warnings.push({
        code: "LEGACY_PET_ID_FALLBACK",
        message: "petId missing, falling back to packageId",
      });
    }

    const releaseId = typeof m.releaseId === "string" ? m.releaseId : "";
    const displayName = typeof m.name === "string" ? m.name : "";
    const description = typeof m.description === "string" ? m.description : "";
    const defaultActionKey = typeof m.defaultAction === "string" ? m.defaultAction : "";
    const preview = typeof m.preview === "string" ? m.preview : null;

    let canvasWidth = 0;
    let canvasHeight = 0;
    if (m.canvas && typeof m.canvas === "object") {
      const cv = m.canvas as { width?: number; height?: number };
      canvasWidth = typeof cv.width === "number" ? cv.width : 0;
      canvasHeight = typeof cv.height === "number" ? cv.height : 0;
    }

    const actionEntries: ManifestActionEntry[] = [];
    if (Array.isArray(m.actions)) {
      for (const entry of m.actions) {
        if (!entry || typeof entry !== "object") continue;
        const ae = entry as Record<string, unknown>;
        const key = typeof ae.key === "string" ? ae.key : "";
        if (!key) continue;
        const name = typeof ae.name === "string" ? ae.name : key;
        const config = typeof ae.config === "string" ? ae.config : `actions/${key}/action.json`;
        const playbackMode = normalizePlaybackModeValue(
          ae.playbackMode ?? ae.loopType,
          key,
        );
        const fps = typeof ae.fps === "number" ? ae.fps : 0;
        const frameCount = typeof ae.frameCount === "number" ? ae.frameCount : 0;
        actionEntries.push({
          key,
          name,
          config,
          playbackMode,
          fps,
          frameCount,
          supportsDefaultIdle: typeof ae.supportsDefaultIdle === "boolean" ? ae.supportsDefaultIdle : true,
          isStableStateCandidate: typeof ae.isStableStateCandidate === "boolean" ? ae.isStableStateCandidate : playbackMode === "loop",
          isTransitionOnly: typeof ae.isTransitionOnly === "boolean" ? ae.isTransitionOnly : false,
        });
      }
    }

    let minRuntimeVersion = "0.0.0";
    let maxRuntimeVersion: string | null = null;
    if (m.compatibility && typeof m.compatibility === "object") {
      const compat = m.compatibility as Record<string, unknown>;
      if (typeof compat.minimumRuntimeVersion === "string") {
        minRuntimeVersion = compat.minimumRuntimeVersion;
      } else if (typeof compat.minRuntimeVersion === "string") {
        minRuntimeVersion = compat.minRuntimeVersion;
        warnings.push({
          code: "LEGACY_COMPAT_FIELD",
          message: "compatibility.minimumRuntimeVersion missing, using minRuntimeVersion",
        });
      }
      if (typeof compat.maxRuntimeVersion === "string") {
        maxRuntimeVersion = compat.maxRuntimeVersion;
      }
    }

    const binding = { policy: "legacy_inferred" };

    let integrity: NormalizedIntegrity = {
      algorithm: INTEGRITY_ALGORITHM_V1_LEGACY,
      manifestHash: "",
      contentRootHash: "",
      fileCount: 0,
      totalBytes: 0,
      files: [],
    };
    if (m.integrity && typeof m.integrity === "object") {
      const ig = m.integrity as Record<string, unknown>;
      const contentRootHash = typeof ig.contentRootHash === "string" ? ig.contentRootHash : "";
      const manifestHash = typeof ig.manifestHash === "string" ? ig.manifestHash : "";
      const algorithm = typeof ig.algorithm === "string" ? ig.algorithm : INTEGRITY_ALGORITHM_V1_LEGACY;
      const fileCount = typeof ig.fileCount === "number" ? ig.fileCount : 0;
      const totalBytes = typeof ig.totalBytes === "number" ? ig.totalBytes : 0;
      const files: RuntimeIntegrityFile[] = [];
      if (Array.isArray(ig.files)) {
        for (const f of ig.files) {
          if (!f || typeof f !== "object") continue;
          const fe = f as Record<string, unknown>;
          const fpath = typeof fe.path === "string" ? fe.path : "";
          if (!fpath) continue;
          const sha256 =
            typeof fe.sha256 === "string" ? fe.sha256 : typeof fe.hash === "string" ? fe.hash : "";
          files.push({
            path: fpath,
            sha256,
            bytes: typeof fe.bytes === "number" ? fe.bytes : 0,
            mediaType: typeof fe.mediaType === "string" ? fe.mediaType : "",
            role: typeof fe.role === "string" ? fe.role : "",
            actionKey: typeof fe.actionKey === "string" ? fe.actionKey : undefined,
            frameId: typeof fe.frameId === "string" ? fe.frameId : undefined,
          });
        }
      }
      integrity = { algorithm, manifestHash, contentRootHash, fileCount, totalBytes, files };
    }

    const data: NormalizedManifestData = {
      schemaVersion,
      manifestFormat: MANIFEST_FORMAT_CANONICAL,
      petId,
      releaseId,
      version: "0.0.0",
      displayName,
      description,
      defaultActionKey,
      preview,
      canvas: { width: canvasWidth, height: canvasHeight, coordinateSystem: "top-left" },
      actionEntries,
      compatibility: { minRuntimeVersion, maxRuntimeVersion, renderMode: "sprite" },
      binding,
      integrity,
    };

    return { data, warnings };
  }

  readAction(raw: unknown, actionKey: string, configPath: string): ActionReadResult {
    const warnings: LegacyWarning[] = [];
    const a = (raw ?? {}) as Record<string, unknown>;

    const key = typeof a.actionKey === "string" ? a.actionKey : typeof a.key === "string" ? a.key : actionKey;
    const displayName =
      typeof a.displayName === "string" ? a.displayName :
      typeof a.actionName === "string" ? a.actionName :
      typeof a.name === "string" ? a.name : key;

    if (a.loopType !== undefined && a.playbackMode === undefined) {
      warnings.push({
        code: "LEGACY_LOOP_TYPE",
        message: `loopType used instead of playbackMode (action: ${key})`,
        actionKey: key,
      });
    }

    const playbackMode = normalizePlaybackModeValue(a.playbackMode ?? a.loopType, key);
    const fps = typeof a.fps === "number" ? a.fps : typeof a.defaultFps === "number" ? a.defaultFps : 0;
    const version = typeof a.version === "number" ? a.version : 1;
    const interruptible = typeof a.interruptible === "boolean" ? a.interruptible : true;
    const priority = typeof a.priority === "number" ? a.priority : 50;
    const cooldownMs = typeof a.cooldownMs === "number" ? a.cooldownMs : 0;
    const minimumPlayMs = typeof a.minimumPlayMs === "number" ? a.minimumPlayMs : 0;
    const maximumPlayMs = typeof a.maximumPlayMs === "number" ? a.maximumPlayMs : 0;
    const mutexGroup = typeof a.mutexGroup === "string" ? a.mutexGroup : "";
    const supportsDefaultIdle = typeof a.supportsDefaultIdle === "boolean" ? a.supportsDefaultIdle : true;
    const isStableStateCandidate = typeof a.isStableStateCandidate === "boolean" ? a.isStableStateCandidate : playbackMode === "loop";
    const isTransitionOnly = typeof a.isTransitionOnly === "boolean" ? a.isTransitionOnly : false;

    const returnTo = normalizeReturnToRuleLegacy(a.returnTo, a.returnAction, key);
    const anchor = normalizeAnchorLegacy(a.anchor, key);

    const frameDurationMs = computeFrameDurationMs(fps, a.frameDurationMs);
    const frames = normalizeFramesLegacy(a.frames, frameDurationMs, key);

    const action: RuntimeAction = {
      actionKey: key,
      displayName,
      fps,
      playbackMode,
      interruptible,
      priority,
      cooldownMs,
      minimumPlayMs,
      maximumPlayMs,
      mutexGroup,
      returnTo,
      anchor,
      frames,
      configPath,
      version,
      supportsDefaultIdle,
      isStableStateCandidate,
      isTransitionOnly,
    };

    return { action, warnings };
  }
}

function normalizeReturnToRuleLegacy(
  returnTo: unknown,
  returnAction: unknown,
  actionKey: string,
): ReturnToRule {
  if (returnTo && typeof returnTo === "object") {
    const rt = returnTo as { type?: string; actionKey?: string };
    const type = rt.type ?? "default";
    switch (type) {
      case "action":
        if (rt.actionKey && typeof rt.actionKey === "string") {
          return { type: "action", actionKey: rt.actionKey };
        }
        return { type: "default" };
      case "default":
        return { type: "default" };
      case "previous":
        return { type: "previous" };
      case "current_activity":
        return { type: "current_activity" };
      case "none":
        return { type: "none" };
      default:
        return { type: "default" };
    }
  }
  if (typeof returnAction === "string" && returnAction.trim()) {
    return { type: "action", actionKey: returnAction };
  }
  return { type: "default" };
}

function normalizeAnchorLegacy(
  anchor: unknown,
  actionKey: string,
): RuntimeAnchor {
  if (!anchor || typeof anchor !== "object") {
    return { x: 0.5, y: 1.0, coordinateSpace: "normalized_canvas" };
  }
  const a = anchor as { x?: number; y?: number; coordinateSpace?: string };
  const x = typeof a.x === "number" ? a.x : 0.5;
  const y = typeof a.y === "number" ? a.y : 1.0;
  return { x, y, coordinateSpace: "normalized_canvas" };
}

function computeFrameDurationMs(
  fps: number,
  frameDurationMs: unknown,
): number {
  if (
    typeof frameDurationMs === "number" &&
    Number.isFinite(frameDurationMs) &&
    frameDurationMs > 0
  ) {
    return frameDurationMs;
  }
  if (fps > 0) {
    return 1000 / fps;
  }
  return 100;
}

function normalizeFramesLegacy(
  frames: unknown,
  defaultDurationMs: number,
  actionKey: string,
): RuntimeFrame[] {
  if (!Array.isArray(frames)) return [];
  const result: RuntimeFrame[] = [];
  for (let i = 0; i < frames.length; i++) {
    const item = frames[i];
    if (typeof item === "string") {
      if (item) {
        result.push({
          frameId: `${actionKey}_frame_${i}`,
          index: i,
          file: item,
          durationMs: defaultDurationMs,
          assetId: `${actionKey}_asset_${i}`,
          contentHash: "",
        });
      }
      continue;
    }
    if (item && typeof item === "object") {
      const f = item as {
        frameId?: string;
        index?: number;
        file?: string;
        durationMs?: number;
        assetId?: string;
        contentHash?: string;
      };
      if (typeof f.file === "string" && f.file) {
        result.push({
          frameId: f.frameId ?? `${actionKey}_frame_${i}`,
          index: f.index ?? i,
          file: f.file,
          durationMs: f.durationMs ?? defaultDurationMs,
          assetId: f.assetId ?? `${actionKey}_asset_${i}`,
          contentHash: f.contentHash ?? "",
        });
      }
    }
  }
  return result;
}

export class RuntimePackageNormalizer {
  private schema1Reader = new Schema1PackageReader();
  private schema2Reader = new Schema2PackageReader();

  normalize(
    manifestRaw: unknown,
    actionRaws: Map<string, unknown>,
    packageRoot: string,
    runtimeVersion: string,
  ): RuntimePetPackage {
    assertCondition(
      manifestRaw !== null && typeof manifestRaw === "object",
      "PACKAGE_SCHEMA_MISSING",
      "manifest is missing or not an object",
    );
    const m = manifestRaw as { schemaVersion?: number };
    const schemaVersion = m.schemaVersion;
    assertCondition(
      typeof schemaVersion === "number",
      "PACKAGE_SCHEMA_MISSING",
      "schemaVersion is required",
    );

    let reader: PackageReader;
    switch (schemaVersion) {
      case 1:
        reader = this.schema1Reader;
        break;
      case 2:
        reader = this.schema2Reader;
        break;
      default:
        throw createPackageError(
          "PACKAGE_SCHEMA_UNSUPPORTED",
          `unsupported schemaVersion: ${schemaVersion}`,
          { expected: "1 or 2", actual: String(schemaVersion) },
        );
    }

    const manifestResult = reader.readManifest(manifestRaw);
    const manifest = manifestResult.data;

    const actions = new Map<string, RuntimeAction>();
    for (const entry of manifest.actionEntries) {
      const actionRaw = actionRaws.get(entry.key);
      if (actionRaw === undefined) {
        throw createPackageError(
          "ACTION_CONFIG_MISSING",
          `action config not provided for key: ${entry.key}`,
          { actionKey: entry.key },
        );
      }
      const actionResult = reader.readAction(actionRaw, entry.key, entry.config);
      actions.set(entry.key, actionResult.action);
    }

    if (!manifest.compatibility || !manifest.compatibility.minRuntimeVersion) {
      throw createPackageError(
        "PACKAGE_MANIFEST_INVALID",
        "compatibility.minRuntimeVersion is required",
      );
    }
    if (!compareVersions(manifest.compatibility.minRuntimeVersion, runtimeVersion)) {
      throw createPackageError(
        "RUNTIME_VERSION_UNSUPPORTED",
        `runtime version ${runtimeVersion} is below required ${manifest.compatibility.minRuntimeVersion}`,
        { expected: manifest.compatibility.minRuntimeVersion, actual: runtimeVersion },
      );
    }
    if (manifest.compatibility.maxRuntimeVersion) {
      if (!compareVersions(runtimeVersion, manifest.compatibility.maxRuntimeVersion)) {
        throw createPackageError(
          "RUNTIME_VERSION_UNSUPPORTED",
          `runtime version ${runtimeVersion} is above max ${manifest.compatibility.maxRuntimeVersion}`,
          { expected: manifest.compatibility.maxRuntimeVersion, actual: runtimeVersion },
        );
      }
    }

    return {
      schemaVersion: 2,
      petId: manifest.petId,
      releaseId: manifest.releaseId,
      packageRoot,
      displayName: manifest.displayName,
      defaultActionKey: manifest.defaultActionKey,
      preview: manifest.preview,
      canvas: manifest.canvas,
      compatibility: {
        minRuntimeVersion: manifest.compatibility.minRuntimeVersion,
        maxRuntimeVersion: manifest.compatibility.maxRuntimeVersion,
        renderMode: "sprite",
      },
      actions,
      integrity: {
        algorithm: manifest.integrity.algorithm,
        manifestHash: manifest.integrity.manifestHash,
        contentRootHash: manifest.integrity.contentRootHash,
        fileCount: manifest.integrity.fileCount,
        totalBytes: manifest.integrity.totalBytes,
        files: manifest.integrity.files,
      },
      sourceSchemaVersion: (schemaVersion === 2 ? 2 : 1) as 1 | 2,
    };
  }

  getReader(schemaVersion: number): PackageReader {
    switch (schemaVersion) {
      case 1:
        return this.schema1Reader;
      case 2:
        return this.schema2Reader;
      default:
        throw createPackageError(
          "PACKAGE_SCHEMA_UNSUPPORTED",
          `unsupported schemaVersion: ${schemaVersion}`,
          { actual: String(schemaVersion) },
        );
    }
  }
}

export type { PackageValidationError };
