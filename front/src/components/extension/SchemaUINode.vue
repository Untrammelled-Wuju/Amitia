<script setup lang="ts">
import { computed, getCurrentInstance, inject, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import type { Router } from "vue-router";
import ExtensionSlot from "./ExtensionSlot.vue";
import SchemaUIImage from "./SchemaUIImage.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";
import {
  SCHEMA_UI_MAX_DEPTH,
  isAllowedNodeType,
  evaluateVisibility,
  resolveBinding,
  getFormBinding,
  lookupPath,
  setPath,
  markdownToHtml,
  toText,
  toNumber,
  toStringArray,
  toKeyValueItems,
  resolveActionInput,
  type SchemaUINode,
  type SchemaUIActionBinding,
  type SchemaUIBinding,
} from "./schema-ui-utils";

defineOptions({ name: "SchemaUINode" });

const props = defineProps<{
  node: SchemaUINode;
  depth: number;
  formState: Record<string, unknown>;
  context: Record<string, unknown>;
  sessionId: string;
  extensionId: string;
  contributionId: string;
}>();

const emit = defineEmits<{
  (e: "action", payload: { action: SchemaUIActionBinding; node: SchemaUINode; item?: Record<string, unknown>; files?: Array<Record<string, unknown>>; followUpActions?: SchemaUIActionBinding[] }): void;
  (e: "error", payload: { nodeId: string; message: string }): void;
}>();

const busyNodeIds = inject<Set<string> | undefined>("schema-ui-busy-node-ids", undefined);

const uiStore = useExtensionUIStore();
const router = getCurrentInstance()?.appContext.config.globalProperties.$router as Router | undefined;
const nodeType = computed(() => props.node.type);
const typeAllowed = computed(() => isAllowedNodeType(nodeType.value));
const depthExceeded = computed(() => props.depth > SCHEMA_UI_MAX_DEPTH);

const mergedProps = computed<Record<string, unknown>>(() => {
  const base: Record<string, unknown> = { ...(props.node.props ?? {}) };
  if (props.node.bindings) {
    for (const b of props.node.bindings) {
      const v = resolveBinding(b, props.formState, props.context);
      if (v !== undefined) base[b.path] = v;
    }
  }
  if (props.node.dataSource) {
    const v = resolveBinding(props.node.dataSource, props.formState, props.context);
    if (v !== undefined) base.__boundValue = v;
  }
  return base;
});

const semanticTarget = computed(() => {
  const explicit = toText(mergedProps.value.amitiaUi ?? mergedProps.value.amitia_ui ?? mergedProps.value.semanticTarget).trim();
  if (explicit) return explicit;
  switch (nodeType.value) {
    case "page":
      return "extension.page";
    case "card":
    case "section":
      return "extension.card";
    case "tabs":
    case "tab_item":
      return "component.tab";
    case "field":
    case "select":
      return "component.input";
    case "switch":
      return "component.switch";
    case "slider":
      return "component.slider";
    case "button":
    case "button_group":
      return "component.button";
    case "list":
      return "component.list_item";
    default:
      return "extension.surface";
  }
});

const nodeTarget = computed(() => {
  const contributionId = toText(props.contributionId).trim().toLowerCase().replace(/[^a-z0-9_-]+/g, "-") || "root";
  const nodeId = toText(props.node.id).trim().toLowerCase().replace(/[^a-z0-9_-]+/g, "-") || "root";
  return `plugin.schema.${contributionId}.${nodeId}`;
});

const visible = computed(() => {
  const cond = props.node.visibleWhen ?? props.node.visibility;
  return evaluateVisibility(cond, props.context);
});

const disabled = computed(() => {
  const conditions = props.node.disabledWhen;
  return Boolean(conditions?.length) && evaluateVisibility(conditions, props.context);
});

const children = computed<SchemaUINode[]>(() => props.node.children ?? []);

const formBinding = computed<SchemaUIBinding | undefined>(() => getFormBinding(props.node));
const editableFormBinding = computed<SchemaUIBinding | undefined>(() => {
  const binding = formBinding.value;
  if (!binding) return undefined;
  return binding.source === "form" || binding.source === "form_state" ? binding : undefined;
});

const modelValue = computed<{ value: unknown }>({
  get: () => {
    const b = formBinding.value;
    if (!b) return { value: mergedProps.value.__boundValue ?? mergedProps.value.value };
    if (b.source === "form_state" || b.source === "form") {
      return { value: lookupPath(props.formState, b.path) };
    }
    return { value: resolveBinding(b, props.formState, props.context) };
  },
  set: (next) => {
    const b = formBinding.value;
    if (b && (b.source === "form_state" || b.source === "form")) {
      setPath(props.formState, b.path, next.value);
    }
  },
});

function updateModelValue(v: unknown) {
  modelValue.value = { value: v };
}

const markdownHtml = computed(() =>
  markdownToHtml(toText(mergedProps.value.content ?? mergedProps.value.text ?? ""))
);

const gridTemplateColumns = computed(() => {
  if (mergedProps.value.layout === "master-detail") return undefined;
  const cols = toNumber(mergedProps.value.columns, 0);
  if (cols > 0 && cols <= 12) return `repeat(${cols}, minmax(0, 1fr))`;
  const tmpl = toText(mergedProps.value.columnsTemplate);
  if (tmpl) return tmpl;
  return "repeat(auto-fill, minmax(220px, 1fr))";
});

const tableColumns = computed<Array<{ prop: string; label: string; width?: number }>>(() => {
  const cols = mergedProps.value.columns;
  if (Array.isArray(cols)) {
    return cols.map((c) => {
      if (c && typeof c === "object") {
        const obj = c as Record<string, unknown>;
        return {
          prop: toText(obj.prop ?? obj.key ?? obj.field),
          label: toText(obj.label ?? obj.title ?? obj.prop ?? obj.key),
          width: obj.width == null ? undefined : toNumber(obj.width),
        };
      }
      return { prop: toText(c), label: toText(c) };
    });
  }
  const data = Array.isArray(mergedProps.value.data) ? mergedProps.value.data : [];
  const keySet = new Set<string>();
  for (const row of data) {
    if (row && typeof row === "object") {
      for (const k of Object.keys(row as Record<string, unknown>)) keySet.add(k);
    }
  }
  return Array.from(keySet).map((k) => ({ prop: k, label: k }));
});

const tableData = computed<Record<string, unknown>[]>(() => {
  const d = mergedProps.value.data ?? mergedProps.value.items ?? [];
  return Array.isArray(d) ? (d as Record<string, unknown>[]) : [];
});

const listItems = computed<string[]>(() => toStringArray(mergedProps.value.items));

const kvItems = computed(() => toKeyValueItems(mergedProps.value.items ?? mergedProps.value.data));

const extensionSlotAuthorized = computed(() => {
  if (nodeType.value !== "extension_slot") return true;
  browserClientPluginRuntime.slots.revision.value;
  const childSlotId = toText(mergedProps.value.slotId).trim();
  const parentSlotId = toText(props.context.slotId).trim();
  if (!childSlotId || !parentSlotId) return false;
  const serverDefinition = uiStore.slotsById.get(childSlotId);
  const clientDefinition = browserClientPluginRuntime.slots.getDefinition(childSlotId);
  const expectedOwner = toText(props.context.clientRuntimePluginId).trim() || props.extensionId;
  if (clientDefinition?.parentSlotId === parentSlotId && clientDefinition.ownerId === expectedOwner) return true;
  return serverDefinition?.parentSlotId === parentSlotId && serverDefinition.ownerExtension === props.extensionId;
});

const extensionSlotFallback = computed(() => {
  const value = toText(mergedProps.value.fallback);
  return ["none", "skeleton", "empty", "default"].includes(value)
    ? value as "none" | "skeleton" | "empty" | "default"
    : undefined;
});

const extensionSlotLayout = computed(() => {
  const value = toText(mergedProps.value.layout);
  return ["inline", "stack", "row", "grid", "tabs", "panel", "drawer", "modal"].includes(value)
    ? value as "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal"
    : undefined;
});

const extensionSlotSurfaceRole = computed(() => {
  const value = toText(mergedProps.value.surfaceRole) || "main";
  return ["header", "status", "sidebar", "message", "composer", "main", "overlay"].includes(value)
    ? value as "header" | "status" | "sidebar" | "message" | "composer" | "main" | "overlay"
    : "main";
});

const permissionList = computed<string[]>(() => {
  const perms = mergedProps.value.permissions ?? mergedProps.value.items;
  return toStringArray(perms);
});

const runtimeStatusInfo = computed(() => ({
  status: toText(mergedProps.value.status ?? mergedProps.value.state ?? "unknown"),
  message: toText(mergedProps.value.message ?? mergedProps.value.detail ?? ""),
  label: toText(mergedProps.value.label ?? "运行时状态"),
}));

const activeTabRaw = ref<string>("");

const tabPanes = computed<SchemaUINode[]>(() =>
  children.value.filter((c) => c.type === "tab_item")
);

const defaultTabName = computed(() => {
  const first = tabPanes.value[0];
  if (!first) return "";
  return toText(first.props?.name ?? first.id);
});

const activeTab = computed<string>({
  get: () => activeTabRaw.value || defaultTabName.value,
  set: (v: string) => {
    activeTabRaw.value = v;
  },
});

const activeTabQueryParam = computed(() => toText(mergedProps.value.queryParam));
const activeTabChildren = computed(() => tabPanes.value.find((pane) => toText(pane.props?.name ?? pane.id) === activeTab.value)?.children ?? []);

watch(
  [activeTabQueryParam, () => props.context.routeQuery, tabPanes],
  () => {
    const param = activeTabQueryParam.value;
    if (!param) return;
    const query = props.context.routeQuery;
    if (!query || typeof query !== "object" || Array.isArray(query)) return;
    const requested = toText((query as Record<string, unknown>)[param]);
    if (!requested) {
      activeTabRaw.value = "";
      return;
    }
    const exists = tabPanes.value.some((pane) => toText(pane.props?.name ?? pane.id) === requested);
    if (exists) activeTabRaw.value = requested;
  },
  { deep: true, immediate: true },
);

const tabsType = computed<"" | "card" | "border-card">(() => {
  const variant = toText(mergedProps.value.variant).trim();
  if (variant === "card" || variant === "border-card") return variant;
  return "";
});

const selectOptions = computed<Array<{ label: string; value: unknown }>>(() => {
  const opts = mergedProps.value.options;
  if (Array.isArray(opts)) {
    return opts.map((o) => {
      if (o && typeof o === "object") {
        const obj = o as Record<string, unknown>;
        return {
          label: toText(obj.label ?? obj.text ?? obj.displayName ?? obj.name ?? obj.value ?? obj.id),
          value: obj.value ?? obj.id,
        };
      }
      return { label: toText(o), value: o };
    });
  }
  return [];
});

const galleryItems = computed<Array<Record<string, unknown>>>(() => {
  const rows = mergedProps.value.items;
  if (!Array.isArray(rows)) return [];
  const query = toText(mergedProps.value.query).trim().toLocaleLowerCase();
  const fields = toStringArray(mergedProps.value.searchFields);
  const filterField = toText(mergedProps.value.filterField);
  const filterValue = mergedProps.value.filterValue;
  return rows.filter((item): item is Record<string, unknown> => {
    if (!item || typeof item !== "object") return false;
    const row = item as Record<string, unknown>;
    if (filterField && filterValue) {
      const value = row[filterField];
      const matches = Array.isArray(value) ? value.includes(filterValue) : value === filterValue;
      if (mergedProps.value.excludeFilter === true ? matches : !matches) return false;
    }
    return !query || (fields.length ? fields : ["name", "meaning", "keywords"])
      .some((field) => toText(Array.isArray(row[field]) ? (row[field] as unknown[]).join(" ") : row[field]).toLocaleLowerCase().includes(query));
  });
});

function galleryItemPlaceholder(item: Record<string, unknown>) {
  if (mergedProps.value.variant === "groups") {
    return toText(item.name ?? item.label ?? item.id).slice(0, 1) || "组";
  }
  return mergedProps.value.placeholder ?? "无预览";
}

const resourceLinkActive = computed(() => {
  const href = toText(mergedProps.value.href).trim();
  const route = toText(props.context.route).trim();
  const defaultRoute = toText(props.context.defaultNavigationRoute).trim();
  const current = route && !/[?#]/.test(route) && defaultRoute.split(/[?#]/, 1)[0] === route
    ? defaultRoute
    : route;
  if (!href || !current) return false;
  if (href.includes("?") || href.includes("#")) return current === href;
  return current.split(/[?#]/, 1)[0] === href;
});

const filePickerInput = ref<HTMLInputElement>();

function onButtonClick() {
  if (busyNodeIds?.has(props.node.id)) return;
  if (!props.node.actions || props.node.actions.length === 0) return;
  const required = toStringArray(mergedProps.value.requiredFields);
  if (required.some((field) => !toText(lookupPath(props.formState, field)).trim())) {
    ElMessage.warning(toText(mergedProps.value.validationMessage) || "请先填写必填内容");
    return;
  }
  const [action, ...followUpActions] = props.node.actions;
  emit("action", { action, node: props.node, followUpActions });
}

function onResourceLinkClick(event: MouseEvent) {
  const href = toText(mergedProps.value.href).trim();
  const target = toText(mergedProps.value.target) || "_blank";
  if (
    !router
    || target !== "_self"
    || !href.startsWith("/")
    || href.startsWith("//")
    || event.defaultPrevented
    || event.button !== 0
    || event.metaKey
    || event.ctrlKey
    || event.shiftKey
    || event.altKey
  ) return;
  event.preventDefault();
  void router.push(href);
}

function onGalleryItemClick(item: Record<string, unknown>) {
  if (!props.node.actions?.length) return;
  for (const action of props.node.actions) {
    emit("action", {
      action,
      node: props.node,
      item: {
        ...item,
        input: resolveActionInput(action.input, props.formState, props.context, item),
      },
    });
  }
}

function openFilePicker() {
  filePickerInput.value?.click();
}

const filePickerBusy = computed(() => busyNodeIds?.has(props.node.id) === true);

async function onFilePicked(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = "";
  if (files.length === 0) return;
  const payload = await Promise.all(files.map(async (file) => ({
    name: file.name,
    mimeType: file.type,
    size: file.size,
    dataBase64: await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        const value = typeof reader.result === "string" ? reader.result : "";
        resolve(value.includes(",") ? value.slice(value.indexOf(",") + 1) : value);
      };
      reader.onerror = () => reject(reader.error ?? new Error("文件读取失败"));
      reader.readAsDataURL(file);
    }),
  })));
  const actions = props.node.actions ?? [];
  if (actions.length === 0) return;
  const [primary, ...followUpActions] = actions;
  emit("action", {
    action: primary,
    node: props.node,
    files: payload,
    item: {
      input: resolveActionInput(primary.input, props.formState, props.context, { files: payload }),
      files: payload,
    },
    followUpActions,
  });
}

function onActionFromChild(payload: { action: SchemaUIActionBinding; node: SchemaUINode; item?: Record<string, unknown>; files?: Array<Record<string, unknown>> }) {
  emit("action", payload);
}
</script>

<template>
  <div
    v-if="depthExceeded"
    class="schema-ui-node__depth-exceeded"
    :data-node-id="node.id"
  >
    节点 {{ node.id }} 递归深度超限 ({{ depth }}/{{ SCHEMA_UI_MAX_DEPTH }})
  </div>
  <div
    v-else-if="!typeAllowed"
    class="schema-ui-node__unknown"
    :data-node-id="node.id"
  >
    未知或禁止的节点类型: {{ nodeType }}
  </div>
  <template v-else-if="visible">
    <div
      v-if="nodeType === 'page'"
      class="schema-ui-page schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>

    <el-dialog
      v-else-if="nodeType === 'section' && mergedProps.presentation === 'dialog'"
      :model-value="modelValue.value === true"
      :title="toText(mergedProps.title)"
      :width="toNumber(mergedProps.width, 440)"
      :close-on-click-modal="false"
      align-center
      destroy-on-close
      :data-node-id="node.id"
      @update:model-value="updateModelValue"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </el-dialog>

    <el-card
      v-else-if="nodeType === 'section'"
      class="schema-ui-section schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :shadow="mergedProps.shadow ?? 'never'"
      :border="mergedProps.bordered !== false"
    >
      <template v-if="mergedProps.title" #header>
        <div class="schema-ui-section__header">
          <span class="schema-ui-section__title">{{ mergedProps.title }}</span>
          <span v-if="mergedProps.subtitle" class="schema-ui-section__subtitle">{{ mergedProps.subtitle }}</span>
        </div>
      </template>
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </el-card>

    <div
      v-else-if="nodeType === 'stack'"
      class="schema-ui-stack schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :style="{ gap: mergedProps.gap ? toText(mergedProps.gap) : undefined, alignItems: toText(mergedProps.align) || undefined, maxWidth: mergedProps.maxWidth ? `${toNumber(mergedProps.maxWidth)}px` : undefined }"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>

    <div
      v-else-if="nodeType === 'row'"
      class="schema-ui-row schema-ui-node"
      :class="{ 'schema-ui-row--toolbar': mergedProps.layout === 'toolbar' }"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :style="{ gap: mergedProps.gap ? toText(mergedProps.gap) : undefined, alignItems: toText(mergedProps.align) || undefined, justifyContent: toText(mergedProps.justify) || undefined }"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>

    <div
      v-else-if="nodeType === 'grid'"
      class="schema-ui-grid schema-ui-node"
      :class="{ 'schema-ui-grid--master-detail': mergedProps.layout === 'master-detail' }"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :style="{ gridTemplateColumns, gap: mergedProps.gap ? toText(mergedProps.gap) : undefined }"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>

    <div
      v-else-if="nodeType === 'tabs' && mergedProps.navigation === 'external'"
      class="schema-ui-stack schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <SchemaUINode
        v-for="child in activeTabChildren"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>

    <el-tabs
      v-else-if="nodeType === 'tabs'"
      v-model="activeTab"
      class="schema-ui-tabs schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :type="tabsType"
      :tab-position="(mergedProps.position as any) || 'top'"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </el-tabs>

    <el-card
      v-else-if="nodeType === 'card'"
      class="schema-ui-card schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :shadow="(mergedProps.shadow as any) ?? 'hover'"
      :border="mergedProps.bordered !== false"
    >
      <template v-if="mergedProps.title" #header>
        <span>{{ mergedProps.title }}</span>
      </template>
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </el-card>

    <component
      :is="mergedProps.variant === 'title' ? 'h3' : 'p'"
      v-else-if="nodeType === 'text'"
      class="schema-ui-text schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      {{ mergedProps.text ?? mergedProps.content ?? toText(mergedProps.value) }}
    </component>

    <div
      v-else-if="nodeType === 'markdown'"
      class="schema-ui-markdown schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      v-html="markdownHtml"
    ></div>

    <el-tag
      v-else-if="nodeType === 'badge'"
      class="schema-ui-badge schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :type="(mergedProps.type as any) || 'info'"
      :effect="(mergedProps.effect as any) || 'light'"
      :closable="mergedProps.closable === true"
      size="small"
    >
      {{ mergedProps.text ?? mergedProps.label ?? mergedProps.value }}
    </el-tag>

    <el-divider
      v-else-if="nodeType === 'divider'"
      class="schema-ui-divider schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :direction="(mergedProps.direction as any) || 'horizontal'"
      :content-position="(mergedProps.position as any) || 'center'"
    >
      <template v-if="mergedProps.text">{{ mergedProps.text }}</template>
    </el-divider>

    <el-icon
      v-else-if="nodeType === 'icon'"
      class="schema-ui-icon schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :size="toNumber(mergedProps.size, 16)"
      :color="toText(mergedProps.color) || undefined"
    >
      <span class="schema-ui-icon__symbol">{{ mergedProps.symbol ?? mergedProps.name ?? mergedProps.label }}</span>
    </el-icon>

    <SchemaUIImage
      v-else-if="nodeType === 'image'"
      class="schema-ui-image schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :src="toText(mergedProps.src)"
      :fit="(mergedProps.fit as any) || 'cover'"
      :alt="toText(mergedProps.alt)"
      :style="{ width: mergedProps.width ? `${Math.max(1, toNumber(mergedProps.width))}px` : undefined, height: mergedProps.height ? `${Math.max(1, toNumber(mergedProps.height))}px` : undefined }"
      :preview="mergedProps.preview === true"
    />

    <el-form-item
      v-else-if="nodeType === 'field'"
      class="schema-ui-field schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :label="toText(mergedProps.label ?? mergedProps.title)"
      :required="mergedProps.required === true"
      :prop="toText(mergedProps.prop || formBinding?.path)"
      :error="toText(mergedProps.error)"
    >
      <template v-if="children.length > 0">
        <SchemaUINode
          v-for="child in children"
          :key="child.id"
          :node="child"
          :depth="depth + 1"
          :form-state="formState"
          :context="context"
          :session-id="sessionId"
          :extension-id="extensionId"
          :contribution-id="contributionId"
          @action="onActionFromChild"
          @error="(p) => emit('error', p)"
        />
      </template>
      <el-input
        v-else
        class="schema-ui-input"
        :model-value="modelValue.value"
        :placeholder="toText(mergedProps.placeholder)"
        :type="(mergedProps.variant as any) || 'text'"
        :disabled="disabled || mergedProps.disabled === true || !editableFormBinding"
        :clearable="mergedProps.clearable === true"
        :maxlength="mergedProps.maxlength ? toNumber(mergedProps.maxlength) : undefined"
        :show-word-limit="mergedProps.showWordLimit === true"
        :rows="mergedProps.rows ? toNumber(mergedProps.rows) : undefined"
        @update:model-value="updateModelValue"
      />
    </el-form-item>

    <el-select
      v-else-if="nodeType === 'select'"
      class="schema-ui-select schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :model-value="modelValue.value"
      :placeholder="toText(mergedProps.placeholder)"
      :disabled="disabled || mergedProps.disabled === true || !editableFormBinding"
      :multiple="mergedProps.multiple === true"
      :clearable="mergedProps.clearable === true"
      :filterable="mergedProps.filterable === true"
      @update:model-value="updateModelValue"
    >
      <el-option
        v-for="(opt, idx) in selectOptions"
        :key="idx"
        :label="opt.label"
        :value="opt.value"
      />
    </el-select>

    <el-switch
      v-else-if="nodeType === 'switch'"
      class="schema-ui-switch schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :model-value="!!modelValue.value"
      :disabled="disabled || mergedProps.disabled === true || !editableFormBinding"
      :active-text="toText(mergedProps.activeText)"
      :inactive-text="toText(mergedProps.inactiveText)"
      :active-value="mergedProps.activeValue ?? true"
      :inactive-value="mergedProps.inactiveValue ?? false"
      @update:model-value="updateModelValue"
    />

    <div
      v-else-if="nodeType === 'slider'"
      class="schema-ui-slider schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <span v-if="mergedProps.label" class="schema-ui-slider__label">{{ mergedProps.label }}</span>
      <el-slider
        :aria-label="toText(mergedProps.label) || undefined"
        :model-value="toNumber(modelValue.value)"
        :min="toNumber(mergedProps.min, 0)"
        :max="toNumber(mergedProps.max, 100)"
        :step="toNumber(mergedProps.step, 1)"
        :disabled="disabled || mergedProps.disabled === true || !editableFormBinding"
        :show-input="mergedProps.showInput === true"
        @update:model-value="updateModelValue"
      />
    </div>

    <el-button
      v-else-if="nodeType === 'button'"
      class="schema-ui-button schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :type="(mergedProps.type as any) || 'default'"
      :size="(mergedProps.size as any) || 'default'"
      :disabled="disabled || mergedProps.disabled === true"
      :loading="mergedProps.loading === true || filePickerBusy"
      :plain="mergedProps.plain === true"
      :round="mergedProps.round === true"
      @click="onButtonClick"
    >
      {{ mergedProps.text ?? mergedProps.label ?? "按钮" }}
    </el-button>

    <el-button-group
      v-else-if="nodeType === 'button_group'"
      class="schema-ui-button-group schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </el-button-group>

    <ul
      v-else-if="nodeType === 'list'"
      class="schema-ui-list schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <template v-if="listItems.length > 0">
        <li v-for="(item, idx) in listItems" :key="idx" class="schema-ui-list__item">{{ item }}</li>
      </template>
      <template v-else>
        <li v-for="child in children" :key="child.id" class="schema-ui-list__item">
          <SchemaUINode
            :node="child"
            :depth="depth + 1"
            :form-state="formState"
            :context="context"
            :session-id="sessionId"
            :extension-id="extensionId"
            :contribution-id="contributionId"
            @action="onActionFromChild"
            @error="(p) => emit('error', p)"
          />
        </li>
      </template>
    </ul>

    <div
      v-else-if="nodeType === 'gallery'"
      class="schema-ui-gallery schema-ui-node"
      :class="{
        'schema-ui-gallery--assets': mergedProps.variant === 'assets',
        'schema-ui-gallery--collections': mergedProps.variant === 'collections',
        'schema-ui-gallery--groups': mergedProps.variant === 'groups',
        'schema-ui-gallery--stickers': mergedProps.variant === 'stickers',
      }"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :style="{ gridTemplateColumns: `repeat(auto-fill, minmax(min(100%, ${toNumber(mergedProps.minItemWidth, 148)}px), 1fr))` }"
    >
      <div v-if="mergedProps.showCount === true" class="schema-ui-gallery__count">{{ galleryItems.length }} 张图片</div>
      <button
        v-for="(item, index) in galleryItems"
        :key="toText(item.id ?? item.value ?? index)"
        type="button"
        class="schema-ui-gallery__item"
        :class="{ 'is-selected': mergedProps.selectedId === item.id }"
        :disabled="disabled || mergedProps.disabled === true"
        @click="onGalleryItemClick(item)"
      >
        <div v-if="mergedProps.variant === 'collections'" class="schema-ui-gallery__cover">
          <template v-for="index in 4" :key="index">
            <SchemaUIImage v-if="Array.isArray(item.covers) && item.covers[index - 1]" :src="toText(item.covers[index - 1])" alt="" />
            <span v-else class="schema-ui-gallery__cover-empty"></span>
          </template>
        </div>
        <SchemaUIImage
          v-else-if="toText(item[toText(mergedProps.imageField) || 'image'] ?? item.thumbnailUrl ?? item.assetUrl)"
          class="schema-ui-gallery__image"
          :src="toText(item[toText(mergedProps.imageField) || 'image'] ?? item.thumbnailUrl ?? item.assetUrl)"
          :alt="toText(item[toText(mergedProps.titleField) || 'title'] ?? item.name ?? item.label)"
          loading="lazy"
        />
        <div v-else class="schema-ui-gallery__placeholder">{{ galleryItemPlaceholder(item) }}</div>
        <span v-if="mergedProps.showTitle !== false" class="schema-ui-gallery__title">
          {{ item[toText(mergedProps.titleField) || "title"] ?? item.name ?? item.label ?? item.id }}
        </span>
        <span v-if="mergedProps.showSubtitle !== false && (item[toText(mergedProps.subtitleField) || 'subtitle'] ?? item.meaning ?? item.description)" class="schema-ui-gallery__subtitle">
          {{ item[toText(mergedProps.subtitleField) || "subtitle"] ?? item.meaning ?? item.description }}
        </span>
      </button>
      <div v-if="galleryItems.length === 0" class="schema-ui-gallery__empty">
        {{ mergedProps.emptyText ?? "暂无数据" }}
      </div>
    </div>

    <div
      v-else-if="nodeType === 'file_picker'"
      class="schema-ui-file-picker schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <input
        ref="filePickerInput"
        class="schema-ui-file-picker__input"
        type="file"
        :accept="toText(mergedProps.accept) || undefined"
        :multiple="mergedProps.multiple !== false"
        @change="onFilePicked"
      />
      <el-button
        class="schema-ui-file-picker__button"
        :type="(mergedProps.type as any) || 'primary'"
        :loading="filePickerBusy"
        :disabled="disabled || mergedProps.disabled === true || filePickerBusy"
        @click="openFilePicker"
      >
        {{ mergedProps.text ?? mergedProps.label ?? "选择文件" }}
      </el-button>
      <span v-if="mergedProps.hint" class="schema-ui-file-picker__hint">{{ mergedProps.hint }}</span>
    </div>

    <el-table
      v-else-if="nodeType === 'table'"
      class="schema-ui-table schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :data="tableData"
      :border="mergedProps.bordered === true"
      :stripe="mergedProps.stripe !== false"
      size="small"
      :max-height="mergedProps.maxHeight ? toNumber(mergedProps.maxHeight) : undefined"
    >
      <el-table-column
        v-for="(col, idx) in tableColumns"
        :key="idx"
        :prop="col.prop"
        :label="col.label"
        :width="col.width"
        show-overflow-tooltip
      />
    </el-table>

    <el-empty
      v-else-if="nodeType === 'empty_state'"
      class="schema-ui-empty schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :description="toText(mergedProps.description ?? mergedProps.text ?? '暂无数据')"
      :image-size="toNumber(mergedProps.imageSize, 60)"
    />

    <el-alert
      v-else-if="nodeType === 'alert'"
      class="schema-ui-alert schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :title="toText(mergedProps.title ?? mergedProps.message)"
      :description="toText(mergedProps.description ?? mergedProps.detail)"
      :type="(mergedProps.type as any) || 'info'"
      :closable="mergedProps.closable !== false"
      :show-icon="mergedProps.showIcon !== false"
      :effect="(mergedProps.effect as any) || 'light'"
    />

    <el-progress
      v-else-if="nodeType === 'progress'"
      class="schema-ui-progress schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :percentage="Math.max(0, Math.min(100, toNumber(mergedProps.value ?? mergedProps.percentage, 0)))"
      :type="(mergedProps.variant as any) || 'line'"
      :status="(mergedProps.status as any) || undefined"
      :stroke-width="toNumber(mergedProps.strokeWidth, 6)"
      :show-text="mergedProps.showText !== false"
    />

    <pre
      v-else-if="nodeType === 'code'"
      class="schema-ui-code schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    ><code>{{ mergedProps.content ?? mergedProps.text ?? mergedProps.value }}</code></pre>

    <el-descriptions
      v-else-if="nodeType === 'key_value'"
      class="schema-ui-key-value schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :column="toNumber(mergedProps.columns, 1)"
      :border="mergedProps.bordered !== false"
      :title="toText(mergedProps.title) || undefined"
      size="small"
    >
      <el-descriptions-item
        v-for="(item, idx) in kvItems"
        :key="idx"
        :label="item.key"
      >{{ item.value }}</el-descriptions-item>
    </el-descriptions>

    <el-link
      v-else-if="nodeType === 'resource_link'"
      class="schema-ui-resource-link schema-ui-node"
      :class="{
        'schema-ui-resource-link--navigation': mergedProps.variant === 'navigation',
        'is-active': mergedProps.variant === 'navigation' && resourceLinkActive,
      }"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :href="toText(mergedProps.href)"
      :target="toText(mergedProps.target) || '_blank'"
      :type="(mergedProps.type as any) || 'primary'"
      :underline="mergedProps.underline !== false"
      :aria-current="mergedProps.variant === 'navigation' && resourceLinkActive ? 'page' : undefined"
      :disabled="disabled || mergedProps.disabled === true"
      @click="onResourceLinkClick"
    >
      {{ mergedProps.text ?? mergedProps.label ?? mergedProps.href }}
    </el-link>

    <div
      v-else-if="nodeType === 'permission_summary'"
      class="schema-ui-permission-summary schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <div class="schema-ui-permission-summary__title">
        {{ mergedProps.title ?? "权限概览" }}
      </div>
      <ul v-if="permissionList.length > 0" class="schema-ui-permission-summary__list">
        <li v-for="(p, idx) in permissionList" :key="idx" class="schema-ui-permission-summary__item">
          <span class="schema-ui-permission-summary__dot"></span>{{ p }}
        </li>
      </ul>
      <div v-else class="schema-ui-permission-summary__empty">无权限声明</div>
    </div>

    <div
      v-else-if="nodeType === 'runtime_status'"
      class="schema-ui-runtime-status schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :data-status="runtimeStatusInfo.status"
    >
      <div class="schema-ui-runtime-status__row">
        <span class="schema-ui-runtime-status__label">{{ runtimeStatusInfo.label }}</span>
        <span class="schema-ui-runtime-status__badge" :data-status="runtimeStatusInfo.status">
          {{ runtimeStatusInfo.status }}
        </span>
      </div>
      <p v-if="runtimeStatusInfo.message" class="schema-ui-runtime-status__message">
        {{ runtimeStatusInfo.message }}
      </p>
    </div>

    <ExtensionSlot
      v-else-if="nodeType === 'extension_slot' && extensionSlotAuthorized"
      class="schema-ui-extension-slot schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :slot-id="toText(mergedProps.slotId)"
      :context="context"
      :fallback="extensionSlotFallback"
      :layout="extensionSlotLayout"
      :surface-role="extensionSlotSurfaceRole"
    />

    <template v-else-if="nodeType === 'extension_slot'"></template>

    <el-tab-pane
      v-else-if="nodeType === 'tab_item'"
      class="schema-ui-tab-item schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
      :label="toText(mergedProps.label ?? mergedProps.title)"
      :name="toText(mergedProps.name ?? node.id)"
      :disabled="disabled"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </el-tab-pane>

    <div
      v-else-if="nodeType === 'column'"
      class="schema-ui-column schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <span v-if="mergedProps.label" class="schema-ui-column__label">{{ mergedProps.label }}</span>
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>

    <div
      v-else
      class="schema-ui-node__fallback schema-ui-node"
      :data-amitia-ui="semanticTarget"
      :data-amitia-node-target="nodeTarget"
      :data-node-id="node.id"
    >
      <SchemaUINode
        v-for="child in children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :form-state="formState"
        :context="context"
        :session-id="sessionId"
        :extension-id="extensionId"
        :contribution-id="contributionId"
        @action="onActionFromChild"
        @error="(p) => emit('error', p)"
      />
    </div>
  </template>
