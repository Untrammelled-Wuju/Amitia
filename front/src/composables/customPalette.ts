export type TextColorMode = "auto" | "dark" | "light" | "custom";
export interface CustomPalette {
  enabled: boolean;
  primary: string;
  secondary: string;
  text: string;
  textMode: TextColorMode;
}
export const defaultCustomPalette: CustomPalette = { enabled: false, primary: "#6C8FEA", secondary: "#52B788", text: "#24221F", textMode: "auto" };
export function normalizeColor(value: unknown, fallback: string): string {
  const raw = String(value ?? "").trim();
  const color = raw.startsWith("#") ? raw : `#${raw}`;
  return /^#[\da-f]{6}$/i.test(color) ? color.toUpperCase() : fallback;
}
export function normalizeCustomPalette(value: unknown): CustomPalette {
  const raw = value && typeof value === "object" ? value as Partial<CustomPalette> : {};
  return {
    enabled: raw.enabled === true,
    primary: normalizeColor(raw.primary, defaultCustomPalette.primary),
    secondary: normalizeColor(raw.secondary, defaultCustomPalette.secondary),
    text: normalizeColor(raw.text, defaultCustomPalette.text),
    textMode: ["auto", "dark", "light", "custom"].includes(raw.textMode ?? "") ? raw.textMode! : "auto",
  };
}
function luminance(hex: string): number {
  const color = normalizeColor(hex, "#000000").slice(1);
  const channels = [0, 2, 4].map(i => {
    const value = parseInt(color.slice(i, i + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
}
export function contrastRatio(text: string, background: string): number {
  const a = luminance(text), b = luminance(background);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}
export function readableText(background: string): string {
  return contrastRatio("#000000", background) >= contrastRatio("#FFFFFF", background) ? "#000000" : "#FFFFFF";
}
export function supportingText(text: string, background: string, amount: number): string {
  const channels = [1, 3, 5].map(i => Math.round(parseInt(text.slice(i, i + 2), 16) * (1 - amount) + parseInt(background.slice(i, i + 2), 16) * amount));
  const candidate = `#${channels.map(value => value.toString(16).padStart(2, "0")).join("")}`;
  return contrastRatio(candidate, background) >= 4.5 ? candidate : text;
}
export function paletteText(palette: CustomPalette, background: string): string {
  return palette.textMode === "custom" ? palette.text : palette.textMode === "dark" ? "#24221F" : palette.textMode === "light" ? "#F5F5F5" : readableText(background);
}
