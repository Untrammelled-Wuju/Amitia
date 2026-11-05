import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, existsSync, realpathSync } from "node:fs";
import { dirname, join } from "node:path";
import { tmpdir } from "node:os";
import { ResourceLoader } from "../resource-loader";
import type { LoadInstallationRequest } from "../resource-loader";
import {
  DEFAULT_ACTION_NOT_FOUND_ERROR,
  MANIFEST_PARSE_FAILED_ERROR,
  MANIFEST_READ_FAILED_ERROR,
  UNSUPPORTED_SCHEMA_VERSION_ERROR,
  ACTION_JSON_READ_FAILED,
  ACTION_JSON_PARSE_FAILED,
} from "../resource-loader";
import {
  canonicalManifestJSON,
  computeTreeHash,
  sha256Hex,
} from "../package-integrity-verifier";
import { MANIFEST_PSEUDO_ENTRY_PATH } from "../../../shared/package-schema";

interface ActionConfig {
  key: string;
  name?: string;
  version?: number;
  loopType?: "loop" | "once" | "hold";
  fps?: number;
  frameDurationMs?: number;
  frameCount?: number;
  frames?: Array<string | { file: string }>;
  interruptible?: boolean;
  returnAction?: string;
  anchor?: { x: number; y: number };
  broken?: "read" | "parse" | "missing-frame";
}

interface ManifestConfig {
  petId?: string;
  schemaVersion?: number;
  name?: string;
  canvas?: { width: number; height: number };
  defaultAction?: string;
  preview?: string;
  actions?: Array<{ key: string; name?: string; loopType?: string }>;
}

function buildActionJson(action: ActionConfig, frameRelPaths: string[]): string {
  if (action.broken === "parse") {
    return "{ not valid json";
  }
  const payload: Record<string, unknown> = {
    schemaVersion: 1,
    actionKey: action.key,
    displayName: action.name ?? action.key,
    version: action.version ?? 1,
    playbackMode: action.loopType ?? "loop",
    fps: action.fps ?? 8,
    interruptible: action.interruptible ?? true,
    priority: 50,
    cooldownMs: 0,
    minimumPlayMs: 0,
    maximumPlayMs: null,
    mutexGroup: null,
    supportsDefaultIdle: true,
    isStableStateCandidate: true,
    isTransitionOnly: false,
    returnTo: action.returnAction
      ? { type: "action", actionKey: action.returnAction }
      : { type: "default" },
    anchor: action.anchor
      ? { ...action.anchor, coordinateSpace: "normalized_canvas" }
      : { x: 0.5, y: 1, coordinateSpace: "normalized_canvas" },
    frames: frameRelPaths.map((file, index) => ({
      file,
      index,
      durationMs: action.frameDurationMs ?? Math.round(1000 / (action.fps ?? 8)),
      contentHash: sha256Hex("png-bytes"),
      frameId: `${action.key}_${index}`,
      assetId: `${action.key}_asset_${index}`,
    })),
  };
  return JSON.stringify(payload);
}

