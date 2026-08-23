<template>
  <div class="workspace-page">
    <div class="head">
      <div><h1>工作区</h1><p>浏览文件、管理 Git 仓库，并创建或检查隔离工作区。</p></div>
      <div class="head-actions">
        <el-button @click="isolatedDialog=true">创建隔离工作区</el-button>
        <el-button :loading="loading" @click="loadMounts">刷新</el-button>
      </div>
    </div>
    <div class="layout">
      <el-card shadow="never" class="mounts">
        <template #header>工作区</template>
        <el-empty v-if="!mounts.length" description="暂无可用工作区" />
        <button v-for="m in mounts" :key="m.id" class="mount" :class="{active:m.id===activeMount?.id}" @click="selectMount(m)">
          <strong>{{ m.name }}</strong><span>{{ m.kind }} · {{ m.status }}<template v-if="m.readOnly"> · 只读</template></span>
        </button>
      </el-card>
      <el-card shadow="never" class="browser">
        <template #header>
          <div class="browser-head">
            <el-tabs v-model="activeTab" class="mode-tabs"><el-tab-pane label="文件" name="files"/><el-tab-pane label="Git" name="git"/></el-tabs>
            <div class="crumb" v-if="activeTab==='files'"><el-button text :disabled="!canUp" @click="up">上一级</el-button><code>{{ currentUri || '请选择工作区' }}</code></div>
            <div class="git-actions" v-else-if="activeMount">
              <el-button size="small" :loading="gitLoading" @click="loadGit">刷新 Git</el-button>
              <el-button v-if="activeMount.kind==='isolated'" size="small" @click="loadIsolatedInfo">隔离信息</el-button>
            </div>
          </div>
        </template>

        <template v-if="activeTab==='files'">
          <el-table v-if="entries.length" :data="entries" @row-dblclick="openEntry">
            <el-table-column label="名称" min-width="260"><template #default="{row}"><span class="entry" @click="openEntry(row)">{{ row.type==='directory'?'📁':'📄' }} {{ row.name }}</span></template></el-table-column>
            <el-table-column prop="type" label="类型" width="110" />
            <el-table-column label="大小" width="120"><template #default="{row}">{{ formatBytes(row.sizeBytes) }}</template></el-table-column>
            <el-table-column prop="mimeType" label="MIME" min-width="160" />
          </el-table>
          <el-empty v-else :description="currentUri ? '目录为空或无法列出' : '请选择工作区'" />
        </template>

        <template v-else>
          <el-empty v-if="!activeMount" description="请选择工作区" />
          <div v-else v-loading="gitLoading" class="git-panel">
            <el-alert v-if="gitError" type="error" :closable="false" :title="gitError" />
            <template v-if="gitStatus">
              <div class="git-summary">
                <el-tag>{{ gitStatus.branch || 'detached' }}</el-tag>
                <el-tag :type="gitStatus.clean?'success':'warning'">{{ gitStatus.clean?'工作区干净':`${gitEntries.length} 项变更` }}</el-tag>
                <el-tag type="info">↑{{ gitStatus.ahead || 0 }} ↓{{ gitStatus.behind || 0 }}</el-tag>
                <code>{{ gitStatus.head || '—' }}</code>
              </div>
              <div class="toolbar">
                <el-button :disabled="activeMount.readOnly" @click="stageAll">暂存全部</el-button>
                <el-button type="primary" :disabled="activeMount.readOnly" @click="commitDialog=true">提交</el-button>
                <el-button @click="fetchGit">Fetch</el-button>
                <el-button :disabled="activeMount.readOnly" @click="pullGit">Pull</el-button>
                <el-button :disabled="activeMount.readOnly" @click="openPush">Push</el-button>
              </div>

              <h3>变更</h3>
              <el-table :data="gitEntries" empty-text="没有未提交变更" size="small">
                <el-table-column prop="uri" label="文件" min-width="280" show-overflow-tooltip />
                <el-table-column prop="staging" label="暂存区" width="100" />
                <el-table-column prop="worktree" label="工作区" width="100" />
                <el-table-column label="状态" width="110"><template #default="{row}"><el-tag v-if="row.conflict" type="danger" size="small">冲突</el-tag><el-tag v-else-if="row.untracked" type="info" size="small">未跟踪</el-tag><span v-else>—</span></template></el-table-column>
                <el-table-column label="操作" width="230" fixed="right"><template #default="{row}">
                  <el-button link type="primary" @click="showDiff(row.uri)">Diff</el-button>
                  <el-button link type="primary" :disabled="activeMount?.readOnly" @click="stagePath(row.uri)">暂存</el-button>
                  <el-popconfirm title="还原该文件的工作区改动？" @confirm="restorePath(row.uri)"><template #reference><el-button link type="danger" :disabled="activeMount?.readOnly">还原</el-button></template></el-popconfirm>
                </template></el-table-column>
              </el-table>

              <div class="git-columns">
                <section><h3>分支</h3><el-table :data="gitBranches" size="small" max-height="300">
                  <el-table-column label="" width="42"><template #default="{row}"><span>{{ row.current?'●':'○' }}</span></template></el-table-column>
                  <el-table-column prop="name" label="分支" min-width="160" />
                  <el-table-column prop="commit" label="Commit" min-width="120" show-overflow-tooltip />
                  <el-table-column width="85"><template #default="{row}"><el-button link :disabled="row.current || activeMount?.readOnly" @click="checkout(row.name)">切换</el-button></template></el-table-column>
                </el-table><el-button class="section-action" :disabled="activeMount.readOnly" @click="newBranchDialog=true">新建分支</el-button></section>
                <section><h3>远程</h3><el-table :data="gitRemotes" size="small" max-height="300">
                  <el-table-column prop="name" label="名称" width="100" />
                  <el-table-column prop="fetchUrl" label="Fetch URL" min-width="220" show-overflow-tooltip />
                  <el-table-column label="凭据" width="70"><template #default="{row}">{{ row.hasCredential?'有':'无' }}</template></el-table-column>
                </el-table></section>
              </div>

              <h3>提交历史</h3>
              <el-table :data="gitLog" size="small" max-height="340">
                <el-table-column label="Hash" width="110"><template #default="{row}"><code>{{ String(row.hash||'').slice(0,8) }}</code></template></el-table-column>
                <el-table-column prop="subject" label="提交" min-width="240" />
                <el-table-column prop="authorName" label="作者" width="140" />
                <el-table-column prop="committedAt" label="时间" min-width="180" />
              </el-table>
            </template>
          </div>
        </template>
      </el-card>
    </div>

    <el-dialog v-model="previewOpen" :title="previewName" width="70%"><pre class="preview">{{ previewContent }}</pre></el-dialog>
    <el-dialog v-model="diffOpen" title="Git Diff" width="80%"><pre class="preview diff">{{ diffContent }}</pre></el-dialog>
    <el-dialog v-model="rawOpen" :title="rawTitle" width="70%"><pre class="preview">{{ rawContent }}</pre></el-dialog>

    <el-dialog v-model="commitDialog" title="创建 Git 提交" width="520px">
      <el-input v-model="commitMessage" type="textarea" :rows="4" placeholder="提交信息" />
      <template #footer><el-button @click="commitDialog=false">取消</el-button><el-button type="primary" :disabled="!commitMessage.trim()" @click="commitGit">提交</el-button></template>
    </el-dialog>
    <el-dialog v-model="newBranchDialog" title="新建并切换分支" width="520px">
      <el-form label-position="top"><el-form-item label="分支名"><el-input v-model="newBranch.name" /></el-form-item><el-form-item label="起点（可选）"><el-input v-model="newBranch.fromRef" placeholder="HEAD" /></el-form-item></el-form>
      <template #footer><el-button @click="newBranchDialog=false">取消</el-button><el-button type="primary" :disabled="!newBranch.name.trim()" @click="createBranch">创建</el-button></template>
    </el-dialog>
    <el-dialog v-model="pushDialog" title="Push" width="520px">
      <el-form label-position="top"><el-form-item label="Remote"><el-input v-model="push.remote" /></el-form-item><el-form-item label="Local ref"><el-input v-model="push.localRef" /></el-form-item><el-form-item label="Remote ref"><el-input v-model="push.remoteRef" /></el-form-item><el-checkbox v-model="push.setUpstream">设置 upstream</el-checkbox></el-form>
      <template #footer><el-button @click="pushDialog=false">取消</el-button><el-button type="primary" @click="pushGit">Push</el-button></template>
    </el-dialog>
    <el-dialog v-model="isolatedDialog" title="创建隔离工作区" width="560px">
      <el-form label-position="top">
        <el-form-item label="名称"><el-input v-model="isolatedForm.name" /></el-form-item>
        <el-form-item label="模式"><el-select v-model="isolatedForm.mode"><el-option label="Snapshot" value="snapshot"/><el-option label="Git Clone" value="git_clone"/></el-select></el-form-item>
        <el-form-item v-if="isolatedForm.mode==='snapshot'" label="源工作区 URI"><el-input v-model="isolatedForm.sourceWorkspaceUri" placeholder="可选择左侧工作区后自动填写" /></el-form-item>
        <el-form-item v-else label="Git Remote URL"><el-input v-model="isolatedForm.remoteUrl" /></el-form-item>
        <el-form-item label="Ref"><el-input v-model="isolatedForm.refName" placeholder="可选" /></el-form-item>
        <el-form-item label="生命周期"><el-input v-model="isolatedForm.lifetime" placeholder="例如 24h，可选" /></el-form-item>
        <el-checkbox v-model="isolatedForm.readOnly">只读</el-checkbox>
      </el-form>
      <template #footer><el-button @click="isolatedDialog=false">取消</el-button><el-button type="primary" :disabled="!isolatedForm.name.trim()" @click="createIsolated">创建</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed,onMounted,ref,watch } from 'vue';
