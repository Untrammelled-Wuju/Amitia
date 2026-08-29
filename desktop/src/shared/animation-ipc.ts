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
  rendererBootstrapped: "pet:animation:renderer-bootstrapped",
  runtimeReady: "pet:animation:runtime-ready",
  runtimeInitFailed: "pet:animation:runtime-init-failed",
  hitMask: "pet:animation:hit-mask",
  dragStart: "pet:animation:drag-start",
  dragMove: "pet:animation:drag-move",
  dragEnd: "pet:animation:drag-end",
  dragCancel: "pet:animation:drag-cancel",
} as const;

export type AnimationIpcChannel =
  (typeof ANIMATION_IPC_CHANNELS)[keyof typeof ANIMATION_IPC_CHANNELS];


export interface PetPointerIpcPayload {
  canvasX: number;
  canvasY: number;
  screenX: number;
  screenY: number;
  occurredAt: number;
}

export interface PetDragIpcPayload {
  pointerId: number;
  screenX: number;
  screenY: number;
  canvasX: number;
  canvasY: number;
  occurredAt: number;
}

export interface PetHitMaskPayload {
  width: number;
  height: number;
  data: Uint8Array;
  threshold: number;
  packageRevision: number;
  actionKey: string;
  frameIndex: number;
  playbackInstanceId: string;
  maskRevision: number;
}

export interface RuntimeReadyPayload {
  snapshotApplied: true;
  packageId: string;
  packageRevision: number;
  defaultActionKey: string;
}

export interface RuntimeInitFailedPayload {
  reason: string;
  packageId?: string;
  packageRevision?: number;
}