function setupInstallation(
  root: string,
  manifest: ManifestConfig,
  actionConfigs: ActionConfig[],
): void {
  mkdirSync(join(root, "actions"), { recursive: true });
  const integrityFiles: Array<{
    path: string;
    sha256: string;
    bytes: number;
    mediaType: string;
    role: string;
    actionKey?: string;
    frameId?: string;
  }> = [];
  for (const action of actionConfigs) {
    const actionDir = join(root, "actions", action.key);
    mkdirSync(actionDir, { recursive: true });
    const rawFrames = action.frames ?? [];
    const frameFileNames: string[] = [];
    for (const frame of rawFrames) {
      const fileName = typeof frame === "string" ? frame : frame.file;
      frameFileNames.push(fileName);
    }
    if (action.broken !== "read") {
      const configText = buildActionJson(action, frameFileNames);
      const configPath = `actions/${action.key}/action.json`;
      writeFileSync(join(actionDir, "action.json"), configText);
      integrityFiles.push({
        path: configPath,
        sha256: sha256Hex(configText),
        bytes: Buffer.byteLength(configText),
        mediaType: "application/json",
        role: "action_config",
        actionKey: action.key,
      });
    }
    if (action.broken !== "missing-frame" && action.broken !== "read") {
      rawFrames.forEach((frame, index) => {
        const fileName = typeof frame === "string" ? frame : frame.file;
        if (fileName) {
          const frameText = "png-bytes";
          if (!existsSync(join(actionDir, fileName))) {
            writeFileSync(join(actionDir, fileName), frameText);
          }
          integrityFiles.push({
            path: `actions/${action.key}/${fileName}`,
            sha256: sha256Hex(frameText),
            bytes: Buffer.byteLength(frameText),
            mediaType: "image/png",
            role: "frame_image",
            actionKey: action.key,
            frameId: `${action.key}_${index}`,
          });
        }
      });
    }
  }
  if (manifest.preview) {
    mkdirSync(dirname(join(root, manifest.preview)), { recursive: true });
    const previewText = "preview-bytes";
    writeFileSync(join(root, manifest.preview), previewText);
    integrityFiles.push({
      path: manifest.preview,
      sha256: sha256Hex(previewText),
      bytes: Buffer.byteLength(previewText),
      mediaType: "image/png",
      role: "preview_image",
    });
  }
  const actions = actionConfigs.map((action) => ({
    key: action.key,
    name: action.name ?? action.key,
    config: `actions/${action.key}/action.json`,
    revisionId: "",
    qualityEvaluationId: "",
    playbackMode: action.loopType ?? "loop",
    fps: action.fps ?? 8,
    frameCount: Math.max(1, action.frameCount ?? action.frames?.length ?? 0),
    supportsDefaultIdle: true,
    isStableStateCandidate: true,
    isTransitionOnly: false,
  }));
  const canonical: Record<string, unknown> = {
    schemaVersion: manifest.schemaVersion ?? 1,
    manifestFormat: "amitia-desktop-pet",
    petId: manifest.petId ?? "pet-test",
    releaseId: "release-test",
    version: "1.0.0",
    name: manifest.name ?? "Pet",
    description: "",
    author: { name: "Test", id: "" },
    license: { spdx: "AGPL-3.0-only" },
    compatibility: { minRuntimeVersion: "1.0.0", renderMode: "sprite" },
    binding: { policy: "unbound" },
    canvas: {
      width: manifest.canvas?.width ?? 256,
      height: manifest.canvas?.height ?? 256,
      coordinateSystem: "top-left",
    },
    defaultAction: manifest.defaultAction ?? actions[0]?.key ?? "",
    ...(manifest.preview === undefined ? {} : { preview: manifest.preview }),
    actions,
    capabilities: { transparentBackground: true, frameSequence: true, perFrameDuration: true, audio: false },
    integrity: {
      algorithm: "amitia-package-sha256-v1",
      manifestHash: "",
      contentRootHash: "",
      fileCount: integrityFiles.length,
      totalBytes: integrityFiles.reduce((sum, file) => sum + file.bytes, 0),
      files: integrityFiles,
    },
    provenance: { sourceType: "generated", builder: "test" },
  };
  const placeholderRaw = JSON.stringify(canonical);
  const manifestHash = sha256Hex(canonicalManifestJSON(placeholderRaw));
  const integrity = canonical.integrity as Record<string, unknown>;
  integrity.manifestHash = manifestHash;
  const contentRootHash = computeTreeHash([
    ...integrityFiles,
    {
      path: MANIFEST_PSEUDO_ENTRY_PATH,
      sha256: manifestHash,
      bytes: Buffer.byteLength(canonicalManifestJSON(placeholderRaw)),
    },
  ]);
  integrity.contentRootHash = contentRootHash;
  writeFileSync(join(root, "manifest.json"), JSON.stringify(canonical));
}

function buildRequest(
  root: string,
  overrides: Partial<LoadInstallationRequest> = {},
): LoadInstallationRequest {
  return {
    installationId: "inst-test",
    petId: "",
    releaseId: "release-test",
    installPath: root,
    manifestPath: join(root, "manifest.json"),
    expectedContentRootHash: "",
    ...overrides,
  };
}