import { ElMessage } from 'element-plus';
import { apiClient } from '@/composables/useApi';

type Mount={id:string;name:string;kind:string;status:string;rootUri:string;available:boolean;readOnly?:boolean};
type Entry={uri:string;name:string;type:'file'|'directory';mimeType?:string;sizeBytes?:number;readable:boolean};

const loading=ref(false),mounts=ref<Mount[]>([]),activeMount=ref<Mount|null>(null),currentUri=ref(''),entries=ref<Entry[]>([]),previewOpen=ref(false),previewName=ref(''),previewContent=ref('');
const activeTab=ref<'files'|'git'>('files');
const gitLoading=ref(false),gitError=ref(''),gitStatus=ref<any>(null),gitBranches=ref<any[]>([]),gitRemotes=ref<any[]>([]),gitLog=ref<any[]>([]);
const diffOpen=ref(false),diffContent=ref(''),rawOpen=ref(false),rawTitle=ref(''),rawContent=ref('');
const commitDialog=ref(false),commitMessage=ref(''),newBranchDialog=ref(false),newBranch=ref({name:'',fromRef:''});
const pushDialog=ref(false),push=ref({remote:'origin',localRef:'',remoteRef:'',setUpstream:false});
const isolatedDialog=ref(false),isolatedForm=ref({name:'',mode:'snapshot',sourceWorkspaceUri:'',remoteUrl:'',refName:'',lifetime:'',readOnly:false});
const canUp=computed(()=>!!activeMount.value && currentUri.value!==activeMount.value.rootUri);
const gitEntries=computed<any[]>(()=>Array.isArray(gitStatus.value?.entries)?gitStatus.value.entries:[]);

