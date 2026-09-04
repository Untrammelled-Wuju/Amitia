import { BrowserWindow } from "electron";
import type { BusinessCoreClient } from "../business-core-client";
import type { ActionInvokeRequest, ActionInvokeResult } from "./types";

export class DesktopActionBridge {
  private readonly mainWindow: BrowserWindow;
  private readonly businessCoreClient: BusinessCoreClient;
  private readonly timeoutMs: number;

  constructor(mainWindow: BrowserWindow, businessCoreClient: BusinessCoreClient, timeoutMs = 30000) {
    this.mainWindow = mainWindow;
    this.businessCoreClient = businessCoreClient;
    this.timeoutMs = timeoutMs;
  }

  async invokeAction(
    request: ActionInvokeRequest,
  ): Promise<ActionInvokeResult> {
    if (this.mainWindow.isDestroyed()) {
      return { success: false, error: "Main window is destroyed" };
    }
    try {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), this.timeoutMs);
      const response = await this.businessCoreClient.fetch(
        `/api/extensions/desktop/contributions/${encodeURIComponent(request.contributionId)}/invoke`,
        {
          method: "POST",
          body: JSON.stringify({
            extensionId: request.extensionId,
            characterId: request.scope?.characterId,
            conversationId: request.scope?.conversationId,
            global: request.scope?.global,
            input: request.input,
          }),
          signal: controller.signal,
        },
      ).finally(() => clearTimeout(timer));
      const payload = await response.json();
      if (!response.ok) {
        return {
          success: false,
          error: payload?.message ?? `Action invoke failed: ${response.status}`,
        };
      }
      return { success: true, result: payload?.result };
    } catch (err) {
      return {
        success: false,
        error: err instanceof Error ? err.message : String(err),
      };
    }
  }

  cleanup(): void {
  }
}
