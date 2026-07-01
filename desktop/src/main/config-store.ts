import { mkdir, readFile, writeFile } from "node:fs/promises"
import { dirname, join } from "node:path"
import { app } from "electron"
import { validateDeploymentConfig } from "../shared/deployment"
import type { DeploymentModeConfig } from "../shared/types"

export class ConfigStore {
  private readonly filePath: string

  constructor(filePath = join(app.getPath("userData"), "deployment-config.json")) {
    this.filePath = filePath
  }

  async getDeploymentConfig(): Promise<DeploymentModeConfig> {
    try {
      const raw = await readFile(this.filePath, "utf8")
      return validateDeploymentConfig(JSON.parse(raw))
    } catch {
      return { mode: "local" }
    }
  }

  async saveDeploymentConfig(config: DeploymentModeConfig): Promise<DeploymentModeConfig> {
    const next = validateDeploymentConfig(config)
    await mkdir(dirname(this.filePath), { recursive: true })
    await writeFile(this.filePath, JSON.stringify(next, null, 2), "utf8")
    return next
  }
}
