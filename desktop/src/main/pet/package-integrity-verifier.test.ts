import { describe, expect, it } from "vitest";
import { mkdtemp, mkdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  MANIFEST_PSEUDO_ENTRY_PATH,
  type NormalizedManifestData,
  type RuntimeAction,
} from "../../shared/package-schema";
import {
  canonicalJSON,
  canonicalManifestJSON,
  computeTreeHash,
  sha256Hex,
  PackageIntegrityVerifier,
} from "./package-integrity-verifier";

describe("PackageIntegrityVerifier Package v2 hash compatibility", () => {
  it("matches the Go tree-hash byte stream", () => {
    const hash = computeTreeHash([
      {
        path: "actions/idle/action.json",
        sha256: "a".repeat(64),
        bytes: 250,
      },
      {
        path: "preview.png",
        sha256: "b".repeat(64),
        bytes: 1234,
      },
      {
        path: "@manifest",
        sha256: "c".repeat(64),
        bytes: 987,
      },
    ]);

    expect(hash).toBe(
      "8906882d671da5000348bf9e8d8067f8cc29f1c854538f33cb4dbacd9b2a6158",
    );
  });

  it("matches Go encoding/json escaping used by canonical manifest hashing", () => {
    const canonical = canonicalJSON({
      z: "A&B <pet>\u2028line\u2029end",
      a: { y: 2, x: 1 },
    });

    expect(canonical).toBe(
      '[{"k":"a","v":[{"k":"x","v":1},{"k":"y","v":2}]},{"k":"z","v":"A\\u0026B \\u003cpet\\u003e\\u2028line\\u2029end"}]',
    );
    expect(sha256Hex(canonical)).toBe(
      "6215ff28de97d52b9e1502e22839994e1a3ead9e8a82dfd94048dca612031bc7",
    );
  });

  it("matches Go Manifest zero-value projection before canonical hashing", () => {
    const raw = JSON.stringify({
      schemaVersion: 2,
      name: "Hash Test",
      integrity: {
        algorithm: "amitia-package-sha256-v2",
        manifestHash: "1".repeat(64),
        contentRootHash: "2".repeat(64),
        fileCount: 0,
        totalBytes: 0,
        files: [],
      },
    });

    const actual = canonicalManifestJSON(raw);
    // Golden values produced by backend/internal/desktoppet/packageformat
    // using the strongly typed Go Manifest struct and CanonicalManifestData.
    expect(Buffer.byteLength(actual, "utf8")).toBe(1167);
    expect(sha256Hex(actual)).toBe(
      "6156d6d6c62cc8cce0898988ba00be883970d3842dcdb6f49ab340411142939f",
    );
  });
});