</template>

<style scoped>
.schema-ui-node {
  box-sizing: border-box;
  width: 100%;
  min-width: 0;
}
.schema-ui-page {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 4px 0;
  container-type: inline-size;
}
.schema-ui-section__header {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.schema-ui-section__title {
  font-weight: 600;
  font-size: 14px;
  color: var(--amitia-color-text, inherit);
}
.schema-ui-section__subtitle {
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.8));
}
.schema-ui-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.schema-ui-row {
  display: flex;
  flex-direction: row;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}
.schema-ui-row--toolbar > .schema-ui-node {
  width: auto;
  flex: 0 0 auto;
}
.schema-ui-row--toolbar > .schema-ui-field {
  flex: 1 1 200px;
  max-width: 420px;
  margin-bottom: 0;
}
.schema-ui-row--toolbar > .schema-ui-text {
  color: var(--text-secondary);
}
.schema-ui-row--toolbar > h3.schema-ui-text {
  color: var(--text-primary);
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
}
.schema-ui-grid {
  display: grid;
  gap: 12px;
}
.schema-ui-grid--master-detail {
  grid-template-columns: minmax(0, 1fr);
  align-items: start;
}
.schema-ui-grid--master-detail:has(> :nth-child(2)) {
  grid-template-columns: minmax(0, 1fr) minmax(260px, 320px);
}
.schema-ui-row > .schema-ui-file-picker {
  width: auto;
  flex: 0 1 auto;
}
.schema-ui-stack > .schema-ui-field {
  margin-bottom: 0;
}
@container (max-width: 760px) {
  .schema-ui-grid--master-detail:has(> :nth-child(2)) {
    grid-template-columns: minmax(0, 1fr);
  }
}
.schema-ui-text {
  margin: 0;
  font-size: 13px;
  color: var(--amitia-color-text, inherit);
  line-height: 1.6;
}
.schema-ui-resource-link--navigation {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 100%;
  min-height: 38px;
  padding: 8px 12px;
  border-radius: 9px;
  color: var(--amitia-color-text-secondary, var(--text-secondary, rgba(127, 127, 127, 0.9)));
  font-size: 13px;
  font-weight: 560;
  line-height: 1.35;
  text-decoration: none;
}
.schema-ui-resource-link--navigation:hover,
.schema-ui-resource-link--navigation.is-active {
  background: var(--workbench-sidebar-hover, rgba(127, 127, 127, 0.1));
  color: var(--amitia-color-text, var(--text-primary, inherit));
  text-decoration: none;
}
.schema-ui-markdown {
  font-size: 13px;
  line-height: 1.7;
  color: var(--amitia-color-text, inherit);
  word-break: break-word;
}
.schema-ui-markdown :deep(h1),
.schema-ui-markdown :deep(h2),
.schema-ui-markdown :deep(h3),
.schema-ui-markdown :deep(h4),
.schema-ui-markdown :deep(h5),
.schema-ui-markdown :deep(h6) {
  margin: 8px 0 4px;
  font-weight: 600;
  color: var(--amitia-color-text, inherit);
}
.schema-ui-markdown :deep(p) {
  margin: 4px 0;
}
.schema-ui-markdown :deep(ul),
.schema-ui-markdown :deep(ol) {
  margin: 4px 0;
  padding-left: 20px;
}
.schema-ui-markdown :deep(code) {
  padding: 1px 4px;
  border-radius: 4px;
  background: var(--amitia-color-surface-elevated, rgba(127, 127, 127, 0.12));
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}
.schema-ui-markdown :deep(pre) {
  margin: 6px 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--amitia-color-surface-elevated, rgba(127, 127, 127, 0.12));
  overflow-x: auto;
}
.schema-ui-markdown :deep(pre code) {
  padding: 0;
  background: transparent;
}
.schema-ui-markdown :deep(blockquote) {
  margin: 6px 0;
  padding: 4px 10px;
  border-left: 3px solid var(--amitia-color-border, rgba(127, 127, 127, 0.3));
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.85));
}
.schema-ui-markdown :deep(a) {
  color: var(--amitia-color-accent, var(--tp-primary));
  text-decoration: none;
}
.schema-ui-markdown :deep(a:hover) {
  text-decoration: underline;
}
.schema-ui-markdown :deep(hr) {
  border: none;
  border-top: 1px solid var(--amitia-color-border, rgba(127, 127, 127, 0.2));
  margin: 8px 0;
}
.schema-ui-icon__symbol {
  font-size: inherit;
  line-height: 1;
}
.schema-ui-image {
  border-radius: 6px;
  overflow: hidden;
}
.schema-ui-image__error,
.schema-ui-image__placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 80px;
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.7));
  background: var(--amitia-color-surface-elevated, rgba(127, 127, 127, 0.08));
}
.schema-ui-button {
  width: auto;
  align-self: flex-start;
  flex: 0 0 auto;
  max-width: 100%;
}
.schema-ui-slider {
  display: flex;
  flex-direction: column;
  gap: 8px;
  container-type: inline-size;
}
.schema-ui-slider :deep(.el-slider) {
  box-sizing: border-box;
  padding-inline: calc(var(--el-slider-button-wrapper-size, 36px) / 2 + 2px);
}
.schema-ui-slider :deep(.el-slider__runway) {
  min-width: 0;
}
@container (max-width: 280px) {
  .schema-ui-slider :deep(.el-slider) {
    height: auto;
    flex-wrap: wrap;
    row-gap: calc(var(--el-slider-button-wrapper-size, 36px) / 2 + 4px);
    padding-block: calc((var(--el-slider-button-wrapper-size, 36px) - var(--el-slider-height, 6px)) / 2);
  }
  .schema-ui-slider :deep(.el-slider__runway.show-input) {
    flex-basis: 100%;
    margin-right: 0;
  }
  .schema-ui-slider :deep(.el-slider__input) {
    margin-left: auto;
  }
}
.schema-ui-slider__label {
  font-size: var(--ac-font-size-base, 14px);
  color: var(--amitia-text-primary, inherit);
}
.schema-ui-list {
  margin: 0;
  padding-left: 18px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: var(--amitia-color-text, inherit);
}
.schema-ui-list__item {
  line-height: 1.6;
}

