import { existsSync, readFileSync } from "node:fs";
import { resolve, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = fileURLToPath(new URL(".", import.meta.url));
const projectRoot = resolve(__dirname, "..");
const distMain = resolve(projectRoot, "dist/main");
const distPreload = resolve(projectRoot, "dist/preload");
const distRenderer = resolve(projectRoot, "dist/renderer");
const repositoryRoot = resolve(projectRoot, "..");
const strictSemVer = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

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

  try {
    const desktopPackage = JSON.parse(readFileSync(resolve(projectRoot, "package.json"), "utf8"));
    const runtimeVersion = desktopPackage.desktopPetRuntimeVersion;
    const contractVersion = desktopPackage.desktopPetRuntimeContractVersion;
    if (!strictSemVer.test(runtimeVersion ?? "")) {
      errors.push("INVALID: package.json desktopPetRuntimeVersion 必须是严格 SemVer");
    }
    if (!strictSemVer.test(contractVersion ?? "")) {
      errors.push("INVALID: package.json desktopPetRuntimeContractVersion 必须是严格 SemVer");
    }

    const backendVersions = readFileSync(
      resolve(repositoryRoot, "backend/internal/desktoppet/contracts/runtime_version.go"),
      "utf8",
    );
    const backendRuntime = backendVersions.match(/RuntimeVersion\s*=\s*"([^"]+)"/)?.[1];
    const backendContract = backendVersions.match(/RuntimeContractVersion\s*=\s*"([^"]+)"/)?.[1];
    if (!backendRuntime || backendRuntime !== runtimeVersion) {
      errors.push(`MISMATCH: desktop runtime=${runtimeVersion ?? "missing"}, backend=${backendRuntime ?? "missing"}`);
    }
    if (!backendContract || backendContract !== contractVersion) {
      errors.push(`MISMATCH: desktop runtime contract=${contractVersion ?? "missing"}, backend=${backendContract ?? "missing"}`);
    }

    const backendEnvelope = readFileSync(
      resolve(repositoryRoot, "backend/internal/desktoppet/runtime/protocol/v2/envelope.go"),
      "utf8",
    );
    if (!/CurrentSchemaVersion\s*=\s*contracts\.RuntimeContractVersion/.test(backendEnvelope)) {
      errors.push("MISMATCH: Runtime v2 CurrentSchemaVersion 未绑定后端唯一契约常量");
    }
  } catch (error) {
    errors.push(`FAILED: 无法验证桌宠 Runtime 版本源 (${error instanceof Error ? error.message : String(error)})`);
  }

  try {
    const legacyPetHtml = readFileSync(resolve(repositoryRoot, "front/pet.html"), "utf8");
    if (/@fs[A-Za-z]:[\\/]/.test(legacyPetHtml) || /[A-Za-z]:\\[^\n<]+pet-main\.ts/.test(legacyPetHtml)) {
      errors.push("FORBIDDEN: front/pet.html 包含机器相关绝对路径");
    }
  } catch (error) {
    errors.push(`FAILED: 无法检查 front/pet.html (${error instanceof Error ? error.message : String(error)})`);
  }


  try {
    const protocolSource = readFileSync(
      resolve(projectRoot, "src/main/pet/resource-protocol.ts"),
      "utf8",
    );
    if (!/corsEnabled:\s*true/.test(protocolSource)) {
      errors.push("FORBIDDEN: amitia-pet 生产协议未启用 corsEnabled");
    }
    if (!/Access-Control-Allow-Origin/.test(protocolSource)) {
      errors.push("FORBIDDEN: amitia-pet 生产协议缺少 CORS 响应头");
    }

    const managerSource = readFileSync(
      resolve(projectRoot, "src/main/pet/manager.ts"),
      "utf8",
    );
    if (!/clickThroughController\.setMode\(clickThroughMode\)/.test(managerSource)) {
      errors.push("FORBIDDEN: 桌宠启动链未把 clickThroughMode 注入 ClickThroughController");
    }
    if (!/idleController\.updateConfig\(/.test(managerSource)) {
      errors.push("FORBIDDEN: Runtime settings 未动态更新 IdleController 配置");
    }
    if (!/setSoundEnabled\(merged\.soundEnabled\)/.test(managerSource)) {
      errors.push("FORBIDDEN: soundEnabled 未应用到桌宠窗口音频状态");
    }
  } catch (error) {
    errors.push(`FAILED: 无法验证桌宠生产运行时关键接线 (${error instanceof Error ? error.message : String(error)})`);
  }

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
