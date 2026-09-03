export const TYPES = [
  { label: "个人信息", value: "personal_info" },
  { label: "爱好", value: "hobby" },
  { label: "偏好", value: "preference" },
  { label: "事实", value: "fact" },
  { label: "计划", value: "plan" },
  { label: "习惯", value: "habit" },
  { label: "关系", value: "relationship" },
  { label: "自定义", value: "custom" },
]
export const SOURCES = [
  { label: "手动", value: "manual" },
  { label: "摘要", value: "summary" },
  { label: "提取", value: "extracted" },
  { label: "导入", value: "import" },
];
export const SCOPE_TYPES = [
  { label: "用户全局", value: "user_global" },
  { label: "用户-角色", value: "user_character" },
  { label: "角色自身", value: "character_self" },
  { label: "世界", value: "world" },
];
export const SENSITIVITY_OPTIONS = [
  { label: "公开", value: "public" },
  { label: "内部", value: "internal" },
  { label: "私密", value: "private" },
  { label: "高度敏感", value: "secret" },
];

export function typeLabel(value: string) {
  return TYPES.find((item) => item.value === value)?.label || value;
}
export function sourceLabel(value: string) {
  return SOURCES.find((item) => item.value === value)?.label || value;
}
export function importanceColor(value: number) {
  return value >= 8
    ? "var(--ac-color-danger)"
    : value >= 5
      ? "var(--ac-color-warning)"
      : "var(--ac-color-primary)";
}
export function isExpired(expiresAt?: string) {
  return !!expiresAt && new Date(expiresAt).getTime() < Date.now();
}
export function legacyScopeToScopeType(scope: string) {
  return scope === "user" ? "user_global" : "user_character";
}
export function rowScopeType(row: any) {
  return (
    row.scopeType ||
    row.scope_type ||
    legacyScopeToScopeType(row.scope || "character")
  );
}
export function scopeTypeToScope(scopeType: string) {
  return scopeType === "user_global"
    ? "user"
    : scopeType === "world"
      ? "world"
      : "character";
}
export function scopeTypeLabel(row: any) {
  return (
    SCOPE_TYPES.find((item) => item.value === rowScopeType(row))?.label ||
    rowScopeType(row)
  );
}
export function rowSensitivity(row: any) {
  return (
    row.sensitivity || row.sensitivityLevel || row.sensitivity_level || "internal"
  );
}
export function sensitivityLabel(value: string) {
  return (
    SENSITIVITY_OPTIONS.find((item) => item.value === value)?.label || value
  );
}
export function sensitivityTagType(value: string) {
  return value === "secret"
    ? "danger"
    : value === "private"
      ? "warning"
      : value === "public"
        ? "success"
        : "info";
}
export function readBooleanFlag(
  row: any,
  keys: string[],
  defaultValue: boolean,
) {
  for (const key of keys) {
    const value = row?.[key];
    if (typeof value === "boolean") return value;
    if (typeof value === "number") return value !== 0;
    if (typeof value === "string") {
      const normalized = value.trim().toLowerCase();
      if (["true", "1", "yes", "y", "on"].includes(normalized)) return true;
      if (["false", "0", "no", "n", "off"].includes(normalized)) return false;
    }
  }
  return defaultValue;
}
export function rowAllowContextUse(row: any) {
  return readBooleanFlag(row, ["allowContextUse", "allow_context_use"], true);
}
export function rowAllowProactiveMention(row: any) {
  return readBooleanFlag(
    row,
    ["allowProactiveMention", "allow_proactive_mention"],
    false,
  );
}
export function rowRequiresConfirmation(row: any) {
  return readBooleanFlag(
    row,
    ["requiresConfirmation", "requires_confirmation"],
    false,
  );
}
export function scopeTypeTagType(row: any) {
  const value = rowScopeType(row);
  return value === "user_global" || value === "world"
    ? "success"
    : value === "character_self"
      ? "warning"
      : "info";
}
export function fmtDate(value: string) {
  if (!value) return "";
  try {
    return new Date(value).toLocaleString("zh-CN");
  } catch {
    return value;
  }
}
export function parseMemIDs(raw: string): string[] {
  if (!raw) return [];
  try {
    return JSON.parse(raw);
  } catch {
    return [];
  }
}
export function maxScore(raw: string): string {
  if (!raw) return "--";
  try {
    const values = JSON.parse(raw);
    if (!Array.isArray(values) || values.length === 0) return "--";
    return (
      (Math.max(...values.map((item: any) => item.score || 0)) * 100).toFixed(
        1,
      ) + "%"
    );
  } catch {
    return "--";
  }
}
