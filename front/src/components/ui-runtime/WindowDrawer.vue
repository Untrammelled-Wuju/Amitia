<template>
  <ElementDrawer
    v-bind="$attrs"
    :model-value="modelValue"
    :append-to-body="appendToBody"
    @update:model-value="handleUpdate"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </ElementDrawer>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance } from "vue"
import { ElDrawer as ElementDrawer } from "element-plus"
import { useWindowLayerBlocker } from "@/composables/useWindowLayer"

defineOptions({ name: "ElDrawer", inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue?: boolean
  appendToBody?: boolean
}>(), {
  appendToBody: true,
})
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>()
const instance = getCurrentInstance()
const layerId = `drawer-${instance?.uid ?? "standalone"}`

useWindowLayerBlocker(layerId, computed(() => Boolean(props.modelValue)))

function handleUpdate(value: boolean) {
  emit("update:modelValue", value)
}
</script>
