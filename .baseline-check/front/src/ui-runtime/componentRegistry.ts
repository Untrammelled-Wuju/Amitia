import { computed } from "vue";
import { useExtensionUIStore } from "@/stores/extensionUI";

export type UIComponentVariant = Record<string, string | number | boolean>;

function readVariants(providerMetadata: Record<string, unknown> | undefined): Record<string, unknown> {
  if (!providerMetadata) return {};
  const variants = providerMetadata.componentVariants ?? providerMetadata.components;
  return variants && typeof variants === "object" && !Array.isArray(variants)
    ? variants as Record<string, unknown>
    : {};
}

export function resolveUIComponentVariant(
  store: ReturnType<typeof useExtensionUIStore>,
  key: string,
): UIComponentVariant {
  const provider = store.getResolvedProvider("ui.components");
  if (!provider || provider.builtin || !provider.enabled) return {};
  const raw = readVariants(provider.metadata)[key];
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
  const result: UIComponentVariant = {};
  for (const [name, value] of Object.entries(raw as Record<string, unknown>)) {
    if (["string", "number", "boolean"].includes(typeof value)) result[name] = value as string | number | boolean;
  }
  return result;
}

function cssDimension(value: unknown): string | undefined {
  if (typeof value === "number") return `${value}px`;
  if (typeof value === "string" && value.trim()) return value.trim();
  return undefined;
}

export function componentVariantStyle(variant: UIComponentVariant): Record<string, string | number> {
  const style: Record<string, string | number> = {};
  const setDimension = (cssKey: string, value: unknown) => {
    const formatted = cssDimension(value);
    if (formatted) style[cssKey] = formatted;
  };
  setDimension("min-height", variant.minHeight);
  setDimension("height", variant.height);
  setDimension("border-radius", variant.radius);
  setDimension("gap", variant.gap);
  setDimension("font-size", variant.fontSize);
  setDimension("--ui-component-icon-size", variant.iconSize);
  setDimension("--ui-component-border-width", variant.borderWidth);
  const paddingX = cssDimension(variant.paddingX);
  if (paddingX) { style["padding-left"] = paddingX; style["padding-right"] = paddingX; }
  const paddingY = cssDimension(variant.paddingY);
  if (paddingY) { style["padding-top"] = paddingY; style["padding-bottom"] = paddingY; }
  if (typeof variant.fontWeight === "number" || typeof variant.fontWeight === "string") {
    style["font-weight"] = variant.fontWeight as string | number;
  }
  if (typeof variant.opacity === "number") style.opacity = variant.opacity;
  return style;
}

export function useUIComponentVariant(key: string) {
  const store = useExtensionUIStore();
  const variant = computed(() => resolveUIComponentVariant(store, key));
  const style = computed(() => componentVariantStyle(variant.value));
  return { variant, style };
}
