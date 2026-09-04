import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { validateDeploymentConfig, validateDeploymentConfigForSave } from "../shared/deployment";
import type { DeploymentModeConfig } from "../shared/types";
import { getAmitiaDataDir } from "./path-manager";

export class ConfigStore {
  private readonly filePath: string;
  private readonly autoLaunchPath: string;

  constructor(filePath?: string) {
    this.filePath =
      filePath || join(getAmitiaDataDir(), "config", "deployment-config.json");
    this.autoLaunchPath = join(
      getAmitiaDataDir(),
      "config",
      "auto-launch.json",
    );
  }

  async getDeploymentConfig(): Promise<DeploymentModeConfig> {
    try {
      const raw = await readFile(this.filePath, "utf8");
      return validateDeploymentConfig(JSON.parse(raw));
    } catch {
      return { mode: "local" };
    }
  }

  async saveDeploymentConfig(
    config: DeploymentModeConfig,
  ): Promise<DeploymentModeConfig> {
    const next = validateDeploymentConfigForSave(config);
    await mkdir(dirname(this.filePath), { recursive: true });
    await writeFile(this.filePath, JSON.stringify(next, null, 2), "utf8");
    return next;
  }

  async getAutoLaunch(): Promise<boolean> {
    try {
      const raw = await readFile(this.autoLaunchPath, "utf8");
      const data = JSON.parse(raw);
      return data.enabled === true;
    } catch {
      return false;
    }
  }

  async setAutoLaunch(enabled: boolean): Promise<void> {
    await mkdir(dirname(this.autoLaunchPath), { recursive: true });
    await writeFile(
      this.autoLaunchPath,
      JSON.stringify({ enabled }, null, 2),
      "utf8",
    );
  }
}
