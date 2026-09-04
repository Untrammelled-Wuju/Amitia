<template>
  <el-form label-position="top" @submit.prevent>
    <el-form-item
      v-for="field in fields"
      :key="field.key"
      :label="field.label"
      :required="field.required"
    >
      <el-switch
        v-if="field.component === 'switch'"
        v-model="model[field.key]"
      />
      <el-input-number
        v-else-if="field.component === 'number'"
        v-model="model[field.key]"
        controls-position="right"
      />
      <el-select
        v-else-if="field.component === 'select'"
        v-model="model[field.key]"
      >
        <el-option
          v-for="option in field.options || []"
          :key="String(option)"
          :label="String(option)"
          :value="option"
        />
      </el-select>
      <el-input
        v-else-if="field.component === 'textarea'"
        v-model="model[field.key]"
        type="textarea"
        :rows="4"
      />
      <el-input
        v-else
        v-model="model[field.key]"
        :type="field.component === 'secret' ? 'password' : 'text'"
        show-password
        :aria-label="field.label"
      />
    </el-form-item>
    <el-button type="primary" :loading="saving" @click="$emit('save')"
      >保存设置</el-button
    >
  </el-form>
</template>

<script setup lang="ts">
import type { SurfaceField } from "../types";

defineProps<{ fields: SurfaceField[]; saving?: boolean }>();
const model = defineModel<Record<string, any>>({ required: true });
defineEmits<{ save: [] }>();
</script>
