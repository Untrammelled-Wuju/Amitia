import { BrowserWindow } from "electron";
import { IPC_CHANNELS } from "../../shared/ipc";
import { getDesktopInstanceID } from "../desktop-identity";
import {
  getMeshIdentity,
  getMeshStatus,
  postMeshBootstrap,
  deleteMeshCredential,
} from "./local-agent-client";
import { createBootstrapTicket } from "./remote-bootstrap-client";
import {
  type DeviceMeshAgentState,
  type DeviceMeshStatusResponse,
  type CloudBootstrapTicketRequest,
} from "./protocol";

export type MeshCoordinatorEvent =
  | { type: "state"; state: DeviceMeshAgentState }
  | { type: "status"; status: DeviceMeshStatusResponse }
  | { type: "error"; code: string; message: string };

export interface MeshCoordinatorOptions {
  getMainWindow: () => BrowserWindow | null;
  pollIntervalMs?: number;
}

const DEFAULT_POLL_INTERVAL_MS = 10000;

export class DeviceMeshCoordinator {
  private getMainWindow: () => BrowserWindow | null;
  private readonly pollIntervalMs: number;
  private readonly platform: string;

  private stopped = true;
  private pollTimer: ReturnType<typeof setTimeout> | null = null;
  private lastState: DeviceMeshAgentState | null = null;
  private lastStatus: DeviceMeshStatusResponse | null = null;
  private lastDeviceId: string | null = null;
  private lastRuntimeId: string | null = null;

  constructor(options: MeshCoordinatorOptions) {
    this.getMainWindow = options.getMainWindow;
    this.pollIntervalMs = options.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS;
    this.platform = process.platform === "win32" ? "windows" : process.platform;
  }

  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    void this.refreshIdentity();
    void this.pollLoop();
  }

  stop(): void {
    this.stopped = true;
    if (this.pollTimer) {
      clearTimeout(this.pollTimer);
      this.pollTimer = null;
    }
  }

  isRunning(): boolean {
    return !this.stopped;
  }

  getStatus(): DeviceMeshStatusResponse | null {
    return this.lastStatus;
  }

  async provision(cloudBaseUrl: string): Promise<void> {
    const identity = await getMeshIdentity();
    if (!identity) {
      throw new Error("本地 device-mesh 身份不可用，请确保核心以 device-agent Profile 运行");
    }

    this.lastDeviceId = identity.deviceId;
    this.lastRuntimeId = identity.runtimeId;

    const ticketReq: CloudBootstrapTicketRequest = {
      deviceId: identity.deviceId,
      runtimeId: identity.runtimeId,
      platform: this.platform,
      label: `Desktop ${getDesktopInstanceID()}`,
    };

    const ticket = await createBootstrapTicket(cloudBaseUrl, ticketReq);

    await postMeshBootstrap({
      cloudBaseUrl,
      bootstrapTicket: ticket.ticket,
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
        this.lastState = status.state;
        if (!this.lastDeviceId && status.deviceId) this.lastDeviceId = status.deviceId;
        if (!this.lastRuntimeId && status.runtimeId) this.lastRuntimeId = status.runtimeId;
        this.emitStatus(status);
        this.emitState(status.state);
      }
      return status;
    } catch {
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
    } catch {}
  }

  private async pollLoop(): Promise<void> {
    while (!this.stopped) {
      try {
        await this.refreshStatus();
      } catch {}
      await new Promise((resolve) => {
        this.pollTimer = setTimeout(resolve, this.pollIntervalMs);
      });
    }
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
    } catch {}
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
