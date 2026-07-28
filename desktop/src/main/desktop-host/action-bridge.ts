import { BrowserWindow } from "electron";
import type { ActionInvokeRequest, ActionInvokeResult } from "./types";

export class DesktopActionBridge {
  private readonly mainWindow: BrowserWindow;
  private readonly timeoutMs: number;

  constructor(mainWindow: BrowserWindow, timeoutMs = 30000) {
    this.mainWindow = mainWindow;
    this.timeoutMs = timeoutMs;
  }

  async invokeAction(
    request: ActionInvokeRequest,
  ): Promise<ActionInvokeResult> {
    if (this.mainWindow.isDestroyed()) {
      return { success: false, error: "Main window is destroyed" };
    }
    try {
      const token = await this.mainWindow.webContents.executeJavaScript(
        'localStorage.getItem("ai-companion-token")',
        true,
      );
      if (typeof token !== "string" || token.length === 0) {
        return { success: false, error: "Authentication required" };
      }
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), this.timeoutMs);
      const response = await fetch(
        `http://127.0.0.1:18899/api/extensions/desktop/contributions/${encodeURIComponent(request.contributionId)}/invoke`,
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
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
