export const IPC_CHANNELS = {
  getEnvironment: "amitia:environment:get",
  getDeploymentConfig: "amitia:deployment:get",
  saveDeploymentConfig: "amitia:deployment:save",
  getRuntimeStatus: "amitia:runtime:status",
  openLogsDirectory: "amitia:logs:open",
  minimizeWindow: "amitia:window:minimize",
  toggleMaximizeWindow: "amitia:window:toggle-maximize",
  closeWindow: "amitia:window:close",
  runtimeStatusChanged: "amitia:runtime:status-changed",
  selectAgentSkillDirectory: "amitia:agent-skill:select-directory",
  selectMCPRoot: "amitia:mcp:select-root",
  selectExtensionPackage: "amitia:extension-package:select-file",
  saveExtensionPackage: "amitia:extension-package:save",
  getAutoLaunch: "amitia:auto-launch:get",
  setAutoLaunch: "amitia:auto-launch:set",
} as const

export type IpcChannel = typeof IPC_CHANNELS[keyof typeof IPC_CHANNELS]
