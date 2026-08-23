<template>
  <div class="panel">
    <div class="head"><div><h2>通知设置</h2><p>管理系统通知订阅并发送测试通知。</p></div><el-button :loading="loading" @click="load">刷新</el-button></div>
    <el-card shadow="never">
      <el-form label-position="left" label-width="150px">
        <el-form-item label="启用通知"><el-switch v-model="settings.enabled" :loading="saving" @change="saveSettings" /></el-form-item>
        <el-form-item label="订阅状态"><el-tag :type="status.subscribed ? 'success' : 'info'">{{ status.subscribed ? '已订阅' : '未订阅' }}</el-tag></el-form-item>
        <el-form-item label="订阅标识" v-if="status.subscriptionId"><code>{{ status.subscriptionId }}</code></el-form-item>
      </el-form>
      <div class="actions">
        <el-button type="primary" :loading="saving" @click="subscribe">订阅通知</el-button>
        <el-button :disabled="!status.subscribed" :loading="saving" @click="unsubscribe">取消订阅</el-button>
        <el-button :loading="saving" @click="test">发送测试通知</el-button>
      </div>
    </el-card>
  </div>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { apiClient } from '@/composables/useApi';
const loading = ref(false); const saving = ref(false);
const settings = reactive<any>({ enabled: false });
const status = reactive<any>({ subscribed: false, subscriptionId: '' });
async function load(){ loading.value=true; try { const [s,st]=await Promise.all([apiClient.get('/api/notifications/settings'),apiClient.get('/api/notifications/status')]); Object.assign(settings,s.data||{}); Object.assign(status,st.data||{});} finally {loading.value=false;} }
async function saveSettings(){ saving.value=true; try{ await apiClient.put('/api/notifications/settings',{...settings}); ElMessage.success('通知设置已保存'); await load(); } finally{saving.value=false;} }
async function subscribe(){
  saving.value=true;
  try{
    if (typeof Notification !== 'undefined' && Notification.permission !== 'granted') {
      const permission = await Notification.requestPermission();
      if (permission !== 'granted') {
        ElMessage.warning('浏览器/系统未授予通知权限');
        return;
      }
    }
    const r=await apiClient.post('/api/notifications/subscribe',{});
    Object.assign(status,r.data||{});
    ElMessage.success('通知订阅已启用');
    await load();
  } finally{saving.value=false;}
}
async function unsubscribe(){ saving.value=true; try{ await apiClient.post('/api/notifications/unsubscribe',{}); ElMessage.success('通知订阅已取消'); await load(); } finally{saving.value=false;} }
async function test(){
  saving.value=true;
  try{
    const r=await apiClient.post('/api/notifications/test',{});
    const result=r.data||{};
    if (!result.accepted) {
      ElMessage.warning(result.reason||'请先启用并订阅通知');
      return;
    }
    if (typeof Notification === 'undefined') {
      ElMessage.warning('当前运行环境不支持系统通知 API');
      return;
    }
    if (Notification.permission !== 'granted') {
      const permission=await Notification.requestPermission();
      if (permission !== 'granted') {
        ElMessage.warning('浏览器/系统未授予通知权限');
        return;
      }
    }
    new Notification('Amitia 测试通知',{body:'桌面通知链路工作正常。'});
    ElMessage.success('测试通知已发送');
  } finally{saving.value=false;}
}
onMounted(load);
</script>
<style scoped>.panel{display:grid;gap:16px}.head{display:flex;align-items:flex-start;justify-content:space-between}.head h2{margin:0 0 4px}.head p{margin:0;color:var(--el-text-color-secondary);font-size:13px}.actions{display:flex;gap:10px;flex-wrap:wrap}code{word-break:break-all}</style>
