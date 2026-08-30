import { describe, it, expect } from "vitest";
import { resolveActionFramePath, buildPetResourceUrl } from "../resource-resolver";
import { PET_PROTOCOL_SCHEME } from "../../../shared/animation-ipc";

describe("resolveActionFramePath", () => {
  it("正常相对路径解析", () => {
    expect(
      resolveActionFramePath("actions/idle/action.json", "frame0.png", "/pkg"),
    ).toBe("actions/idle/frame0.png");
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

  it("包含真实父目录段的路径抛出 FRAME_PATH_TRAVERSAL", () => {
    expect(() =>
      resolveActionFramePath("actions/idle/action.json", "../frame0.png", "/pkg"),
    ).toThrow("FRAME_PATH_TRAVERSAL");
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
    expect(
      resolveActionFramePath("actions/idle/action.json", "sprites/f0.png", "/pkg"),
    ).toBe("actions/idle/sprites/f0.png");
  });

  it("普通文件名中的连续点不是 traversal", () => {
    expect(
      resolveActionFramePath("actions/idle/action.json", "frame..png", "/pkg"),
    ).toBe("actions/idle/frame..png");
    expect(
      resolveActionFramePath("actions/idle/action.json", "..frame.png", "/pkg"),
    ).toBe("actions/idle/..frame.png");
  });

  it("Package V2 拒绝反斜杠和其他非规范路径", () => {
    expect(() =>
      resolveActionFramePath("actions/idle/action.json", "sprites\\f0.png", "/pkg"),
    ).toThrow("FRAME_PATH_OUTSIDE_PACKAGE");
    expect(() =>
      resolveActionFramePath("actions//idle/action.json", "f0.png", "/pkg"),
    ).toThrow("FRAME_PATH_OUTSIDE_PACKAGE");
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

  it("逐段编码 Unicode、空格、# 和 %，避免 URL 语义截断", () => {
    const installationId = "install #1";
    const path = "actions/待机/frame #100%.png";
    expect(buildPetResourceUrl(installationId, path)).toBe(
      `${PET_PROTOCOL_SCHEME}://installation/${encodeURIComponent(installationId)}/actions/${encodeURIComponent("待机")}/${encodeURIComponent("frame #100%.png")}`,
    );
  });

  it("拒绝由调用端偷偷修正的非规范 package path", () => {
    expect(() =>
      buildPetResourceUrl("inst-123", "actions\\idle\\frame0.png"),
    ).toThrow();
    expect(() =>
      buildPetResourceUrl("inst-123", "/actions/idle/frame0.png"),
    ).toThrow();
  });
});