.schema-ui-gallery {
  display: grid;
  gap: 10px;
}
.schema-ui-gallery__count {
  grid-column: 1 / -1;
  color: var(--text-secondary);
  font-size: var(--ac-font-size-sm);
}

.schema-ui-gallery__item {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border: 1px solid var(--amitia-color-border, rgba(127, 127, 127, 0.22));
  border-radius: 8px;
  background: var(--amitia-color-surface, transparent);
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.schema-ui-gallery__item:hover {
  border-color: var(--amitia-color-accent, rgba(90, 120, 255, 0.7));
}

.schema-ui-gallery--assets,
.schema-ui-gallery--collections {
  gap: 20px;
  align-items: start;
}
.schema-ui-gallery--assets .schema-ui-gallery__item,
.schema-ui-gallery--collections .schema-ui-gallery__item {
  padding: 8px;
  border-color: transparent;
  border-radius: 12px;
  background: transparent;
}
.schema-ui-gallery--assets .schema-ui-gallery__item:hover,
.schema-ui-gallery--collections .schema-ui-gallery__item:hover,
.schema-ui-gallery__item.is-selected {
  background: var(--control-hover-bg, rgba(127, 127, 127, 0.08));
  border-color: var(--ac-color-primary);
}

.schema-ui-gallery--groups {
  display: flex;
  gap: 4px;
  padding: 2px 0 6px;
  overflow-x: auto;
  overscroll-behavior-x: contain;
  scrollbar-width: thin;
}

.schema-ui-gallery--groups .schema-ui-gallery__item {
  width: 66px;
  flex: 0 0 66px;
  align-items: center;
  gap: 4px;
  padding: 4px;
  border-color: transparent;
  background: transparent;
  text-align: center;
}

.schema-ui-gallery--groups .schema-ui-gallery__image,
.schema-ui-gallery--groups .schema-ui-gallery__placeholder {
  width: 46px;
  height: 46px;
  aspect-ratio: 1;
  border-radius: 50%;
}

.schema-ui-gallery--groups .schema-ui-gallery__placeholder {
  background: var(--ac-color-bg-secondary);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 600;
}

.schema-ui-gallery--groups .schema-ui-gallery__title {
  width: 100%;
  font-size: 10px;
  font-weight: 500;
  text-align: center;
}

.schema-ui-gallery--stickers {
  gap: 4px;
}

.schema-ui-gallery--stickers .schema-ui-gallery__item {
  padding: 3px;
  border-color: transparent;
  background: transparent;
}

.schema-ui-gallery--stickers .schema-ui-gallery__item:hover {
  background: var(--surface-hover);
  border-color: transparent;
}

.schema-ui-gallery--stickers .schema-ui-gallery__image,
.schema-ui-gallery--stickers .schema-ui-gallery__placeholder {
  border-radius: 7px;
}

.schema-ui-gallery__item:focus-visible {
  outline: 2px solid var(--ac-color-primary);
  outline-offset: 2px;
}
.schema-ui-gallery__cover {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-template-rows: repeat(2, minmax(0, 1fr));
  gap: 4px;
  width: 100%;
  aspect-ratio: 4 / 3;
  overflow: hidden;
  border-radius: 10px;
  background: var(--control-hover-bg, rgba(127, 127, 127, 0.06));
}
.schema-ui-gallery__cover img {
  width: 100%;
  height: 100%;
  min-height: 0;
  object-fit: contain;
}
.schema-ui-gallery__cover-empty {
  background: var(--control-hover-bg, rgba(127, 127, 127, 0.04));
}

.schema-ui-gallery__image,
.schema-ui-gallery__placeholder {
  width: 100%;
  aspect-ratio: 1 / 1;
  border-radius: 6px;
  object-fit: contain;
  background: var(--amitia-color-surface-elevated, rgba(127, 127, 127, 0.08));
}

.schema-ui-gallery__placeholder,
.schema-ui-gallery__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.8));
  font-size: 12px;
}

