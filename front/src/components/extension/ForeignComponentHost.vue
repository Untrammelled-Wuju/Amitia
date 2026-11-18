<script setup lang="ts">
import {
  onBeforeUnmount,
  onMounted,
  inject,
  ref,
  shallowRef,
  watch,
} from "vue";
import {
  FOREIGN_COMPONENT_REGISTRY_KEY,
  foreignComponentRegistry,
  type ForeignComponentEvent,
  type ForeignComponentInstance,
  type ForeignComponentMountInput,
} from "@/ui-runtime/foreignComponentRuntime";
import type { UIPluginContext } from "@/ui-runtime/uiContext";

const props = withDefaults(defineProps<{
  adapter: string;
  context: UIPluginContext;
  componentProps?: Readonly<Record<string, unknown>>;
  ariaLabel?: string;
}>(), {
  componentProps: () => ({}),
  ariaLabel: undefined,
});
const emit = defineEmits<{
  (event: "component-event", value: ForeignComponentEvent): void;
  (event: "error", error: unknown): void;
}>();

const root = ref<HTMLElement>();
const registry = inject(FOREIGN_COMPONENT_REGISTRY_KEY, foreignComponentRegistry);
const instance = shallowRef<ForeignComponentInstance>();
const error = shallowRef<unknown>();
let active = true;
let mounted = false;
let queue = Promise.resolve();

function mountInput(): ForeignComponentMountInput | undefined {
  if (!root.value) return undefined;
  return {
    element: root.value,
    props: props.componentProps,
    context: props.context,
    emit: (value) => emit("component-event", value),
  };
}

function enqueue(task: () => void | Promise<void>): Promise<void> {
  queue = queue.then(task, task);
  return queue;
}

async function mountComponent(): Promise<void> {
  const input = mountInput();
  if (!input) throw new Error(`foreign component host ${props.adapter} has no root element`);
  const mountedInstance = await registry.mount(props.adapter, input);
  if (!active) {
    await mountedInstance.unmount();
    return;
  }
  instance.value = mountedInstance;
  mounted = true;
}

async function updateComponent(): Promise<void> {
  if (!mounted || !instance.value?.update) return;
  const input = mountInput();
  if (!input) throw new Error(`foreign component host ${props.adapter} has no root element`);
  await instance.value.update(input);
}

async function unmountComponent(): Promise<void> {
  const mountedInstance = instance.value;
  instance.value = undefined;
  mounted = false;
  if (mountedInstance) await mountedInstance.unmount();
}

function report(errorValue: unknown): void {
  error.value = errorValue;
  emit("error", errorValue);
}

onMounted(() => {
  void enqueue(mountComponent).catch(report);
});

watch(
  () => [props.adapter, props.componentProps, props.context] as const,
  () => {
    error.value = undefined;
    void enqueue(async () => {
      if (!mounted || instance.value?.update === undefined) {
        await unmountComponent();
        await mountComponent();
        return;
      }
      await updateComponent();
    }).catch(report);
  },
  { deep: true },
);

onBeforeUnmount(() => {
  active = false;
  void enqueue(unmountComponent).catch(report);
});
</script>

<template>
  <div
    ref="root"
    class="amitia-foreign-component"
    :aria-label="ariaLabel"
    :data-adapter="adapter"
  ></div>
  <div v-if="error" class="amitia-foreign-component__error" role="alert">
    {{ error instanceof Error ? `${error.name}: ${error.message}\n${error.stack ?? ""}` : String(error) }}
  </div>
</template>

<style scoped>
.amitia-foreign-component {
  display: contents;
}

.amitia-foreign-component__error {
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--ac-color-danger) 35%, transparent);
  border-radius: 6px;
  background: var(--ac-color-danger-bg);
  color: var(--ac-color-danger);
  font-family: monospace;
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