describe("ResourceLoader", () => {
  let root: string;
  let loader: ResourceLoader;

  beforeEach(() => {
    root = realpathSync(mkdtempSync(join(tmpdir(), "amitia-pet-loader-")));
    loader = new ResourceLoader();
  });

  afterEach(() => {
    rmSync(root, { recursive: true, force: true });
  });

  it("manifest 解析成功时返回完整 LoadedInstallation", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-1",
        schemaVersion: 1,
        name: "Pet",
        canvas: { width: 256, height: 256 },
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "wave", loopType: "once" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png", "frame1.png"],
          frameCount: 2,
          frameDurationMs: 100,
        },
        {
          key: "wave",
          frames: ["wave0.png", "wave1.png", "wave2.png"],
          frameCount: 3,
          frameDurationMs: 80,
          loopType: "once",
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-1" }),
    );

    expect(loaded.installationId).toBe("inst-test");
    expect(loaded.manifest.schemaVersion).toBe(1);
    expect(loaded.manifest.defaultAction).toBe("idle");
    expect(loaded.actions.size).toBe(2);
    expect(loaded.defaultAction?.key).toBe("idle");
    expect(loaded.defaultAction?.available).toBe(true);
    expect(loaded.actions.get("wave")?.available).toBe(true);
    expect(loaded.actions.get("idle")?.frames.length).toBe(2);
  });

  it("schemaVersion 不受支持时抛出 UNSUPPORTED_SCHEMA_VERSION_ERROR", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-x",
        schemaVersion: 99,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root)),
    ).rejects.toThrow(UNSUPPORTED_SCHEMA_VERSION_ERROR);
  });

  it("manifest 读取失败时抛出 MANIFEST_READ_FAILED_ERROR", async () => {
    await expect(
      loader.loadInstallation(
        buildRequest(root, { manifestPath: join(root, "missing-manifest.json") }),
      ),
    ).rejects.toThrow(MANIFEST_READ_FAILED_ERROR);
  });

  it("manifest 解析失败时抛出 MANIFEST_PARSE_FAILED_ERROR", async () => {
    writeFileSync(join(root, "manifest.json"), "{ invalid");
    await expect(
      loader.loadInstallation(buildRequest(root)),
    ).rejects.toThrow(MANIFEST_PARSE_FAILED_ERROR);
  });

  it("petId 不匹配时抛出 PET_IDENTITY_CONFLICT", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-1",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "different-pet" })),
    ).rejects.toThrow("PET_IDENTITY_CONFLICT");
  });

  it("action.json 读取失败时整个 Package 加载失败", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-3",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "ghost", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "ghost",
          broken: "read",
          frames: ["frame0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "pet-3" })),
    ).rejects.toThrow(ACTION_JSON_READ_FAILED);
  });

  it("action.json 解析失败时整个 Package 加载失败", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-4",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "bad", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "bad",
          broken: "parse",
          frames: ["frame0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "pet-4" })),
    ).rejects.toThrow(ACTION_JSON_PARSE_FAILED);
  });

  it("默认动作的 action.json 读取失败时抛出 ACTION_JSON_READ_FAILED", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-5",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "wave", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          broken: "read",
          frames: ["frame0.png"],
        },
        {
          key: "wave",
          frames: ["wave0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "pet-5" })),
    ).rejects.toThrow(ACTION_JSON_READ_FAILED);
  });

  it("默认动作不存在时抛出 DEFAULT_ACTION_NOT_FOUND_ERROR", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-6",
        schemaVersion: 1,
        defaultAction: "ghost",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "pet-6" })),
    ).rejects.toThrow(DEFAULT_ACTION_NOT_FOUND_ERROR);
  });

  it("defaultAction 为空字符串时拒绝无效 manifest", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-7",
        schemaVersion: 1,
        defaultAction: "",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "pet-7" })),
    ).rejects.toThrow(MANIFEST_PARSE_FAILED_ERROR);
  });

  it("仅按 manifest.actions 列表建立动作索引", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-8",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-8" }),
    );

    expect(loaded.actions.size).toBe(1);
    expect(loaded.actions.has("idle")).toBe(true);
  });

  it("帧序列校验：frames 数组为空时拒绝整个 Package", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-9",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "empty", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "empty",
          frames: [],
          frameCount: 0,
        },
      ],
    );

    await expect(
      loader.loadInstallation(buildRequest(root, { petId: "pet-9" })),
    ).rejects.toThrow("PACKAGE_MANIFEST_INVALID");
  });

  it("findFirstAvailableAction 按顺序返回首个可用动作", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-10",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [
          { key: "idle", loopType: "loop" },
          { key: "wave", loopType: "once" },
          { key: "sleep", loopType: "loop" },
        ],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "wave",
          frames: ["wave0.png"],
        },
        {
          key: "sleep",
          frames: ["sleep0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-10" }),
    );

    const found = loader.findFirstAvailableAction(loaded, ["wave", "sleep", "idle"]);
    expect(found?.key).toBe("wave");

    const found2 = loader.findFirstAvailableAction(loaded, ["nonexistent"]);
    expect(found2).toBeNull();

    expect(loader.findFirstAvailableAction(loaded, [])).toBeNull();
  });

  it("getAvailableActions 返回所有 available 为 true 的动作", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-11",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [
          { key: "idle", loopType: "loop" },
          { key: "wave", loopType: "once" },
        ],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "wave",
          frames: ["wave0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-11" }),
    );

    const available = loader.getAvailableActions(loaded);
    const keys = available.map((a) => a.key).sort();
    expect(keys).toEqual(["idle", "wave"]);
  });

  it("isDefaultActionAvailable 反映默认动作可用状态", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-12",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-12" }),
    );

    expect(loader.isDefaultActionAvailable(loaded)).toBe(true);
  });

  it("loadAction 返回已缓存的可用动作", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-13",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "wave", loopType: "once" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "wave",
          frames: ["wave0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-13" }),
    );

    const cached = await loader.loadAction("idle", loaded);
    expect(cached?.key).toBe("idle");
    expect(cached?.available).toBe(true);
  });

  it("loadAction 可重新加载动作", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-14",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }, { key: "wave", loopType: "once" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
        {
          key: "wave",
          frames: ["wave0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-14" }),
    );

    expect(loaded.actions.get("wave")?.available).toBe(true);

    const reloaded = await loader.loadAction("wave", loaded);
    expect(reloaded?.available).toBe(true);
    expect(reloaded?.key).toBe("wave");
    expect(loaded.actions.get("wave")?.available).toBe(true);
  });

  it("loadAction 对未知动作返回 null", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-15",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-15" }),
    );

    expect(await loader.loadAction("unknown", loaded)).toBeNull();
    expect(await loader.loadAction("", loaded)).toBeNull();
  });

  it("preview 路径根据 manifest.preview 解析", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-16",
        schemaVersion: 1,
        defaultAction: "idle",
        preview: "custom-preview.png",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-16" }),
    );

    expect(loaded.previewPath).toBe(join(root, "custom-preview.png"));
  });

  it("preview 为空时 previewPath 为 null（不猜测 preview.png）", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-17",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-17" }),
    );

    expect(loaded.previewPath).toBeNull();
  });

  it("anchor 数值在 manifest 中被保留", async () => {
    setupInstallation(
      root,
      {
        petId: "pet-18",
        schemaVersion: 1,
        defaultAction: "idle",
        actions: [{ key: "idle", loopType: "loop" }],
      },
      [
        {
          key: "idle",
          frames: ["frame0.png"],
          anchor: { x: 0.1, y: 0.2 },
        },
      ],
    );

    const loaded = await loader.loadInstallation(
      buildRequest(root, { petId: "pet-18" }),
    );

    const action = loaded.actions.get("idle");
    expect(action?.anchor).toEqual({ x: 0.1, y: 0.2, coordinateSpace: "normalized_canvas" });
  });
});
