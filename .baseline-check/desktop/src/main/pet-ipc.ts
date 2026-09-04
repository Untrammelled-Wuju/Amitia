import { ipcMain } from "electron";
import type { DesktopPetManager } from "./pet/manager";
import type { RuntimeSettingsInfo } from "./pet/manager";

export const PET_IPC_CHANNELS = {
  enable: "pet:enable",
  disable: "pet:disable",
  switch: "pet:switch",
  playAction: "pet:play-action",
  recenter: "pet:recenter",
  updateSettings: "pet:update-settings",
  updateDefaultAction: "pet:update-default-action",
  list: "pet:list",
  get: "pet:get",
  state: "pet:state",
} as const;

export function registerPetIpcHandlers(manager: DesktopPetManager): void {
  ipcMain.handle(PET_IPC_CHANNELS.enable, async (_event, installationId: string) => {
    await manager.enableInstallation(installationId);
    return { ok: true };
  });

  ipcMain.handle(PET_IPC_CHANNELS.disable, async () => {
    await manager.disableInstallation();
    return { ok: true };
  });

  ipcMain.handle(PET_IPC_CHANNELS.switch, async (_event, installationId: string) => {
    await manager.switchInstallation(installationId);
    return { ok: true };
  });

  ipcMain.handle(PET_IPC_CHANNELS.playAction, async (_event, actionKey: string) => {
    await manager.playAction(actionKey);
    return { ok: true };
  });

  ipcMain.handle(PET_IPC_CHANNELS.recenter, async () => {
    await manager.recenter();
    return { ok: true };
  });

  ipcMain.handle(
    PET_IPC_CHANNELS.updateSettings,
    async (_event, settings: Partial<RuntimeSettingsInfo>) => {
      const result = await manager.updateSettings(settings);
      return { ok: true, ...(result ?? {}) };
    },
  );

  ipcMain.handle(
    PET_IPC_CHANNELS.updateDefaultAction,
    async (_event, actionKey: string) => {
      const result = await manager.updateDefaultAction(actionKey);
      return { ok: true, ...result };
    },
  );

  ipcMain.handle(PET_IPC_CHANNELS.list, async () => {
    const items = await manager.listInstallations();
    return { items, total: items.length };
  });

  ipcMain.handle(PET_IPC_CHANNELS.get, async (_event, installationId: string) => {
    return manager.getInstallation(installationId);
  });

  ipcMain.handle(PET_IPC_CHANNELS.state, () => {
    return {
      state: manager.getState(),
      activeInstallationId: manager.getActiveInstallationId(),
      activeInstallation: manager.getActiveInstallation(),
      activeSettings: manager.getActiveSettings(),
    };
  });
}
