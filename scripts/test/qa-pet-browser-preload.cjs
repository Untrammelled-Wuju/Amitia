const { contextBridge, ipcRenderer } = require('electron')
contextBridge.exposeInMainWorld('amitiaDesktop', {
  getDeploymentConfig: async () => ({ mode: 'local' }),
  getRuntimeEndpoint: async () => ({ httpBaseURL: 'http://127.0.0.1:5178' }),
  getBackendAuthHeaders: async () => ({}),
  getPendingUpdate: async () => null,
  onUpdateAvailable: () => () => {},
  onUpdateDownloadProgress: () => () => {},
  onUpdateDownloaded: () => () => {},
  onUpdateError: () => () => {},
  onUpdateNotAvailable: () => () => {},
  getMeshIdentity: () => ipcRenderer.invoke('qa-mesh-identity'),
})
