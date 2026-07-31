export const PET_PROTOCOL_SCHEME = "amitia-pet";

export const ANIMATION_IPC_CHANNELS = {
  getPackageSnapshot: "pet:animation:get-package-snapshot",
  resolveResourceUrl: "pet:animation:resolve-resource-url",
  reportEvent: "pet:animation:report-event",
  reportSnapshot: "pet:animation:report-snapshot",
  getDiagnostics: "pet:animation:get-diagnostics",
  sendClick: "pet:animation:click",
  sendDoubleClick: "pet:animation:double-click",
  sendHover: "pet:animation:hover",
  playAction: "pet:animation:play-action",
  pause: "pet:animation:pause",
  resume: "pet:animation:resume",
  stop: "pet:animation:stop",
  switchPackage: "pet:animation:switch-package",
  windowHidden: "pet:animation:window-hidden",
  windowShown: "pet:animation:window-shown",
  systemSuspend: "pet:animation:system-suspend",
  systemResume: "pet:animation:system-resume",
  recovery: "pet:animation:recovery",
  updateDefaultAction: "pet:animation:update-default-action",
  rendererReady: "pet:animation:renderer-ready",
  rendererReadyAck: "pet:animation:renderer-ready-ack",
  hitMask: "pet:animation:hit-mask",
} as const;

export type AnimationIpcChannel =
  (typeof ANIMATION_IPC_CHANNELS)[keyof typeof ANIMATION_IPC_CHANNELS];
