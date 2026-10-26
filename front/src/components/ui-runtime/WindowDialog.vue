<template>
  <ElementDialog
    v-bind="$attrs"
    :model-value="modelValue"
    :append-to-body="appendToBody"
    @update:model-value="handleUpdate"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </ElementDialog>
</template>

<script setup lang="ts">
import { computed, getCurrentInstance } from "vue"
import { ElDialog as ElementDialog } from "element-plus"
import { useWindowLayerBlocker } from "@/composables/useWindowLayer"

defineOptions({ name: "ElDialog", inheritAttrs: false })

const props = withDefaults(defineProps<{
  modelValue?: boolean
  appendToBody?: boolean
}>(), {
  appendToBody: true,
})
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>()
const instance = getCurrentInstance()
const layerId = `dialog-${instance?.uid ?? "standalone"}`

useWindowLayerBlocker(layerId, computed(() => Boolean(props.modelValue)))

function handleUpdate(value: boolean) {
  emit("update:modelValue", value)
}
</script>
