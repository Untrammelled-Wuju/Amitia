#!/usr/bin/env node

import { spawn, spawnSync } from "node:child_process";
import {
  readFileSync,
  existsSync,
  statSync,
  readdirSync,
} from "node:fs";
import { join, basename } from "node:path";
import { fileURLToPath } from "node:url";
import { dirname } from "node:path";
import { verifyReleaseGateStamp } from "./release-integrity.mjs";

const __dirname = dirname(fileURLToPath(import.meta.url));
const configPath = join(__dirname, ".publish-config.json");
const releaseDir = join(__dirname, "..", "release");

function formatBytes(bytes) {
  const mb = bytes / 1024 / 1024;
  if (mb >= 1) return `${mb.toFixed(2)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

function uploadFile(localPath, remoteUrl, user, password, insecure) {
  return new Promise((resolve, reject) => {
    const args = [
      "-T", localPath,
      remoteUrl,
      "--user", `${user}:${password}`,
      "--ftp-create-dirs",
      "--progress-bar",
    ];
    if (insecure) {
      args.push("--insecure");
    }

    const proc = spawn("curl", args, {
      stdio: "inherit",
    });

    proc.on("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`curl 退出码: ${code}`));
    });

    proc.on("error", () => {
      reject(new Error("curl 不可用，请确认系统已安装 curl (Windows 10+ 自带)"));
    });
  });
}

function shellQuote(value) {
  return `'${String(value).replaceAll("'", "'\\\"'\\\"'")}'`;
}

async function main() {
  if (!existsSync(configPath)) {
    console.error("配置文件不存在: scripts/.publish-config.json");
    console.error("请复制 .publish-config.example.json 为 .publish-config.json 并填入服务器信息");
    process.exit(1);
  }

  const config = JSON.parse(readFileSync(configPath, "utf-8"));

  let verified;
  try {
    verified = verifyReleaseGateStamp();
    console.log(
      `[release-gate] verified source + artifacts for ${verified.stamp.packageVersion} (${verified.stamp.sourceGateSha256.slice(0, 12)}...)`,
    );
  } catch (error) {
    console.error(
      `发布被阻止：${error instanceof Error ? error.message : String(error)}`,
    );
    console.error("请重新运行 pnpm dist:win，通过完整桌宠发布门禁后再上传。");
    process.exit(1);
  }

  const packagedVerify = spawnSync(
    process.execPath,
    [join(__dirname, "verify-packaged-desktop-pet.mjs"), join(releaseDir, "win-unpacked")],
    { stdio: "inherit" },
  );
  if (packagedVerify.error || packagedVerify.status !== 0) {
    console.error("发布被阻止：win-unpacked 桌宠完整性验证未通过，请重新构建。");
    process.exit(1);
  }

  if (!existsSync(releaseDir)) {
    console.error("release 目录不存在，请先运行 pnpm dist:win 构建");
    process.exit(1);
  }

  const files = readdirSync(releaseDir);
  const exeFile = files.find((f) => /^AmitiaSetup-.*-x64\.exe$/.test(f));
  const blockmapFile = files.find((f) =>
    /^AmitiaSetup-.*-x64\.exe\.blockmap$/.test(f),
  );
  const ymlFile = files.find((f) => f === "latest.yml");

  const missing = [];
  if (!exeFile) missing.push("AmitiaSetup-*-x64.exe");
  if (!blockmapFile) missing.push("AmitiaSetup-*-x64.exe.blockmap");
  if (!ymlFile) missing.push("latest.yml");

  if (missing.length > 0) {
    console.error("构建产物不完整，缺少:", missing.join(", "));
    console.error("请先运行 pnpm dist:win 构建");
    process.exit(1);
  }

  const uploadFiles = [
    join(releaseDir, exeFile),
    join(releaseDir, blockmapFile),
    join(releaseDir, ymlFile),
  ];
  const stagingRoot = String(config.stagingRemotePath || `${config.remotePath}/.staging`).replace(/\/+$/, "");
  const stagingPath = `${stagingRoot}/${verified.stamp.packageVersion}`;

  console.log("=== Amitia 发布工具 ===\n");
  console.log("FTP 服务器:", `${config.host}:${config.port || 21}`);
  console.log("临时目录:", stagingPath);
  console.log("\n待上传文件:");
  for (const f of uploadFiles) {
    const stat = statSync(f);
    console.log(`  ${basename(f)} (${formatBytes(stat.size)})`);
  }
  console.log("");

  const protocol = config.secure ? "ftps" : "ftp";
  const baseUrl = `${protocol}://${config.host}:${config.port || 21}${stagingPath}`;

  for (const localPath of uploadFiles) {
    const fileName = basename(localPath);
    const fileSize = statSync(localPath).size;
    const remoteUrl = `${baseUrl}/${fileName}`;

    console.log(`上传 ${fileName} (${formatBytes(fileSize)})...`);
    try {
      await uploadFile(
        localPath,
        remoteUrl,
        config.user,
        config.password,
        config.insecure || false,
      );
    } catch (err) {
      console.error(`\n上传失败: ${err.message}`);
      process.exit(1);
    }
    console.log(`\n  ${fileName} 上传完成\n`);
  }

  if (config.serverReleaseRoot) {
    const root = String(config.serverReleaseRoot).replace(/\/+$/, "");
    const command = [
      "set -euo pipefail",
      `release_root=${shellQuote(root)}`,
      `staged_dir=${shellQuote(`${root}/.staging/${verified.stamp.packageVersion}`)}`,
      'install -m 0644 "$staged_dir/latest.yml" "$release_root/latest.yml"',
      `install -m 0644 "$staged_dir/${exeFile}" "$release_root/${exeFile}"`,
      `install -m 0644 "$staged_dir/${blockmapFile}" "$release_root/${blockmapFile}"`,
    ].join("; ");
    console.log("\n服务器替换命令:");
    console.log(command);
  } else {
    console.log("\n暂存上传完成。请在 scripts/.publish-config.json 设置 serverReleaseRoot 后生成服务器替换命令。");
  }
}

main();
