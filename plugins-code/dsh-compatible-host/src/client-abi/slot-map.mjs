import { DSH_UI_SLOT_RESOLUTIONS } from "./slot-catalog.generated.mjs";

function text(value) {
  return value == null ? "" : String(value).trim();
}

export const DSH_SLOT_ALIASES = Object.freeze(Object.fromEntries(
  Object.values(DSH_UI_SLOT_RESOLUTIONS)
    .filter((entry) => entry.status !== "pending" && entry.targets.length > 0 && entry.key !== entry.targets[0])
    .map((entry) => [entry.key, entry.targets[0]]),
));

export function hasDshSlot(slotId) {
  return Boolean(DSH_UI_SLOT_RESOLUTIONS[text(slotId)]);
}

export function mapDshSlot(slotId, hasAmitiaSlot) {
  const id = text(slotId);
  if (!id) throw new Error("DSH client slot id is required");
  const entry = DSH_UI_SLOT_RESOLUTIONS[id];
  if (!entry) throw new Error(`DSH client slot ${id} is not in the 0.2.2-rc.2 catalog`);
  if (entry.status === "pending" || entry.targets.length === 0) {
    throw new Error(`DSH client slot ${id} is pending ${entry.phase}: ${entry.reason}`);
  }
  const mapped = entry.targets.find((target) => !hasAmitiaSlot || hasAmitiaSlot(target));
  if (!mapped) throw new Error(`DSH client slot ${id} maps to unavailable Amitia slot ${entry.targets[0]}`);
  if (hasAmitiaSlot && !hasAmitiaSlot(mapped)) throw new Error(`DSH client slot ${id} maps to unavailable Amitia slot ${mapped}`);
  return mapped;
}
