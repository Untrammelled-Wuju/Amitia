import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { expect, it } from "vitest";
import { ConfigStore } from "./config-store";

it("treats missing deployment and auto-launch files as first-run defaults", async () => {
  const configPath = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "amitia-config-")), "deployment-config.json");
  const store = new ConfigStore(configPath);

  await expect(store.getDeploymentConfig()).resolves.toEqual({ mode: "local" });
  await expect(store.getAutoLaunch()).resolves.toBe(false);
});
