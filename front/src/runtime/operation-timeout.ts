export interface OperationTimeout { disabled: boolean; seconds: number }
const settingsByBackend = new Map<string, OperationTimeout>();

export function setOperationTimeout(baseURL: string, settings: OperationTimeout) {
  if (typeof settings.disabled === "boolean" && Number.isInteger(settings.seconds)
    && settings.seconds >= 30 && settings.seconds <= 1800) {
    settingsByBackend.set(baseURL, { ...settings });
  }
}

export function operationRequestTimeout(baseURL: string): number {
  const settings = settingsByBackend.get(baseURL) || { disabled: false, seconds: 180 };
  return settings.disabled ? 0 : (settings.seconds + 5) * 1000;
}

export function readOperationTimeoutHeaders(baseURL: string, headers: Record<string, any>) {
  const disabled = headers["x-amitia-timeout-disabled"];
  if (disabled === "true" || disabled === "false") {
    setOperationTimeout(baseURL, { disabled: disabled === "true", seconds: Number(headers["x-amitia-timeout-seconds"]) });
  }
}

export function formatTimeout(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  const remainder = seconds % 60;
  return minutes ? `${minutes} 分钟${remainder ? ` ${remainder} 秒` : ""}` : `${seconds} 秒`;
}
