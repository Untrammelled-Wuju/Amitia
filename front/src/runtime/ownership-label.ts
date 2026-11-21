export type OwnershipCategory =
  | "角色"
  | "对话"
  | "群聊"
  | "团队"
  | "skill"
  | "mcp"
  | "任务";

export const CORE_OWNER_PREFIX = "统筹中心";
export const DEVICE_OWNER_PREFIX = "设备";
export const UNNAMED_OWNER_DEVICE = "未命名设备";

const CATEGORIES = new Set<string>(["角色", "对话", "群聊", "团队", "skill", "mcp", "任务"]);

export type OwnershipInput = {
  coordinated: boolean;
  deviceName?: string;
};

export function ownershipPrefix(input: OwnershipInput): string {
  if (input.coordinated) return CORE_OWNER_PREFIX;
  const name = (input.deviceName || "").trim() || UNNAMED_OWNER_DEVICE;
  return `${DEVICE_OWNER_PREFIX}${name}`;
}

export function hasOwnershipPrefix(name: string): boolean {
  const trimmed = (name || "").trim();
  if (!trimmed) return false;
  const parts = trimmed.split("-");
  if (parts.length < 3) return false;
  if (parts[0] === CORE_OWNER_PREFIX) return CATEGORIES.has(parts[1]);
  return (
    parts[0].startsWith(DEVICE_OWNER_PREFIX) &&
    parts[0].length > DEVICE_OWNER_PREFIX.length &&
    CATEGORIES.has(parts[1])
  );
}

export function coreOwnershipLabel(category: OwnershipCategory, name: string): string {
  return ownershipLabel({ coordinated: true, category, name });
}

export function deviceOwnershipLabel(deviceName: string, category: OwnershipCategory, name: string): string {
  return ownershipLabel({ coordinated: false, deviceName, category, name });
}

export function ownershipLabel(
  input: OwnershipInput & { category: OwnershipCategory; name: string },
): string {
  const base = (input.name || "").trim();
  const prefix = ownershipPrefix(input);
  if (!base) return prefix;
  if (hasOwnershipPrefix(base)) return base;
  return `${prefix}-${input.category}-${base}`;
}