.schema-ui-gallery__empty {
  min-height: 72px;
  grid-column: 1 / -1;
  border: 1px dashed var(--amitia-color-border, rgba(127, 127, 127, 0.25));
  border-radius: 8px;
}

.schema-ui-gallery__title {
  overflow: hidden;
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-ui-gallery__subtitle {
  overflow: hidden;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.8));
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.schema-ui-file-picker {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.schema-ui-file-picker__input {
  display: none;
}

.schema-ui-file-picker__hint {
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.8));
  font-size: 12px;
}
.schema-ui-table {
  width: 100%;
  max-width: 100%;
  overflow-x: auto;
}
.schema-ui-code {
  margin: 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--amitia-color-surface-elevated, rgba(127, 127, 127, 0.12));
  overflow-x: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--amitia-color-text, inherit);
}
.schema-ui-code code {
  font-family: inherit;
}
.schema-ui-permission-summary {
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--amitia-color-border, rgba(127, 127, 127, 0.2));
  background: var(--amitia-color-surface, transparent);
}
.schema-ui-permission-summary__title {
  font-size: 12px;
  font-weight: 600;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.85));
  margin-bottom: 6px;
}
.schema-ui-permission-summary__list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.schema-ui-permission-summary__item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--amitia-color-text, inherit);
}
.schema-ui-permission-summary__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--amitia-color-accent, var(--tp-primary));
  flex-shrink: 0;
}
.schema-ui-permission-summary__empty {
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.7));
}
.schema-ui-runtime-status {
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--amitia-color-border, rgba(127, 127, 127, 0.2));
  background: var(--amitia-color-surface, transparent);
}
.schema-ui-runtime-status__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.schema-ui-runtime-status__label {
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.85));
}
.schema-ui-runtime-status__badge {
  padding: 1px 8px;
  border-radius: 10px;
  font-size: 11px;
  background: rgba(127, 127, 127, 0.15);
  color: var(--amitia-color-text, inherit);
}
.schema-ui-runtime-status__badge[data-status="ready"],
.schema-ui-runtime-status__badge[data-status="running"],
.schema-ui-runtime-status__badge[data-status="ok"] {
  background: rgba(80, 200, 80, 0.15);
  color: rgb(40, 160, 40);
}
.schema-ui-runtime-status__badge[data-status="error"],
.schema-ui-runtime-status__badge[data-status="failed"],
.schema-ui-runtime-status__badge[data-status="offline"] {
  background: rgba(220, 60, 60, 0.15);
  color: rgb(180, 40, 40);
}
.schema-ui-runtime-status__message {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.85));
}
.schema-ui-column__label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: var(--amitia-color-text-secondary, rgba(127, 127, 127, 0.85));
  margin-bottom: 4px;
}
.schema-ui-node__depth-exceeded,
.schema-ui-node__unknown {
  padding: 6px 8px;
  border-radius: 4px;
  font-size: 11px;
  color: rgb(180, 80, 40);
  background: rgba(220, 140, 40, 0.1);
  border: 1px dashed rgba(220, 140, 40, 0.4);
}
.schema-ui-node__fallback {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.schema-ui-node :deep(.el-card), .schema-ui-node :deep(.el-table), .schema-ui-node :deep(.el-input__wrapper) { max-width: 100%; background: var(--amitia-bg-surface); border-color: var(--amitia-border); }
.schema-ui-node :deep(.el-button:focus-visible), .schema-ui-node :deep(input:focus-visible) { outline: 2px solid var(--surface-border-focus); outline-offset: 2px; }
</style>
