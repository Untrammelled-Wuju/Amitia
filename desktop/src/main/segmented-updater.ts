import { session } from "electron";
import type { AppUpdater } from "electron-updater";
import {
  downloadSegmented,
  RangeUnsupportedError,
  type SegmentedProgress,
} from "./segmented-download";

interface UpdateDownloadOptions {
  headers?: Record<string, unknown> | null;
  sha512?: string | null;
  cancellationToken?: {
    cancelled: boolean;
    onCancel: (handler: () => void) => void;
    removeListener: (event: string, handler: () => void) => void;
  };
  onProgress?: (progress: SegmentedProgress) => void;
}

interface UpdateHttpExecutor {
  download: (url: URL, destination: string, options: UpdateDownloadOptions) => Promise<string>;
}

interface UpdaterWithExecutor extends AppUpdater {
  httpExecutor: UpdateHttpExecutor | null;
}

const patchedUpdaters = new WeakSet<object>();

function normalizeHeaders(headers: UpdateDownloadOptions["headers"]) {
  const result: Record<string, string> = {};
  for (const [key, value] of Object.entries(headers ?? {})) {
    if (typeof value === "string" || typeof value === "number") {
      result[key] = String(value);
    }
  }
  return result;
}

export function installSegmentedFullDownload(updater: AppUpdater) {
  const internal = updater as UpdaterWithExecutor;
  if (!internal.httpExecutor || patchedUpdaters.has(updater)) return;
  patchedUpdaters.add(updater);
  const executor = internal.httpExecutor;
  const originalDownload = executor.download.bind(executor);
  const updaterSession = session.fromPartition("electron-updater", { cache: false });
  executor.download = async (url, destination, options) => {
    if (options.cancellationToken?.cancelled) throw new Error("cancelled");
    const controller = new AbortController();
    const cancel = () => controller.abort();
    options.cancellationToken?.onCancel(cancel);
    try {
      await downloadSegmented({
        url: url.toString(),
        destination,
        expectedSha512: options.sha512,
        headers: normalizeHeaders(options.headers),
        signal: controller.signal,
        fetch: (input, init) => updaterSession.fetch(input, init),
        onProgress: options.onProgress,
      });
      return destination;
    } catch (error) {
      if (controller.signal.aborted) throw error;
      if (error instanceof RangeUnsupportedError) {
        return originalDownload(url, destination, options);
      }
      console.warn("[UpdateManager] 多线程下载失败，回退单线程:", error);
      return originalDownload(url, destination, options);
    } finally {
      options.cancellationToken?.removeListener("cancel", cancel);
    }
  };
}