describe("PackageIntegrityVerifier filesystem verification", () => {
  it("accepts manifest.json as the only undeclared physical file and resolves frames relative to action config", async () => {
    const root = await mkdtemp(join(tmpdir(), "amitia-pet-integrity-"));
    try {
      const actionDir = join(root, "actions", "idle");
      const frameDir = join(actionDir, "frames");
      await mkdir(frameDir, { recursive: true });

      const actionBytes = Buffer.from('{"schemaVersion":2,"actionKey":"idle"}', "utf8");
      const frameBytes = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a]);
      const previewBytes = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x01]);
      await writeFile(join(actionDir, "action.json"), actionBytes);
      await writeFile(join(frameDir, "0.png"), frameBytes);
      await writeFile(join(root, "preview.png"), previewBytes);

      const files = [
        {
          path: "actions/idle/action.json",
          sha256: sha256Hex(actionBytes),
          bytes: actionBytes.length,
          mediaType: "application/json",
          role: "action-config",
          actionKey: "idle",
        },
        {
          path: "actions/idle/frames/0.png",
          sha256: sha256Hex(frameBytes),
          bytes: frameBytes.length,
          mediaType: "image/png",
          role: "frame",
          actionKey: "idle",
          frameId: "idle_0",
        },
        {
          path: "preview.png",
          sha256: sha256Hex(previewBytes),
          bytes: previewBytes.length,
          mediaType: "image/png",
          role: "preview",
        },
      ];
      const totalBytes = files.reduce((sum, file) => sum + file.bytes, 0);

      const rawManifest: Record<string, unknown> = {
        schemaVersion: 2,
        manifestFormat: "amitia-desktop-pet",
        petId: "integrity-test-pet",
        releaseId: "integrity-test-release",
        version: "1.0.0",
        name: "Integrity Test",
        description: "A&B <pet>",
        defaultAction: "idle",
        preview: "preview.png",
        canvas: { width: 64, height: 64, coordinateSystem: "top-left" },
        actions: [
          {
            key: "idle",
            name: "Idle",
            config: "actions/idle/action.json",
            playbackMode: "loop",
            fps: 12,
            frameCount: 1,
            supportsDefaultIdle: true,
            isStableStateCandidate: true,
            isTransitionOnly: false,
          },
        ],
        compatibility: { minRuntimeVersion: "2.0.0", renderMode: "sprite" },
        binding: { policy: "unbound" },
        integrity: {
          algorithm: "amitia-package-sha256-v2",
          manifestHash: "",
          contentRootHash: "",
          fileCount: files.length,
          totalBytes,
          files,
        },
      };

      const unsignedRaw = JSON.stringify(rawManifest);
      const canonicalManifest = canonicalManifestJSON(unsignedRaw);
      const manifestHash = sha256Hex(canonicalManifest);
      const contentRootHash = computeTreeHash([
        ...files.map((file) => ({
          path: file.path,
          sha256: file.sha256,
          bytes: file.bytes,
        })),
        {
          path: MANIFEST_PSEUDO_ENTRY_PATH,
          sha256: manifestHash,
          bytes: Buffer.byteLength(canonicalManifest, "utf8"),
        },
      ]);
      const integrity = rawManifest.integrity as Record<string, unknown>;
      integrity.manifestHash = manifestHash;
      integrity.contentRootHash = contentRootHash;
      const manifestRawText = JSON.stringify(rawManifest, null, 2);
      const manifestPath = join(root, "manifest.json");
      await writeFile(manifestPath, manifestRawText, "utf8");

      const manifest: NormalizedManifestData = {
        schemaVersion: 2,
        manifestFormat: "amitia-desktop-pet",
        petId: "integrity-test-pet",
        releaseId: "integrity-test-release",
        version: "1.0.0",
        displayName: "Integrity Test",
        description: "A&B <pet>",
        defaultActionKey: "idle",
        preview: "preview.png",
        canvas: { width: 64, height: 64, coordinateSystem: "top-left" },
        actionEntries: [
          {
            key: "idle",
            name: "Idle",
            config: "actions/idle/action.json",
            playbackMode: "loop",
            fps: 12,
            frameCount: 1,
            supportsDefaultIdle: true,
            isStableStateCandidate: true,
            isTransitionOnly: false,
          },
        ],
        compatibility: {
          minRuntimeVersion: "2.0.0",
          maxRuntimeVersion: null,
          renderMode: "sprite",
        },
        binding: { policy: "unbound" },
        integrity: {
          algorithm: "amitia-package-sha256-v2",
          manifestHash,
          contentRootHash,
          fileCount: files.length,
          totalBytes,
          files,
        },
      };

      const action: RuntimeAction = {
        actionKey: "idle",
        displayName: "Idle",
        fps: 12,
        playbackMode: "loop",
        interruptible: true,
        priority: 0,
        cooldownMs: 0,
        minimumPlayMs: 0,
        maximumPlayMs: 0,
        mutexGroup: "",
        returnTo: { type: "none" },
        anchor: { x: 0.5, y: 1, coordinateSpace: "normalized_canvas" },
        frames: [
          {
            frameId: "idle_0",
            index: 0,
            file: "frames/0.png",
            durationMs: 83,
            assetId: "idle_asset_0",
            contentHash: sha256Hex(frameBytes),
          },
        ],
        configPath: "actions/idle/action.json",
        version: 2,
        supportsDefaultIdle: true,
        isStableStateCandidate: true,
        isTransitionOnly: false,
      };

      const verifier = new PackageIntegrityVerifier();
      const params = {
        manifestRawText,
        manifest,
        installPath: root,
        manifestPath,
        actions: new Map([["idle", action]]),
      };
      const first = await verifier.verify(params);
      expect(first.valid).toBe(true);
      expect(first.errors.filter((item) => item.severity === "error")).toEqual([]);

      await writeFile(join(root, "tampered.txt"), "undeclared", "utf8");
      const second = await verifier.verify(params);
      expect(second.valid).toBe(false);
      expect(second.errors.some((item) => item.code === "PACKAGE_FILE_UNDECLARED")).toBe(true);
    } finally {
      await rm(root, { recursive: true, force: true });
    }
  });
});