async function loadMounts(){loading.value=true;try{const r=await apiClient.get('/api/workspaces');mounts.value=Array.isArray(r.data)?r.data:[];if(activeMount.value){const fresh=mounts.value.find(x=>x.id===activeMount.value?.id);if(fresh)activeMount.value=fresh;}}finally{loading.value=false;}}
async function selectMount(m:Mount){activeMount.value=m;currentUri.value=m.rootUri;isolatedForm.value.sourceWorkspaceUri=m.rootUri;await list();if(activeTab.value==='git')await loadGit();}
async function list(){if(!currentUri.value)return;loading.value=true;try{const r=await apiClient.get('/api/workspaces/list',{params:{uri:currentUri.value,limit:500}});entries.value=r.data?.Entries??r.data?.entries??[];}catch{entries.value=[];}finally{loading.value=false;}}
function parentUri(uri:string){const root=activeMount.value?.rootUri||'';if(uri===root)return root;const clean=uri.replace(/\/+$/,'');const slash=clean.lastIndexOf('/');const p=slash>=0?clean.slice(0,slash):root;return p.length>=root.length?p:root;}
async function up(){currentUri.value=parentUri(currentUri.value);await list();}
async function openEntry(e:Entry){if(e.type==='directory'){currentUri.value=e.uri;await list();return;}if(!e.readable){ElMessage.warning('该文件不可读取');return;}const r=await apiClient.get('/api/workspaces/read',{params:{uri:e.uri,maxBytes:1048576}});if(!r.data?.isText){ElMessage.warning('当前仅支持预览文本文件');return;}previewName.value=e.name;previewContent.value=String(r.data?.content??'');previewOpen.value=true;}
function formatBytes(v?:number){if(v==null)return '—';if(v<1024)return `${v} B`;if(v<1024*1024)return `${(v/1024).toFixed(1)} KB`;return `${(v/1024/1024).toFixed(1)} MB`;}
async function gitPost(path:string,data:any){return (await apiClient.post(path,data)).data;}
async function loadGit(){if(!activeMount.value)return;gitLoading.value=true;gitError.value='';const workspaceUri=activeMount.value.rootUri;try{const [status,branches,remotes,log]=await Promise.all([gitPost('/api/workspaces/git/status',{workspaceUri,includeIgnored:false}),gitPost('/api/workspaces/git/branches',{workspaceUri}),gitPost('/api/workspaces/git/remotes',{workspaceUri}),gitPost('/api/workspaces/git/log',{workspaceUri,limit:50})]);gitStatus.value=status;gitBranches.value=branches?.branches??[];gitRemotes.value=remotes?.remotes??remotes?.items??[];gitLog.value=log?.entries??[];}catch(e:any){gitError.value=e?.response?.data?.error||e?.message||String(e);}finally{gitLoading.value=false;}}
async function runGit(fn:()=>Promise<any>,message:string){try{await fn();ElMessage.success(message);await loadGit();}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||String(e));}}
async function stageAll(){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/add',{workspaceUri:activeMount.value!.rootUri,paths:[],all:true,force:false}),'已暂存全部变更');}
async function stagePath(uri:string){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/add',{workspaceUri:activeMount.value!.rootUri,paths:[uri],all:false,force:false}),'已暂存');}
async function restorePath(uri:string){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/restore',{workspaceUri:activeMount.value!.rootUri,paths:[uri],staged:false,worktree:true}),'已还原工作区文件');}
async function showDiff(uri:string){if(!activeMount.value)return;try{const data=await gitPost('/api/workspaces/git/diff',{workspaceUri:activeMount.value.rootUri,mode:'worktree',base:'',target:'',paths:[uri],maxBytes:2097152});diffContent.value=(data?.files??[]).map((f:any)=>`# ${f.uri}\n${f.patch||'[binary/no patch]'}`).join('\n\n');diffOpen.value=true;}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||String(e));}}
async function commitGit(){if(!activeMount.value||!commitMessage.value.trim())return;await runGit(()=>gitPost('/api/workspaces/git/commit',{workspaceUri:activeMount.value!.rootUri,message:commitMessage.value.trim()}),'提交完成');commitMessage.value='';commitDialog.value=false;}
async function checkout(branch:string){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/checkout',{workspaceUri:activeMount.value!.rootUri,branch,create:false,fromRef:'',detach:false,force:false}),`已切换到 ${branch}`);}
async function createBranch(){if(!activeMount.value||!newBranch.value.name.trim())return;await runGit(()=>gitPost('/api/workspaces/git/checkout',{workspaceUri:activeMount.value!.rootUri,branch:newBranch.value.name.trim(),create:true,fromRef:newBranch.value.fromRef.trim(),detach:false,force:false}),'分支已创建并切换');newBranchDialog.value=false;newBranch.value={name:'',fromRef:''};}
async function fetchGit(){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/fetch',{workspaceUri:activeMount.value!.rootUri}),'Fetch 完成');}
async function pullGit(){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/pull',{workspaceUri:activeMount.value!.rootUri}),'Pull 完成');}
function openPush(){const branch=String(gitStatus.value?.branch||'');push.value={remote:String(gitRemotes.value[0]?.name||'origin'),localRef:branch||'HEAD',remoteRef:branch||'HEAD',setUpstream:false};pushDialog.value=true;}
async function pushGit(){if(!activeMount.value)return;await runGit(()=>gitPost('/api/workspaces/git/push',{workspaceUri:activeMount.value!.rootUri,...push.value}),'Push 完成');pushDialog.value=false;}
async function loadIsolatedInfo(){if(!activeMount.value)return;try{const data=await gitPost('/api/workspaces/isolated/info',{workspaceUri:activeMount.value.rootUri});rawTitle.value=`隔离工作区 · ${activeMount.value.name}`;rawContent.value=JSON.stringify(data,null,2);rawOpen.value=true;}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||String(e));}}
async function createIsolated(){const f=isolatedForm.value;try{await gitPost('/api/workspaces/isolated',{name:f.name.trim(),mode:f.mode,sourceWorkspaceUri:f.mode==='snapshot'?f.sourceWorkspaceUri.trim():'',gitRemote:f.mode==='git_clone'?{url:f.remoteUrl.trim(),ref:f.refName.trim()}:undefined,ref:f.refName.trim(),readOnly:f.readOnly,lifetime:f.lifetime.trim()});isolatedDialog.value=false;isolatedForm.value={name:'',mode:'snapshot',sourceWorkspaceUri:activeMount.value?.rootUri||'',remoteUrl:'',refName:'',lifetime:'',readOnly:false};ElMessage.success('隔离工作区已创建');await loadMounts();}catch(e:any){ElMessage.error(e?.response?.data?.error||e?.message||String(e));}}
watch(activeTab,tab=>{if(tab==='git')loadGit();});
onMounted(loadMounts);
</script>

<style scoped>
.workspace-page{display:grid;gap:16px;padding:4px}.head{display:flex;justify-content:space-between;align-items:flex-start}.head h1{margin:0 0 4px;font-size:22px}.head p{margin:0;color:var(--el-text-color-secondary);font-size:13px}.head-actions{display:flex;gap:8px}.layout{display:grid;grid-template-columns:260px minmax(0,1fr);gap:16px}.mounts{min-height:620px}.mount{width:100%;border:0;background:transparent;text-align:left;padding:10px;border-radius:8px;display:grid;gap:3px;cursor:pointer}.mount span{font-size:12px;color:var(--el-text-color-secondary)}.mount:hover,.mount.active{background:var(--el-fill-color-light)}.browser-head{display:flex;justify-content:space-between;align-items:center;gap:16px}.mode-tabs{width:150px}.mode-tabs :deep(.el-tabs__header){margin:0}.crumb{display:flex;align-items:center;gap:12px;min-width:0}.crumb code{word-break:break-all}.git-actions{display:flex;gap:8px}.entry{cursor:pointer}.preview{max-height:65vh;overflow:auto;white-space:pre-wrap;word-break:break-word;background:var(--el-fill-color-light);padding:14px;border-radius:8px}.diff{white-space:pre;overflow:auto}.git-panel{display:grid;gap:14px;min-height:500px}.git-summary,.toolbar{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.git-summary code{margin-left:auto;max-width:340px;overflow:hidden;text-overflow:ellipsis}.git-panel h3{font-size:15px;margin:10px 0 6px}.git-columns{display:grid;grid-template-columns:1fr 1fr;gap:16px}.git-columns section{min-width:0}.section-action{margin-top:8px}@media(max-width:1000px){.layout{grid-template-columns:1fr}.mounts{min-height:auto}.git-columns{grid-template-columns:1fr}.browser-head{align-items:flex-start;flex-direction:column}}
</style>
