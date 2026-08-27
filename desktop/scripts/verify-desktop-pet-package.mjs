import { existsSync, readFileSync } from "node:fs";
import { resolve, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const projectRoot = resolve(__dirname, "..");
const distMain = resolve(projectRoot, "dist/main");
const distPreload = resolve(projectRoot, "dist/preload");
const distRenderer = resolve(projectRoot, "dist/renderer");

const errors = [];
const warnings = [];

function checkExists(path, label, isWarning = false) {
  if (!existsSync(path)) {
    const msg = `MISSING: ${label} (${relative(projectRoot, path)})`;
    if (isWarning) {
      warnings.push(msg);
    } else {
      errors.push(msg);
    }
    return false;
  }
  return true;
}

function main() {
  console.log("[verify-desktop-pet-package] 开始检查...");

  const mainJs = resolve(distMain, "index.cjs");
  const mainJsAlt = resolve(distMain, "index.js");
  const mainExists = existsSync(mainJs) || existsSync(mainJsAlt);
  const actualMainPath = existsSync(mainJs) ? mainJs : mainJsAlt;

  if (!mainExists) {
    errors.push(`MISSING: dist/main/index.cjs`);
  } else {
    try {
      const mainContent = readFileSync(actualMainPath, "utf8");
      if (mainContent.includes("action-player") || mainContent.includes("ActionPlayer")) {
        errors.push("FORBIDDEN: dist/main 包含旧 ActionPlayer 引用");
      }
      if (mainContent.includes("127.0.0.1:5178")) {
        errors.push("FORBIDDEN: dist/main 包含开发URL硬依赖 (127.0.0.1:5178)");
      }
    } catch {
      warnings.push("WARN: 无法读取 dist/main/index.cjs");
    }
  }

  const indexPreload = resolve(distPreload, "index.cjs");
  checkExists(indexPreload, "dist/preload/index.cjs");

  const animationPreload = resolve(distPreload, "animation-preload.cjs");
  checkExists(animationPreload, "dist/preload/animation-preload.cjs");

  const combinedPreload = resolve(distPreload, "pet-combined-preload.cjs");
  if (existsSync(combinedPreload)) {
    errors.push("FORBIDDEN: dist/preload/pet-combined-preload.cjs 仍存在");
  }

  const petPreload = resolve(distPreload, "pet-preload.cjs");
  if (existsSync(petPreload)) {
    errors.push("FORBIDDEN: dist/preload/pet-preload.cjs 仍存在");
  }

  const petHtml = resolve(distRenderer, "pet.html");
  if (checkExists(petHtml, "dist/renderer/pet.html")) {
    try {
      const htmlContent = readFileSync(petHtml, "utf8");
      const tsMatches = htmlContent.match(/src="[^"]*\.ts"/g);
      if (tsMatches) {
        errors.push(`FORBIDDEN: pet.html 引用 .ts 文件: ${tsMatches.join(", ")}`);
      }
      const tsxMatches = htmlContent.match(/src="[^"]*\.tsx"/g);
      if (tsxMatches) {
        errors.push(`FORBIDDEN: pet.html 引用 .tsx 文件: ${tsxMatches.join(", ")}`);
      }
    } catch {
      warnings.push("WARN: 无法读取 dist/renderer/pet.html");
    }
  }

  if (warnings.length > 0) {
    console.log("[verify-desktop-pet-package] warnings:");
    for (const w of warnings) {
      console.log(`  WARN: ${w}`);
    }
  }

  if (errors.length > 0) {
    console.error("[verify-desktop-pet-package] FAILED:");
    for (const e of errors) {
      console.error(`  ERROR: ${e}`);
    }
    process.exit(1);
  }

  console.log("[verify-desktop-pet-package] PASSED");
}

main();
