import { BrowserWindow } from "electron";
import { IPC_CHANNELS } from "../../shared/ipc";
import { getDesktopInstanceID } from "../desktop-identity";
import {
  getMeshIdentity,
  getMeshStatus,
  watchMeshState,
  postMeshBootstrap,
  deleteMeshCredential,
  claimPinnedMeshPairing,
} from "./local-agent-client";
import { claimPairing } from "./remote-bootstrap-client";
import {
  type DeviceMeshAgentState,
  type DeviceMeshStatusResponse,
  type CloudPairingDeviceIdentity,
  type MeshPairingInput,
} from "./protocol";

export type MeshCoordinatorEvent =
  | { type: "state"; state: DeviceMeshAgentState }
  | { type: "status"; status: DeviceMeshStatusResponse }
  | { type: "error"; code: string; message: string };

export interface MeshCoordinatorOptions {
  getMainWindow: () => BrowserWindow | null;
}

export class DeviceMeshCoordinator {
  private getMainWindow: () => BrowserWindow | null;
  private readonly platform: string;

  private stopped = true;
  private stopStateStream: (() => void) | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private lastState: DeviceMeshAgentState | null = null;
  private lastStatus: DeviceMeshStatusResponse | null = null;
  private lastDeviceId: string | null = null;
  private lastRuntimeId: string | null = null;

  constructor(options: MeshCoordinatorOptions) {
    this.getMainWindow = options.getMainWindow;
    this.platform = process.platform === "win32" ? "windows" : process.platform;
  }

  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    void this.refreshIdentity();
    void this.refreshStatus();
    this.connectStateStream();
  }

  stop(): void {
    this.stopped = true;
    this.stopStateStream?.();
    this.stopStateStream = null;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
  }

  isRunning(): boolean {
    return !this.stopped;
  }

  getStatus(): DeviceMeshStatusResponse | null {
    return this.lastStatus;
  }

  async provision(cloudBaseUrl: string, pairing?: MeshPairingInput): Promise<void> {
    const identity = await getMeshIdentity();
    if (!identity) {
      throw new Error("本地 device-mesh 身份不可用，请确保核心以 device-agent Profile 运行");
    }

    this.lastDeviceId = identity.deviceId;
    this.lastRuntimeId = identity.runtimeId;

    const pairingIdentity: CloudPairingDeviceIdentity = {
      deviceId: identity.deviceId,
      runtimeId: identity.runtimeId,
      platform: this.platform,
      label: `Desktop ${getDesktopInstanceID()}`,
    };

    const ticket = pairing?.fingerprint ? await claimPinnedMeshPairing(cloudBaseUrl, pairing) : await claimPairing(cloudBaseUrl, {
      ...pairingIdentity,
      offerToken: pairing?.offerToken?.trim() || undefined,
      setupCode: pairing?.setupCode?.trim() || undefined,
    });

    await postMeshBootstrap({
      cloudBaseUrl,
      bootstrapTicket: ticket.ticket,
      fingerprint: pairing?.fingerprint,
      coreId: pairing?.coreId,
      replaceCandidate: true,
    });

    await this.refreshStatus();
  }

  async deprovision(): Promise<void> {
    await deleteMeshCredential();
    this.lastStatus = null;
    this.emitState("unprovisioned");
  }

  async refreshStatus(): Promise<DeviceMeshStatusResponse | null> {
    try {
      const status = await getMeshStatus();
      if (status) {
        this.lastStatus = status;
        if (!this.lastDeviceId && status.deviceId) this.lastDeviceId = status.deviceId;
        if (!this.lastRuntimeId && status.runtimeId) this.lastRuntimeId = status.runtimeId;
        this.emitStatus(status);
        this.emitState(status.state);
      }
      return status;
    } catch (error) {
  console.error("[original-error]", error);
return null;
    }
  }

  private async refreshIdentity(): Promise<void> {
    try {
      const identity = await getMeshIdentity();
      if (identity) {
        this.lastDeviceId = identity.deviceId;
        this.lastRuntimeId = identity.runtimeId;
      }
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  private connectStateStream(): void {
    if (this.stopped || this.stopStateStream) return;
    this.stopStateStream = watchMeshState(
      (state) => {
        this.reconnectAttempt = 0;
        this.emitState(state as DeviceMeshAgentState);
        void this.refreshStatus();
      },
      (error) => {
        if (error) console.error("[original-error]", error);
        this.stopStateStream?.();
        this.stopStateStream = null;
        this.scheduleStateReconnect();
      },
    );
  }

  private scheduleStateReconnect(): void {
    if (this.stopped || this.reconnectTimer) return;
    const delay = Math.min(30_000, 1000 * 2 ** this.reconnectAttempt++);
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connectStateStream();
    }, delay);
  }

  private emitState(state: DeviceMeshAgentState): void {
    if (this.lastState === state) return;
    this.lastState = state;
    this.sendToRenderer("mesh:state-changed", { state });
  }

  private emitStatus(status: DeviceMeshStatusResponse): void {
    this.sendToRenderer("mesh:status-updated", status);
  }

  private sendToRenderer(channel: string, payload: unknown): void {
    const win = this.getMainWindow();
    if (!win || win.isDestroyed()) return;
    try {
      win.webContents.send(channel, payload);
    } catch (error) {
  console.error("[original-error]", error);
}
  }
}

let singleton: DeviceMeshCoordinator | null = null;
let resolvedGetMainWindow: () => BrowserWindow | null = () => null;

export function getMeshCoordinator(getMainWindow: () => BrowserWindow | null): DeviceMeshCoordinator {
  resolvedGetMainWindow = getMainWindow;
  if (!singleton) {
    singleton = new DeviceMeshCoordinator({
      getMainWindow: () => resolvedGetMainWindow(),
    });
  }
  return singleton;
}
