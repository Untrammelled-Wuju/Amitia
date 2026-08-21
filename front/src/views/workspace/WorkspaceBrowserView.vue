<template>
  <div class="workspace-page">
    <div class="head"><div><h1>工作区</h1><p>浏览后端已注册的本地、SAF、远程或隔离工作区。</p></div><el-button :loading="loading" @click="loadMounts">刷新</el-button></div>
    <div class="layout">
      <el-card shadow="never" class="mounts"><template #header>工作区</template>
        <el-empty v-if="!mounts.length" description="暂无可用工作区" />
        <button v-for="m in mounts" :key="m.id" class="mount" :class="{active:m.id===activeMount?.id}" @click="selectMount(m)">
          <strong>{{ m.name }}</strong><span>{{ m.kind }} · {{ m.status }}</span>
        </button>
      </el-card>
      <el-card shadow="never" class="browser"><template #header><div class="crumb"><el-button text :disabled="!canUp" @click="up">上一级</el-button><code>{{ currentUri || '请选择工作区' }}</code></div></template>
        <el-table v-if="entries.length" :data="entries" @row-dblclick="openEntry">
          <el-table-column label="名称" min-width="260"><template #default="{row}"><span class="entry" @click="openEntry(row)">{{ row.type==='directory'?'📁':'📄' }} {{ row.name }}</span></template></el-table-column>
          <el-table-column prop="type" label="类型" width="110" /><el-table-column label="大小" width="120"><template #default="{row}">{{ formatBytes(row.sizeBytes) }}</template></el-table-column>
          <el-table-column prop="mimeType" label="MIME" min-width="160" />
        </el-table>
        <el-empty v-else :description="currentUri ? '目录为空或无法列出' : '请选择工作区'" />
      </el-card>
    </div>
    <el-dialog v-model="previewOpen" :title="previewName" width="70%"><pre class="preview">{{ previewContent }}</pre></el-dialog>
  </div>
</template>
<script setup lang="ts">
import { computed,onMounted,ref } from 'vue'; import { ElMessage } from 'element-plus'; import { apiClient } from '@/composables/useApi';
type Mount={id:string;name:string;kind:string;status:string;rootUri:string;available:boolean}; type Entry={uri:string;name:string;type:'file'|'directory';mimeType?:string;sizeBytes?:number;readable:boolean};
const loading=ref(false),mounts=ref<Mount[]>([]),activeMount=ref<Mount|null>(null),currentUri=ref(''),entries=ref<Entry[]>([]),previewOpen=ref(false),previewName=ref(''),previewContent=ref('');
const canUp=computed(()=>!!activeMount.value && currentUri.value!==activeMount.value.rootUri);
async function loadMounts(){loading.value=true;try{const r=await apiClient.get('/api/workspaces');mounts.value=Array.isArray(r.data)?r.data:[]; if(activeMount.value){const fresh=mounts.value.find(x=>x.id===activeMount.value?.id); if(fresh)activeMount.value=fresh;}}finally{loading.value=false;}}
async function selectMount(m:Mount){activeMount.value=m;currentUri.value=m.rootUri;await list();}
async function list(){if(!currentUri.value)return;loading.value=true;try{const r=await apiClient.get('/api/workspaces/list',{params:{uri:currentUri.value,limit:500}});entries.value=r.data?.Entries??r.data?.entries??[];}catch{entries.value=[];}finally{loading.value=false;}}
function parentUri(uri:string){const root=activeMount.value?.rootUri||''; if(uri===root)return root; const clean=uri.replace(/\/+$/,''); const slash=clean.lastIndexOf('/'); const p=slash>=0?clean.slice(0,slash):root; return p.length>=root.length?p:root;}
async function up(){currentUri.value=parentUri(currentUri.value);await list();}
async function openEntry(e:Entry){if(e.type==='directory'){currentUri.value=e.uri;await list();return;} if(!e.readable){ElMessage.warning('该文件不可读取');return;} const r=await apiClient.get('/api/workspaces/read',{params:{uri:e.uri,maxBytes:1048576}}); if(!r.data?.isText){ElMessage.warning('当前仅支持预览文本文件');return;} previewName.value=e.name;previewContent.value=String(r.data?.content??'');previewOpen.value=true;}
function formatBytes(v?:number){if(v==null)return '—';if(v<1024)return `${v} B`;if(v<1024*1024)return `${(v/1024).toFixed(1)} KB`;return `${(v/1024/1024).toFixed(1)} MB`;}
onMounted(loadMounts);
</script>
<style scoped>.workspace-page{display:grid;gap:16px;padding:4px}.head{display:flex;justify-content:space-between;align-items:flex-start}.head h1{margin:0 0 4px;font-size:22px}.head p{margin:0;color:var(--el-text-color-secondary)}.layout{display:grid;grid-template-columns:260px minmax(0,1fr);gap:16px}.mounts{min-height:520px}.mount{width:100%;border:0;background:transparent;text-align:left;padding:10px;border-radius:8px;display:grid;gap:3px;cursor:pointer}.mount span{font-size:12px;color:var(--el-text-color-secondary)}.mount:hover,.mount.active{background:var(--el-fill-color-light)}.crumb{display:flex;align-items:center;gap:12px}.crumb code{word-break:break-all}.entry{cursor:pointer}.preview{max-height:65vh;overflow:auto;white-space:pre-wrap;word-break:break-word;background:var(--el-fill-color-light);padding:14px;border-radius:8px}@media(max-width:900px){.layout{grid-template-columns:1fr}.mounts{min-height:auto}}</style>
