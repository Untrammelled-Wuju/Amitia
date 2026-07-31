import { describe, it, expect } from "vitest";
import { join, dirname, normalize } from "node:path";
import { resolveActionFramePath, buildPetResourceUrl } from "../resource-resolver";
import { PET_PROTOCOL_SCHEME } from "../../../shared/animation-ipc";

function toPosix(p: string): string {
  return p.replace(/\\/g, "/");
}

describe("resolveActionFramePath", () => {
  it("正常相对路径解析", () => {
    const actionConfigPath = "actions/idle/action.json";
    const frameFile = "frame0.png";
    const result = resolveActionFramePath(actionConfigPath, frameFile, "/pkg");
    const expected = normalize(join(dirname(actionConfigPath), frameFile));
    expect(toPosix(result)).toBe(toPosix(expected));
    expect(toPosix(result)).toBe("actions/idle/frame0.png");
  });

  it("空文件名抛出 FRAME_FILE_EMPTY", () => {
    expect(() =>
      resolveActionFramePath("actions/idle/action.json", "", "/pkg"),
    ).toThrow("FRAME_FILE_EMPTY");
  });

  it("绝对路径抛出 FRAME_PATH_ABSOLUTE", () => {
    expect(() =>
      resolveActionFramePath("actions/idle/action.json", "/etc/passwd", "/pkg"),
    ).toThrow("FRAME_PATH_ABSOLUTE");
  });

  it("包含 .. 的路径抛出 FRAME_PATH_TRAVERSAL", () => {
    expect(() =>
      resolveActionFramePath("actions/idle/action.json", "../frame0.png", "/pkg"),
    ).toThrow("FRAME_PATH_TRAVERSAL");
  });

  it("包含多级 .. 的路径抛出 FRAME_PATH_TRAVERSAL", () => {
    expect(() =>
      resolveActionFramePath("actions/idle/action.json", "../../../etc/passwd", "/pkg"),
    ).toThrow("FRAME_PATH_TRAVERSAL");
  });

  it("路径超出包根目录时抛出 FRAME_PATH_OUTSIDE_PACKAGE", () => {
    expect(() =>
      resolveActionFramePath("../outside/action.json", "frame0.png", "/pkg"),
    ).toThrow("FRAME_PATH_OUTSIDE_PACKAGE");
  });

  it("子目录路径正确解析", () => {
    const actionConfigPath = "actions/idle/action.json";
    const frameFile = "sprites/f0.png";
    const result = resolveActionFramePath(actionConfigPath, frameFile, "/pkg");
    const expected = normalize(join(dirname(actionConfigPath), frameFile));
    expect(toPosix(result)).toBe(toPosix(expected));
    expect(toPosix(result)).toBe("actions/idle/sprites/f0.png");
  });
});

describe("buildPetResourceUrl", () => {
  it("正常生成 URL", () => {
    const url = buildPetResourceUrl("inst-123", "actions/idle/frame0.png");
    expect(url).toBe(
      `${PET_PROTOCOL_SCHEME}://installation/inst-123/actions/idle/frame0.png`,
    );
  });

  it("installationId 被编码", () => {
    const installationId = "inst/with/slash";
    const url = buildPetResourceUrl(installationId, "actions/idle/frame0.png");
    expect(url).toContain(encodeURIComponent(installationId));
    expect(url).toBe(
      `${PET_PROTOCOL_SCHEME}://installation/${encodeURIComponent(installationId)}/actions/idle/frame0.png`,
    );
  });

  it("relativePath 中的反斜杠被转换为正斜杠", () => {
    const url = buildPetResourceUrl("inst-123", "actions\\idle\\frame0.png");
    expect(url).toBe(
      `${PET_PROTOCOL_SCHEME}://installation/inst-123/actions/idle/frame0.png`,
    );
  });

  it("relativePath 开头的斜杠被移除", () => {
    const url = buildPetResourceUrl("inst-123", "/actions/idle/frame0.png");
    expect(url).toBe(
      `${PET_PROTOCOL_SCHEME}://installation/inst-123/actions/idle/frame0.png`,
    );
  });
});
