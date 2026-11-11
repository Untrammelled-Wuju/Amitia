import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { inflateSync } from "node:zlib";
import { describe, expect, it } from "vitest";

const resourcesDir = resolve(process.cwd(), "resources");

type PngImage = { width: number; height: number; colorType: number; alpha: Uint8Array };

function decodePng(name: string): PngImage {
  const buffer = readFileSync(resolve(resourcesDir, name));
  if (buffer.subarray(1, 4).toString("ascii") !== "PNG") {
    throw new Error(`${name} 不是 PNG 文件`);
  }
  let offset = 8;
  let width = 0;
  let height = 0;
  let bitDepth = 0;
  let colorType = 0;
  let interlace = 0;
  const chunks: Buffer[] = [];
  while (offset < buffer.length) {
    const length = buffer.readUInt32BE(offset);
    const type = buffer.subarray(offset + 4, offset + 8).toString("ascii");
    const data = buffer.subarray(offset + 8, offset + 8 + length);
    if (type === "IHDR") {
      width = data.readUInt32BE(0);
      height = data.readUInt32BE(4);
      bitDepth = data[8];
      colorType = data[9];
      interlace = data[12];
    } else if (type === "IDAT") {
      chunks.push(Buffer.from(data));
    }
    offset += 12 + length;
  }
  if (bitDepth !== 8 || colorType !== 6 || interlace !== 0) {
    throw new Error(
      `${name} 不是 8 位 RGBA 非隔行 PNG（bitDepth=${bitDepth} colorType=${colorType} interlace=${interlace}）`,
    );
  }
  const raw = inflateSync(Buffer.concat(chunks));
  const channels = 4;
  const stride = width * channels;
  const pixels = Buffer.alloc(width * height * channels);
  let previous = Buffer.alloc(stride);
  for (let y = 0; y < height; y += 1) {
    const filter = raw[y * (stride + 1)];
    const line = raw.subarray(y * (stride + 1) + 1, y * (stride + 1) + 1 + stride);
    const current = Buffer.alloc(stride);
    for (let x = 0; x < stride; x += 1) {
      const left = x >= channels ? current[x - channels] : 0;
      const up = previous[x];
      const upLeft = x >= channels ? previous[x - channels] : 0;
      let value = line[x];
      if (filter === 1) {
        value = (value + left) & 0xff;
      } else if (filter === 2) {
        value = (value + up) & 0xff;
      } else if (filter === 3) {
        value = (value + ((left + up) >> 1)) & 0xff;
      } else if (filter === 4) {
        const estimate = left + up - upLeft;
        const distanceLeft = Math.abs(estimate - left);
        const distanceUp = Math.abs(estimate - up);
        const distanceUpLeft = Math.abs(estimate - upLeft);
        const predictor =
          distanceLeft <= distanceUp && distanceLeft <= distanceUpLeft
            ? left
            : distanceUp <= distanceUpLeft
              ? up
              : upLeft;
        value = (value + predictor) & 0xff;
      } else if (filter !== 0) {
        throw new Error(`${name} 使用了未知的 PNG 过滤类型 ${filter}`);
      }
      current[x] = value;
    }
    current.copy(pixels, y * stride);
    previous = current;
  }
  const alpha = new Uint8Array(width * height);
  for (let index = 0; index < width * height; index += 1) {
    alpha[index] = pixels[index * channels + 3];
  }
  return { width, height, colorType, alpha };
}

function countAtLeast(alpha: Uint8Array, threshold: number): number {
  let total = 0;
  for (const value of alpha) {
    if (value >= threshold) total += 1;
  }
  return total;
}

function centerAlpha(image: PngImage): number {
  const x = Math.floor(image.width / 2);
  const y = Math.floor(image.height / 2);
  return image.alpha[y * image.width + x];
}

describe("macOS 菜单栏图标资源", () => {
  it("模板图保持菜单栏标准像素尺寸", () => {
    expect(decodePng("trayTemplate.png")).toMatchObject({ width: 16, height: 16 });
    expect(decodePng("trayTemplate@2x.png")).toMatchObject({ width: 32, height: 32 });
  });

  it("模板图使用透明背景与纯黑图形", () => {
    expect(decodePng("trayTemplate.png").colorType).toBe(6);
    expect(decodePng("trayTemplate@2x.png").colorType).toBe(6);
  });

  it("模板图线条具备菜单栏可辨认的不透明度", () => {
    const oneX = decodePng("trayTemplate.png");
    const twoX = decodePng("trayTemplate@2x.png");
    expect(Math.max(...oneX.alpha)).toBe(255);
    expect(Math.max(...twoX.alpha)).toBe(255);
    expect(countAtLeast(oneX.alpha, 192)).toBeGreaterThanOrEqual(24);
    expect(countAtLeast(twoX.alpha, 192)).toBeGreaterThanOrEqual(180);
  });

  it("模板图保留四周留白与中心镂空", () => {
    const oneX = decodePng("trayTemplate.png");
    const twoX = decodePng("trayTemplate@2x.png");
    expect(oneX.alpha[0]).toBe(0);
    expect(oneX.alpha[15]).toBe(0);
    expect(twoX.alpha[0]).toBe(0);
    expect(twoX.alpha[32 * 32 - 1]).toBe(0);
    expect(centerAlpha(oneX)).toBeLessThan(128);
    expect(centerAlpha(twoX)).toBeLessThan(128);
  });
});

describe("托盘菜单交互", () => {
  const source = readFileSync(resolve(process.cwd(), "src/main/tray.ts"), "utf8");

  it("左键显示窗口、右键弹出菜单", () => {
    expect(source).toContain('tray.on("click"');
    expect(source).toContain('tray.on("right-click"');
    expect(source).toContain("tray.popUpContextMenu(");
  });

  it("macOS 不绑定常驻菜单，菜单按平台绑定", () => {
    expect(source).toMatch(
      /process\.platform !== "darwin"[\s\S]{0,80}tray\.setContextMenu\(/,
    );
    expect(source.match(/tray\.setContextMenu\(/g)).toHaveLength(1);
  });
});

describe("品牌图加载", () => {
  const source = readFileSync(resolve(process.cwd(), "src/main/branding.ts"), "utf8");

  it("macOS 走模板图分支并标记为模板", () => {
    expect(source).toContain('process.platform === "darwin"');
    expect(source).toContain("trayTemplate.png");
    expect(source).toContain("setTemplateImage(true)");
  });

  it("非 macOS 托盘图限制在 16 像素逻辑尺寸", () => {
    expect(source).toContain("resize(trayPixelSize)");
    expect(source).toContain("width: 16, height: 16");
  });
});
