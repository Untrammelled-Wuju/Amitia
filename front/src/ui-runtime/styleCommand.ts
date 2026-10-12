import { useTheme, type CornerStyle, type ThemePreset, type ThemeState } from "../composables/useTheme";
import { useBackgroundPreference } from "../composables/useBackgroundPreference";
import {
  normalizeBackground,
  readBackground,
  readBackgroundHistoryMedia,
  writeBackgroundHistoryMedia,
  type BackgroundSettings,
} from "../composables/backgroundStorage";
import {
  addStyleRules,
  captureStyleDocument,
  clearStylePatch,
  listStyleRules,
  popStyleHistory,
  popStyleTransaction,
  pushStyleHistory,
  recordStyleTransaction,
  removeStyleRules,
  replaceStyleDocument,
  replaceStylePatch,
  stylePatch,
  styleRevision,
  styleRules,
  type StyleDocument,
  type StylePatch,
  type StyleRule,
  type StyleRuleFilters,
  type StyleScope,
  type StyleSource,
} from "./styleProfile";
import { applyStyleRuntime, applyStyleRules } from "./styleRuntime";
import { inspectDomTargets } from "./styleEngine";
import { normalizeStylePageId } from "./stylePage";

type Json = Record<string, unknown>;

function isRecord(value: unknown): value is Json {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

function clone<T>(value: T): T {
  if (typeof structuredClone === "function") return structuredClone(value);
  return JSON.parse(JSON.stringify(value)) as T;
}

function mapValue(source: Json, key: string): Json {
  const value = source[key];
  return isRecord(value) ? value : {};
}

function color(value: unknown): string | null {
  const candidate = String(value ?? "").trim();
  return /^#[0-9a-f]{6}$/i.test(candidate) ? candidate.toUpperCase() : null;
}

function deepMerge(base: Json, overlay: Json): Json {
  const merged: Json = { ...base };
  for (const [key, value] of Object.entries(overlay)) {
    const current = merged[key];
    merged[key] = isRecord(current) && isRecord(value)
      ? deepMerge(current, value)
      : value;
  }
  return merged;
}

async function snapshot(revision: number): Promise<Json> {
  const theme = useTheme();
  const background = useBackgroundPreference();
  const persisted = await readBackground();
  let backgroundMediaKey = "";
  if (persisted.media) {
    backgroundMediaKey = `style-history-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    await writeBackgroundHistoryMedia(backgroundMediaKey, persisted.media);
  }
  return {
    revision,
    theme: theme.snapshot(),
    background: background.settings.value,
    backgroundMediaKey,
    document: captureStyleDocument(),
    createdAt: new Date().toISOString(),
  };
}

function readDocumentSnapshot(value: Json): StyleDocument | null {
  const documentValue = value.document;
  if (!isRecord(documentValue)) return null;
  return {
    version: 1,
    patch: isRecord(documentValue.patch) ? clone(documentValue.patch) : {},
    rules: Array.isArray(documentValue.rules) ? clone(documentValue.rules) as StyleRule[] : [],
    transactions: Array.isArray(documentValue.transactions) ? clone(documentValue.transactions) as StyleDocument["transactions"] : [],
    revision: Math.max(0, Number(documentValue.revision ?? value.revision ?? 0) || 0),
  };
}

async function restore(snapshotValue: Json) {
  const documentSnapshot = readDocumentSnapshot(snapshotValue);
  if (documentSnapshot) {
    replaceStyleDocument(documentSnapshot);
    applyStyleRuntime(documentSnapshot.patch, documentSnapshot.rules);
  }
  const theme = useTheme();
  const background = useBackgroundPreference();
  const themeValue = snapshotValue.theme;
  if (isRecord(themeValue)) {
    theme.replaceState(themeValue as Partial<ThemeState>);
  }
  const backgroundValue = snapshotValue.background;
  if (isRecord(backgroundValue)) {
    const mediaKey = String(snapshotValue.backgroundMediaKey ?? "");
    const media = mediaKey ? await readBackgroundHistoryMedia(mediaKey) : null;
    await background.replace(normalizeBackground(backgroundValue), media);
  }
}

function applyThemeChanges(changes: Json) {
  const theme = useTheme();
  const mode = changes.themeMode ?? changes.preset;
  if (typeof mode === "string" && ["system", "light", "dark"].includes(mode)) {
    theme.setPreset(mode as ThemePreset);
  }
  const accent = color(changes.accentColor);
  if (accent) {
    theme.setCustomPalette({
      enabled: true,
      primary: accent,
    });
  }
  if (typeof changes.fontScale === "number") theme.setFontScale(changes.fontScale);
  if (typeof changes.cornerStyle === "number") theme.setCornerStyle(changes.cornerStyle as CornerStyle);
  if (typeof changes.dynamicEffect === "boolean") theme.setDynamicEffect(changes.dynamicEffect);
  if (typeof changes.reduceAnimation === "boolean") theme.setReduceAnimation(changes.reduceAnimation);
  const custom = mapValue(changes, "customPalette");
  if (Object.keys(custom).length > 0) {
    theme.setCustomPalette({
      ...(typeof custom.enabled === "boolean" ? { enabled: custom.enabled } : {}),
      ...(color(custom.primary) ? { primary: color(custom.primary)! } : {}),
      ...(color(custom.secondary) ? { secondary: color(custom.secondary)! } : {}),
      ...(color(custom.text) ? { text: color(custom.text)! } : {}),
      ...(typeof custom.textMode === "string" ? { textMode: custom.textMode as any } : {}),
    });
  }
}

async function applyBackgroundChanges(changes: Json) {
  const background = useBackgroundPreference();
  await background.init();
  const raw = mapValue(changes, "background");
  if (Object.keys(raw).length === 0) return;
  const current = background.settings.value;
  const mode = String(raw.mode ?? current.mode);
  const name = typeof raw.name === "string" ? raw.name : current.name;
  if (mode === "default") {
    await background.reset();
    return;
  }
  if ((mode === "image" || mode === "video") && typeof raw.assetUri === "string" && raw.assetUri.trim()) {
    await background.applySource(raw.assetUri.trim(), mode, name);
  } else {
    const next = normalizeBackground({
      ...current,
      ...raw,
      ...(color(raw.color) ? { color: color(raw.color)! } : {}),
      ...(color(raw.gradientStart) ? { gradientStart: color(raw.gradientStart)! } : {}),
      ...(color(raw.gradientEnd) ? { gradientEnd: color(raw.gradientEnd)! } : {}),
      ...(color(raw.overlayColor) ? { overlayColor: color(raw.overlayColor)! } : {}),
      mode,
      kind: mode === "video" ? "video" : "image",
      enabled: typeof raw.enabled === "boolean" ? raw.enabled : mode !== "default",
      sourceUri: typeof raw.assetUri === "string" ? raw.assetUri : current.sourceUri,
    });
    await background.replace(next);
  }
  const post = mapValue(changes, "background");
  delete post.assetUri;
  delete post.mode;
  delete post.enabled;
  if (Object.keys(post).length > 0) {
    await background.update(post as Partial<BackgroundSettings>);
  }
}

function mergeStylePatch(changes: Json): StylePatch {
  const patch: StylePatch = { ...stylePatch.value, schemaVersion: 1 };
  for (const key of ["palette", "typography", "layout", "icons", "components", "componentVariants"]) {
    if (isRecord(changes[key])) {
      patch[key] = deepMerge(mapValue(patch, key), changes[key] as Json);
    }
  }
  return patch;
}

function ruleInputs(changes: Json): unknown[] {
  const values: unknown[] = [];
  if (Array.isArray(changes.targets)) values.push(...changes.targets);
  if (Array.isArray(changes.rules)) values.push(...changes.rules);
  if (changes.target || changes.selector || changes.component) values.push(changes);
  return values.map((value) => {
    if (!isRecord(value)) return value;
    return {
      ...value,
      target: value.target ?? value.selector ?? value.component,
      properties: value.properties ?? value.set ?? value.styles,
      scope: value.scope ?? {
        platform: value.platform,
        page: value.page,
        surface: value.surface,
      },
    };
  });
}

function readFilters(changes: Json): StyleRuleFilters {
  const rawScope = isRecord(changes.scope) ? changes.scope : changes;
  const scope: StyleScope = {};
  if (rawScope.platform === "desktop" || rawScope.platform === "mobile" || rawScope.platform === "web") {
    scope.platform = rawScope.platform;
  }
  if (typeof rawScope.page === "string" && rawScope.page) {
    scope.page = normalizeStylePageId(rawScope.page);
  }
  if (typeof rawScope.surface === "string" && rawScope.surface) scope.surface = rawScope.surface;
  const states = Array.isArray(changes.states)
    ? changes.states.map((item) => String(item))
    : typeof changes.state === "string"
      ? [changes.state]
      : undefined;
  const properties = Array.isArray(changes.properties)
    ? changes.properties.map((item) => String(item))
    : Array.isArray(changes.property)
      ? changes.property.map((item) => String(item))
      : undefined;
  const source = changes.source === "user" || changes.source === "ai"
    ? changes.source as StyleSource
    : undefined;
  return {
    scope: Object.keys(scope).length > 0 ? scope : undefined,
    target: typeof changes.target === "string"
      ? changes.target
      : typeof changes.selector === "string"
        ? changes.selector
        : undefined,
    states,
    properties,
    transactionId: typeof changes.transactionId === "string" ? changes.transactionId : undefined,
    source,
  };
}

function hasPartialReset(filters: StyleRuleFilters): boolean {
  return !!(
    filters.source ||
    filters.scope ||
    filters.target ||
    (filters.states && filters.states.length > 0) ||
    (filters.properties && filters.properties.length > 0) ||
    filters.transactionId ||
    (filters.ruleIds && filters.ruleIds.length > 0)
  );
}

export async function executeStyleCommand(
  action: string,
  expectedRevision: number,
  changes: Json,
): Promise<Record<string, unknown>> {
  const revision = styleRevision.value;
  if (expectedRevision > 0 && expectedRevision !== revision) {
    throw new Error(`style revision conflict: expected ${expectedRevision}, current ${revision}`);
  }
  const theme = useTheme();
  const background = useBackgroundPreference();
  await Promise.all([theme.loadFromServer(), background.init()]);
  if (action === "inspect") {
    const scope = isRecord(changes.scope) ? changes.scope : {
      platform: changes.platform,
      page: changes.page,
      surface: changes.surface,
    };
    const dom = inspectDomTargets({
      scope,
      target: typeof changes.target === "string" ? changes.target : undefined,
      limit: Number(changes.limit ?? 240) || 240,
      includeComputed: changes.includeComputed !== false,
    });
    return {
      ok: true,
      revision,
      page: dom.page,
      platform: dom.platform,
      patch: stylePatch.value,
      rules: listStyleRules(readFilters(changes)),
      theme: theme.snapshot(),
      background: background.settings.value,
      targets: dom.targets,
      count: dom.count,
    };
  }
  if (action === "apply") {
    const inputs = ruleInputs(changes);
    const hasThemeChanges = Object.keys(changes).some((key) => !["targets", "rules"].includes(key) && key !== "transactionId");
    if (inputs.length === 0 && !hasThemeChanges) throw new Error("style command is empty");
    const previous = await snapshot(revision);
    pushStyleHistory(previous);
    try {
      applyThemeChanges(changes);
      await applyBackgroundChanges(changes);
      const transactionId = String(changes.transactionId ?? `style_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`);
      const rules = addStyleRules(inputs.map((input) => {
        if (!isRecord(input)) return input;
        return { ...input, transactionId: input.transactionId ?? transactionId };
      }));
      const nextRevision = revision + 1;
      if (rules.length > 0) {
        recordStyleTransaction({
          id: transactionId,
          source: rules[0]?.source ?? "ai",
          ruleIds: rules.map((rule) => rule.id),
          changeKeys: Object.keys(changes),
          revision: nextRevision,
          createdAt: new Date().toISOString(),
        });
      }
      const patch = mergeStylePatch(changes);
      replaceStylePatch(patch, nextRevision);
      applyStyleRuntime(stylePatch.value, styleRules.value);
      return {
        ok: true,
        revision: nextRevision,
        previousRevision: revision,
        applied: Object.keys(changes),
        transactionId,
        ruleIds: rules.map((rule) => rule.id),
        ruleCount: rules.length,
      };
    } catch (error) {
      await restore(previous);
      throw error;
    }
  }
  if (action === "reset") {
    const filters = readFilters(changes);
    if (hasPartialReset(filters) && !filters.source) filters.source = "ai";
    const previous = await snapshot(revision);
    pushStyleHistory(previous);
    try {
      if (hasPartialReset(filters)) {
        const result = removeStyleRules(filters);
        const nextRevision = revision + 1;
        replaceStylePatch(stylePatch.value, nextRevision);
        applyStyleRuntime(stylePatch.value, styleRules.value);
        return {
          ok: true,
          revision: nextRevision,
          previousRevision: revision,
          reset: true,
          partial: true,
          removedRules: result.removedRules,
          changedRules: result.changedRules,
          removedProperties: result.removedProperties,
        };
      }
      theme.reset();
      await background.reset();
      removeStyleRules({});
      const nextRevision = revision + 1;
      clearStylePatch(nextRevision);
      applyStyleRuntime({}, []);
      return { ok: true, revision: nextRevision, previousRevision: revision, reset: true, partial: false };
    } catch (error) {
      await restore(previous);
      throw error;
    }
  }
  if (action === "undo") {
    const transactionId = String(changes.transactionId ?? "").trim();
    if (transactionId) {
      const previous = await snapshot(revision);
      pushStyleHistory(previous);
      const result = removeStyleRules({ transactionId });
      const nextRevision = revision + 1;
      replaceStylePatch(stylePatch.value, nextRevision);
      applyStyleRules(styleRules.value);
      return {
        ok: true,
        revision: nextRevision,
        previousRevision: revision,
        undo: true,
        transactionId,
        removedRules: result.removedRules,
        changedRules: result.changedRules,
      };
    }
    const transaction = popStyleTransaction();
    if (transaction) {
      const previous = await snapshot(revision);
      pushStyleHistory(previous);
      const result = removeStyleRules({ ruleIds: transaction.ruleIds });
      const nextRevision = revision + 1;
      replaceStylePatch(stylePatch.value, nextRevision);
      applyStyleRules(styleRules.value);
      return {
        ok: true,
        revision: nextRevision,
        previousRevision: revision,
        undo: true,
        transactionId: transaction.id,
        removedRules: result.removedRules,
        changedRules: result.changedRules,
      };
    }
    const value = popStyleHistory();
    if (!value) throw new Error("no style revision to undo");
    try {
      await restore(value);
      const nextRevision = revision + 1;
      replaceStyleDocument({
        ...captureStyleDocument(),
        revision: nextRevision,
      });
      return { ok: true, revision: nextRevision, previousRevision: revision, undo: true };
    } catch (error) {
      pushStyleHistory(value);
      throw error;
    }
  }
  throw new Error(`unsupported style action: ${action}`);
}
