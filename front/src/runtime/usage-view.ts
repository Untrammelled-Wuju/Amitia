export function usageNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

export function usageScopeLabel(data: any): string {
  return data?.scope?.deviceId ? "本设备在当前 Core" : "当前 Core";
}

export function usageMetricSummary(rows: any[], key: string, available: boolean): { total: number | null; average: number | null } {
  if (!available || rows.some(row => usageNumber(row[key]) === null)) return { total: null, average: null };
  const total = rows.reduce((sum, row) => sum + row[key], 0);
  return { total, average: rows.length ? Math.round(total / rows.length) : 0 };
}
