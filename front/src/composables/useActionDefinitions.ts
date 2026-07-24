// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, computed, type ComputedRef } from "vue";
import { useApi } from "./useApi";

export interface ActionDefinition {
  id: string | number;
  key: string;
  name: string;
  description?: string;
  supportsDefaultIdle: boolean;
  recommended: boolean;
  defaultFrameCount?: number;
  estimatedGenerationCount: number;
  definitionVersion?: string;
}

export interface ActionCategory {
  key: string;
  name: string;
  sortOrder?: number;
  actions: ActionDefinition[];
}

export interface ActionPreset {
  key: string;
  name: string;
  description?: string;
  actionKeys: string[];
}

interface ActionDefinitionsPayload {
  categories: ActionCategory[];
  presets: ActionPreset[];
}

const sharedCategories = ref<ActionCategory[]>([]);
const sharedPresets = ref<ActionPreset[]>([]);
const sharedLoading = ref(false);
let loadPromise: Promise<void> | null = null;

export function useActionDefinitions() {
  const { get } = useApi();
  const selectedKeys = ref<string[]>([]);

  async function load(force = false): Promise<void> {
    if (loadPromise && !force) return loadPromise;
    sharedLoading.value = true;
    loadPromise = (async () => {
      try {
        const data = await get<ActionDefinitionsPayload>(
          "/api/desktop-pets/action-definitions",
        );
        sharedCategories.value = data?.categories || [];
        sharedPresets.value = data?.presets || [];
      } finally {
        sharedLoading.value = false;
      }
    })();
    return loadPromise;
  }

  function isSelected(key: string): boolean {
    return selectedKeys.value.includes(key);
  }

  function toggle(key: string): void {
    if (selectedKeys.value.includes(key)) {
      selectedKeys.value = selectedKeys.value.filter((k) => k !== key);
    } else {
      selectedKeys.value = [...selectedKeys.value, key];
    }
  }

  function selectOne(key: string): void {
    if (!selectedKeys.value.includes(key)) {
      selectedKeys.value = [...selectedKeys.value, key];
    }
  }

  function unselectOne(key: string): void {
    selectedKeys.value = selectedKeys.value.filter((k) => k !== key);
  }

  function categoryActions(categoryKey: string): ActionDefinition[] {
    const cat = sharedCategories.value.find((c) => c.key === categoryKey);
    return cat?.actions || [];
  }

  function isCategoryAllSelected(categoryKey: string): boolean {
    const actions = categoryActions(categoryKey);
    if (!actions.length) return false;
    return actions.every((a) => selectedKeys.value.includes(a.key));
  }

  function isCategoryPartialSelected(categoryKey: string): boolean {
    const actions = categoryActions(categoryKey);
    if (!actions.length) return false;
    return (
      actions.some((a) => selectedKeys.value.includes(a.key)) &&
      !actions.every((a) => selectedKeys.value.includes(a.key))
    );
  }

  function toggleCategory(categoryKey: string): void {
    if (isCategoryAllSelected(categoryKey)) {
      clearCategory(categoryKey);
    } else {
      selectAllCategory(categoryKey);
    }
  }

  function selectAllCategory(categoryKey: string): void {
    const keys = categoryActions(categoryKey).map((a) => a.key);
    const set = new Set(selectedKeys.value);
    keys.forEach((k) => set.add(k));
    selectedKeys.value = Array.from(set);
  }

  function clearCategory(categoryKey: string): void {
    const keys = new Set(categoryActions(categoryKey).map((a) => a.key));
    selectedKeys.value = selectedKeys.value.filter((k) => !keys.has(k));
  }

  function clearAll(): void {
    selectedKeys.value = [];
  }

  function applyPreset(target: ActionPreset | string | string[]): void {
    let keys: string[];
    if (Array.isArray(target)) {
      keys = target;
    } else if (typeof target === "string") {
      const found = sharedPresets.value.find((p) => p.key === target);
      keys = found ? [...found.actionKeys] : [];
    } else {
      keys = [...target.actionKeys];
    }
    selectedKeys.value = keys;
  }

  const selectedCount: ComputedRef<number> = computed(
    () => selectedKeys.value.length,
  );

  const estimatedGenerationCount: ComputedRef<number> = computed(() => {
    const set = new Set(selectedKeys.value);
    let total = 0;
    for (const cat of sharedCategories.value) {
      for (const action of cat.actions) {
        if (set.has(action.key)) {
          total += action.estimatedGenerationCount || 0;
        }
      }
    }
    return total;
  });

  const hasDefaultIdle: ComputedRef<boolean> = computed(() => {
    const set = new Set(selectedKeys.value);
    return sharedCategories.value.some((cat) =>
      cat.actions.some((a) => set.has(a.key) && a.supportsDefaultIdle),
    );
  });

  const allActionCount: ComputedRef<number> = computed(() => {
    let n = 0;
    for (const cat of sharedCategories.value) n += cat.actions.length;
    return n;
  });

  return {
    categories: sharedCategories,
    presets: sharedPresets,
    loading: sharedLoading,
    selectedKeys,
    load,
    isSelected,
    toggle,
    selectOne,
    unselectOne,
    toggleCategory,
    selectAllCategory,
    clearCategory,
    clearAll,
    applyPreset,
    isCategoryAllSelected,
    isCategoryPartialSelected,
    categoryActions,
    selectedCount,
    estimatedGenerationCount,
    hasDefaultIdle,
    allActionCount,
  };
}
