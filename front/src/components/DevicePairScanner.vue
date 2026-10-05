<template>
  <el-dialog :model-value="modelValue" title="扫描设备配对码" width="min(560px, 94vw)" @close="close">
    <video ref="video" autoplay muted playsinline class="scan-preview" />
    <el-alert v-if="error" :title="error" type="warning" :closable="false" show-icon />
    <p>将另一台设备的 Amitia 二维码放入镜头，也可以选择二维码图片。</p>
    <input type="file" accept="image/*" aria-label="选择配对二维码图片" @change="readImage" />
    <template #footer><el-button @click="close">取消</el-button></template>
  </el-dialog>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { BrowserQRCodeReader, type IScannerControls } from '@zxing/browser';

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; scanned: [payload: string] }>();
const video = ref<HTMLVideoElement>();
const error = ref('');
let controls: IScannerControls | undefined;
let generation = 0;
let accepted = false;

function stop() {
  generation++;
  controls?.stop();
  controls = undefined;
  const stream = video.value?.srcObject;
  if (typeof MediaStream !== 'undefined' && stream instanceof MediaStream) stream.getTracks().forEach((track) => track.stop());
}

function close() {
  stop();
  emit('update:modelValue', false);
}

function accept(raw: string) {
  if (accepted || !props.modelValue) return;
  try {
    const parsed = new URL(raw);
    if (parsed.protocol !== 'amitia:' || parsed.hostname !== 'pair' || !parsed.searchParams.get('offer')) throw new Error();
    accepted = true;
    close();
    emit('scanned', raw);
  } catch { error.value = '请扫描 Amitia 设备配对二维码'; }
}

watch(() => props.modelValue, async (visible) => {
  stop();
  if (!visible) return;
  error.value = '';
  accepted = false;
  const current = generation;
  await nextTick();
  if (generation !== current || !props.modelValue) return;
  try {
    const started = await new BrowserQRCodeReader().decodeFromConstraints({ video: { facingMode: { ideal: 'environment' } }, audio: false }, video.value, (result) => {
      if (result && generation === current) accept(result.getText());
    });
    if (generation !== current || !props.modelValue) started.stop();
    else controls = started;
  } catch { if (generation === current) error.value = '摄像头暂不可用，请检查相机权限或选择二维码图片'; }
});

async function readImage(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (!file) return;
  const url = URL.createObjectURL(file);
  const current = generation;
  try {
    const result = await new BrowserQRCodeReader().decodeFromImageUrl(url);
    if (generation === current) accept(result.getText());
  } catch { if (generation === current) error.value = '没有识别到配对二维码，请选择清晰完整的图片'; }
  finally { URL.revokeObjectURL(url); }
}

onBeforeUnmount(stop);
</script>

<style scoped>
.scan-preview { width: 100%; max-height: 360px; object-fit: cover; border-radius: 12px; background: #111; }
</style>
