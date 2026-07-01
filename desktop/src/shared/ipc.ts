export const IPC_CHANNELS = {
  getEnvironment: "amitia:environment:get",
  getDeploymentConfig: "amitia:deployment:get",
  saveDeploymentConfig: "amitia:deployment:save",
  getRuntimeStatus: "amitia:runtime:status",
  openLogsDirectory: "amitia:logs:open",
  runtimeStatusChanged: "amitia:runtime:status-changed",
} as const

export type IpcChannel = typeof IPC_CHANNELS[keyof typeof IPC_CHANNELS]
