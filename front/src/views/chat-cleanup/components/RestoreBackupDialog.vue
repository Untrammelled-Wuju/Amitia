<template>
  <el-dialog
    :model-value="visible"
    title="恢复完整备份"
    width="680px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
    @closed="reset"
  >
    <template v-if="backup">
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="备份">{{ backup.name }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ backup.createdAt }}</el-descriptions-item>
        <el-descriptions-item label="加密">{{ backup.encrypted ? "是" : "否" }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ preview ? "已预检" : "待预检" }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="backup.encrypted" class="dialog-field">
        <el-input v-model="passphrase" type="password" show-password autocomplete="off" placeholder="输入备份口令" />
      </div>
      <div class="dialog-actions">
        <el-button :loading="previewing" @click="previewRestore">预检备份</el-button>
      </div>
      <template v-if="preview">
        <el-alert
          type="warning"
          :closable="false"
          show-icon
          title="恢复会替换当前设备 Runtime 数据"
          description="执行前系统会自动创建恢复前备份。恢复成功后需要重新启动完整项目。"
        />
        <el-table :data="preview.manifest.components" max-height="300" class="preview-table">
          <el-table-column prop="logicalName" label="组件" min-width="220" />
          <el-table-column prop="itemCount" label="记录数" width="110" />
          <el-table-column prop="kind" label="类型" width="120" />
        </el-table>
        <el-input v-model="confirmText" placeholder='输入“确认恢复”' />
      </template>
    </template>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="danger" :disabled="confirmText !== '确认恢复' || !preview" :loading="executing" @click="executeRestore">
        确认恢复
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  cancelDataPortabilityRestore,
  executeDataPortabilityRestore,
  previewDataPortabilityRestore,
  type DataPortabilityBackup,
  type DataPortabilityRestorePreview,
} from "@/runtime/data-portability";

const props = defineProps<{
  visible: boolean;
  backup: DataPortabilityBackup | null;
}>();

const emit = defineEmits<{
  (event: "update:visible", value: boolean): void;
}>();

const passphrase = ref("");
const previewing = ref(false);
const executing = ref(false);
const preview = ref<DataPortabilityRestorePreview | null>(null);
const confirmText = ref("");

watch(() => props.backup?.name, () => {
  preview.value = null;
  passphrase.value = "";
  confirmText.value = "";
});

async function previewRestore() {
  if (!props.backup) return;
  previewing.value = true;
  try {
    preview.value = await previewDataPortabilityRestore(props.backup.name, passphrase.value);
    ElMessage.success("备份校验通过");
  } catch (error: any) {
    ElMessage.error(error?.message || "备份预检失败");
  } finally {
    previewing.value = false;
  }
}

async function executeRestore() {
  if (!preview.value || confirmText.value !== "确认恢复") return;
  executing.value = true;
  try {
    await executeDataPortabilityRestore(preview.value.operationId);
    ElMessage.success("恢复完成，请重新启动完整项目");
    emit("update:visible", false);
  } catch (error: any) {
    ElMessage.error(error?.message || "恢复失败");
  } finally {
    executing.value = false;
  }
}

async function reset() {
  if (preview.value) {
    try {
      await cancelDataPortabilityRestore();
    } catch {}
  }
  preview.value = null;
  passphrase.value = "";
  confirmText.value = "";
}
</script>

<style scoped>
.dialog-field { margin-top: 16px; }
.dialog-actions { display: flex; justify-content: flex-end; margin: 14px 0; }
.preview-table { margin: 14px 0; }
</style>
