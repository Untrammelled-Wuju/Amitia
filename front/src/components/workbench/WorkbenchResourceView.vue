<template>
  <div class="resource-view">
    <div v-if="tab.kind === 'tasks'"><WorkbenchTasksView :conversation-id="store.conversationId" :character-id="store.characterId" /></div>
    <div v-else-if="tab.targetDeviceId && (tab.kind !== 'browser' || !tab.browserRef)" class="muted" role="alert">目标设备资源不支持远程访问，已阻止向本机错误发起请求。</div>
    <template v-else-if="tab.kind === 'files'">
      <div class="toolbar"><select v-model="workspaceUri" aria-label="工作目录" @change="changeWorkspace"><option value="">选择工作区</option><option v-for="mount in mounts" :key="mount.id" :value="mount.rootUri" :disabled="!mount.available">{{ mount.name }}{{ mount.available ? '' : '（不可用）' }}</option></select><button type="button" @click="refresh">刷新</button></div>
      <div class="path">{{ directory || '请选择工作区' }}</div>
      <button v-if="directory && directory !== workspaceUri" class="entry" type="button" @click="goUp">.. 上一级</button>
      <button v-for="file in files" :key="file.uri" class="entry" type="button" @click="openEntry(file)"><span>{{ file.type === 'directory' ? '▸' : '▤' }}</span><span>{{ file.name }}</span><small v-if="file.sizeBytes !== undefined">{{ file.sizeBytes }} B</small></button>
      <button v-if="nextCursor && !loading" type="button" class="load-more" @click="loadMore">加载更多文件</button>
      <div v-if="!files.length && !loading" class="muted">没有文件或目录</div>
    </template>
    <template v-else-if="tab.kind === 'file'">
      <div class="toolbar"><span class="path" :title="tab.resourceUri">{{ tab.resourceUri }}</span><button type="button" @click="loadFile(false)">刷新</button></div>
      <pre v-if="content" class="code"><code>{{ content }}</code></pre>
      <div v-else-if="!loading" class="muted">{{ fileIsText ? '文件为空' : '不支持文本预览的文件类型' }}</div>
      <button v-if="fileHasMore && !loading" type="button" class="load-more" @click="loadFile(true)">加载下一部分文件</button>
    </template>
    <template v-else-if="tab.kind === 'git' || tab.kind === 'diff'">
      <div class="toolbar"><select v-model="workspaceUri" aria-label="Git 工作目录" @change="changeGitWorkspace"><option value="">选择仓库</option><option v-for="mount in mounts" :key="mount.id" :value="mount.rootUri" :disabled="!mount.available || (mount.kind !== 'local' && mount.kind !== 'isolated')">{{ mount.name }}</option></select><button type="button" @click="refresh">刷新</button></div>
      <div v-if="gitStatus" class="summary">{{ gitStatus.branch || 'detached' }} · {{ changes.length }} 项变更 <small v-if="gitStatus.ahead || gitStatus.behind">↑{{ gitStatus.ahead || 0 }} ↓{{ gitStatus.behind || 0 }}</small></div>
      <template v-if="tab.kind === 'git' && gitStatus">
        <div class="section-title">Git Worktree · 独立任务环境</div>
        <p class="muted">创建独立的 Git 工作树并注册为本机工作区。仅管理 U-Ai 创建的 Worktree；删除时不会强制丢弃未提交内容。</p>
        <div class="toolbar worktree-controls">
          <input v-model.trim="newWorktreeName" maxlength="100" aria-label="Worktree 显示名称" placeholder="工作区名称（可选）" />
          <input v-model.trim="newWorktreeBranch" maxlength="80" aria-label="新分支名称" placeholder="新分支（留空则 detached）" />
          <input v-model.trim="newWorktreeRef" maxlength="160" aria-label="起始提交" placeholder="起点：HEAD" />
          <input v-model.trim="newWorktreeTaskId" maxlength="128" aria-label="关联任务 ID" placeholder="关联任务 ID（可选，绑定当前对话）" />
          <button type="button" :disabled="worktreeBusy" @click="createWorktree">{{ worktreeBusy ? '处理中…' : '创建 Worktree' }}</button>
        </div>
        <div v-if="worktreeBusy" class="status" role="status">正在处理 Worktree…</div>
        <div v-for="tree in worktrees" :key="tree.path" class="worktree-item">
          <div class="worktree-description"><strong>{{ tree.branch || 'detached HEAD' }}</strong>
            <small>{{ tree.head.slice(0, 8) }} · {{ tree.managed ? 'U-Ai 管理' : '外部 Worktree' }}{{ tree.locked ? ' · 已锁定' : '' }}</small>
            <small v-if="tree.taskBinding">任务 {{ tree.taskBinding.taskId }} · 对话 {{ tree.taskBinding.conversationId }}（解绑前禁止删除）</small>
            <small class="path" :title="tree.path">{{ tree.path }}</small>
          </div>
          <button v-if="tree.workspaceUri" type="button" @click="openWorktree(tree)">浏览文件</button>
          <button v-if="tree.workspaceUri && tree.managed" type="button" @click="openWorktreeTerminal(tree)">在独立终端中打开</button>
          <button v-if="tree.managed && tree.taskBinding && tree.id" type="button" :disabled="worktreeBusy" @click="reviewWorktree(tree)">审查变更</button>
          <button v-if="tree.managed && tree.id && tree.taskBinding" type="button" :disabled="worktreeBusy" @click="releaseWorktree(tree)">解除任务绑定</button>
          <button v-if="tree.managed && tree.id" type="button" :disabled="worktreeBusy || tree.locked || tree.prunable || !!tree.taskBinding" @click="removeWorktree(tree)">安全删除</button>
        </div>
        <div v-if="worktreeReview" class="commit-detail" role="region" aria-label="任务 Worktree 审查">
          <div class="toolbar"><strong>任务 {{ worktreeReview.entry.taskBinding?.taskId }} · 工作树审查</strong><button type="button" @click="worktreeReview = null">关闭</button></div>
          <div class="summary">HEAD {{ worktreeReview.status.head?.slice(0, 8) }} · {{ worktreeReview.diff.stats.filesChanged }} 个文件 · <span class="added">+{{ worktreeReview.diff.stats.insertions }}</span> / <span class="removed">-{{ worktreeReview.diff.stats.deletions }}</span></div>
          <div v-if="worktreeReview.reviewIncomplete" class="status" role="alert">审查不完整：未跟踪文件 {{ worktreeReview.untrackedCount }} 个、旧绑定缺少起点，或 Diff/文件列表已截断。禁止仅凭此预览决定自动合并。</div>
          <div v-if="worktreeReview.conflictedCount" class="status error" role="alert">存在 {{ worktreeReview.conflictedCount }} 个 Git 冲突</div>
          <div class="section-title">任务验收检查点</div>
          <div class="toolbar worktree-controls">
            <button type="button" :disabled="worktreeBusy || worktreeReview.reviewIncomplete" @click="prepareCheckpoint">创建 / 恢复检查点</button>
            <select v-model="checkpointProfile" aria-label="验证测试方案"><option value="go">Go 测试</option><option value="go-race">Go race 测试</option></select>
            <button type="button" :disabled="worktreeBusy || !checkpoint || !!checkpoint.approvedAt" @click="runCheckpointTest">执行测试</button>
            <button type="button" :disabled="worktreeBusy || !checkpoint || !!checkpoint.approvedAt || !checkpoint.tests?.at(-1)?.passed" @click="approveCheckpoint">人工批准</button>
            <button v-if="checkpoint?.approvedAt" type="button" :disabled="worktreeBusy" @click="previewApprovedMerge">预检快进合并</button>
            <button v-if="checkpoint?.approvedAt" type="button" :disabled="worktreeBusy" @click="readMergeAudit">查看合并审计</button>
          </div>
          <div v-if="checkpoint" class="commit-detail">
            <div class="summary">固定提交 {{ checkpoint.head.slice(0, 12) }} · {{ checkpoint.approvedAt ? '已人工批准' : '未批准' }}</div>
            <small v-if="checkpoint.approvedBy">批准主体 {{ checkpoint.approvedBy }}</small>
            <div v-for="(test, index) in checkpoint.tests || []" :key="index" class="status" :class="{ error: !test.passed }">{{ test.profile }} · {{ test.passed ? '通过' : '失败' }} · exit {{ test.exitCode }} · {{ test.durationMillis }} ms<pre class="code"><code>{{ test.logExcerpt }}</code></pre></div>
            <div class="muted">执行前请检查待测代码是否可信。审批只适用于此提交，不会自动合并。</div>
            <div v-if="mergePlan" class="commit-detail" role="region" aria-label="安全合并预检">
              <div class="status">目标分支 {{ mergePlan.targetBranch }} · 目标 {{ mergePlan.targetHead.slice(0, 12) }} → 来源 {{ mergePlan.sourceHead.slice(0, 12) }} · {{ mergePlan.filesChanged }} 个文件</div>
              <div class="muted">仅允许干净目标工作树的 fast-forward 合并。目标分支或 HEAD 变化后须重新预检；不会强制覆盖或推送远端。</div>
              <button type="button" :disabled="worktreeBusy" @click="applyApprovedMerge">确认并执行本地快进合并</button>
            </div>

          </div>
          <pre class="code diff"><code><template v-for="(line, index) in worktreeReview.diff.files.map(f => f.patch || '[未包含文本差异]').join('\n').split('\n').slice(0, 3000)" :key="index"><span :class="diffClass(line)">{{ line }}{{ '\n' }}</span></template></code></pre>
        </div>
        <div v-if="mergeAudit" class="status" role="status">合并审计：{{ mergeAudit.result }} · {{ mergeAudit.appliedHead?.slice(0, 12) || '需人工核对' }}</div>
        <div v-if="worktreeError" class="status error" role="alert">{{ worktreeError }}</div>
        <div class="section-title">未提交变更</div>
        <button v-for="entry in changes" :key="entry.uri" type="button" class="entry" @click="openDiff(entry.uri)"><span :class="{ conflict: entry.conflict }">{{ entry.conflict ? '冲突' : entry.untracked ? '?' : `${entry.staging}${entry.worktree}` }}</span><span>{{ relativePath(entry.uri) }}</span></button>
        <div v-if="gitStatus.clean" class="muted">工作区没有未提交的更改</div>
        <div class="section-title">提交历史</div>
        <button v-for="commit in commits" :key="commit.hash" type="button" class="commit commit-button" @click="inspectCommit(commit)"><strong>{{ commit.subject }}</strong><small>{{ commit.hash.slice(0, 8) }} · {{ commit.authorName }} · {{ formatDate(commit.committedAt) }}</small></button>
        <div v-if="selectedCommit" class="commit-detail">
          <div class="toolbar"><strong>{{ selectedCommit.subject }}</strong><button type="button" @click="selectedCommit = null">关闭详情</button></div>
          <small>{{ selectedCommit.hash }} · {{ selectedCommit.authorName }}</small>
          <p v-if="selectedCommit.body">{{ selectedCommit.body }}</p>
          <div v-if="commitDiffStats" class="summary">{{ commitDiffStats.filesChanged }} 个文件 · <span class="added">+{{ commitDiffStats.insertions }}</span> / <span class="removed">-{{ commitDiffStats.deletions }}</span></div>
          <pre v-if="commitPatch" class="code"><code><template v-for="(line, index) in commitPatch.split('\n').slice(0, 3000)" :key="index"><span :class="diffClass(line)">{{ line }}{{ '\n' }}</span></template></code></pre>
          <div v-else class="muted">{{ commitDetailsMessage }}</div>
        </div>
        <div v-if="!commits.length" class="muted">暂无提交记录</div>
      </template>
      <template v-if="tab.kind === 'diff' && gitStatus">
        <div v-if="diffStats" class="summary">{{ diffStats.filesChanged }} 个文件 · <span class="added">+{{ diffStats.insertions }}</span> / <span class="removed">-{{ diffStats.deletions }}</span></div>
        <div v-if="diffTruncated" class="muted" role="status">变更过大，服务端已截断结果</div>
        <pre class="code diff"><code><template v-for="(line, index) in visibleDiffLines" :key="index"><span :class="diffClass(line)">{{ line }}{{ index < visibleDiffLines.length - 1 ? '\n' : '' }}</span></template></code></pre>
        <button v-if="diffLines.length > visibleDiffLines.length" type="button" class="load-more" @click="diffLineLimit += 3500">继续显示 {{ diffLines.length - visibleDiffLines.length }} 行</button>
      </template>
    </template>
    <template v-else-if="tab.kind === 'browser'"><WorkbenchBrowserView :tab="tab" /></template>
    <template v-else-if="tab.kind === 'terminal'"><WorkbenchTerminalView :tab="tab" /></template>
    <template v-else><div class="muted">交互式终端尚未连接。本视图不会模拟任何命令执行。</div></template>
    <div v-if="loading" class="status" role="status">正在加载…</div>
    <div v-if="error" class="status error" role="alert">{{ error }}</div>
  </div>
</template>
<script setup lang="ts">
import { ref, watch, computed, onActivated, onDeactivated, onBeforeUnmount } from 'vue';
import { apiClient } from '@/composables/useApi';
import { useWorkbenchStore, type WorkbenchTab } from '@/stores/workbench';
import WorkbenchBrowserView from './WorkbenchBrowserView.vue';
import WorkbenchTerminalView from './WorkbenchTerminalView.vue';
import WorkbenchTasksView from './WorkbenchTasksView.vue';
import { parseWorkbenchDocumentChunk, type WorkbenchDocumentContent } from './workbench-document-chunks';
import { needsWorkbenchWorkspaceData } from './workbench-resource-lifecycle';

interface Mount { id: string; name: string; kind: string; rootUri: string; available: boolean }
interface FileEntry { uri: string; name: string; type: 'directory' | 'file'; readable?: boolean; sizeBytes?: number }
interface GitChange { uri: string; staging: string; worktree: string; conflict: boolean; untracked: boolean }
interface GitCommit { hash: string; subject: string; body?: string; authorName: string; committedAt: string; parents?: string[] }
interface GitStatus { branch: string; ahead: number; behind: number; clean: boolean }
interface DiffStats { filesChanged: number; insertions: number; deletions: number }
interface GitWorktree { taskBinding?: { taskId: string; conversationId: string; boundAt: string }; id?: string; path: string; head: string; branch?: string; detached: boolean; managed: boolean; locked: boolean; prunable: boolean; workspaceUri?: string }
interface GitCheckpointTest { profile: string; revision: string; passed: boolean; exitCode: number; durationMillis: number; logExcerpt: string }
interface GitCheckpoint { revision: string; head: string; baseHead: string; approvedAt?: string; approvedBy?: string; tests?: GitCheckpointTest[] }
interface GitMergePlan { worktreeId: string; taskId: string; conversationId: string; revision: string; sourceHead: string; targetBranch: string; targetHead: string; planId: string; filesChanged: number; approvedBy: string }
interface GitMergeAudit { result: string; appliedHead?: string; plan: GitMergePlan; actor: string }
interface GitWorktreeReview { entry: GitWorktree; status: { head: string }; diff: { stats: DiffStats; files: Array<{ patch: string }> }; untrackedCount: number; conflictedCount: number; baselineKnown: boolean; reviewIncomplete: boolean }
const props = defineProps<{ tab: WorkbenchTab }>();
const store = useWorkbenchStore();
const mounts = ref<Mount[]>([]);
const files = ref<FileEntry[]>([]);
const directory = ref('');
const workspaceUri = ref(props.tab.workspaceUri);
const loading = ref(false);
const error = ref('');
const content = ref('');
const fileIsText = ref(true);
const fileOffset = ref(0);
const fileHasMore = ref(false);
let fileContent: WorkbenchDocumentContent | null = null;
const nextCursor = ref('');
const gitStatus = ref<GitStatus | null>(null);
const changes = ref<GitChange[]>([]);
const commits = ref<GitCommit[]>([]);
const selectedCommit = ref<GitCommit | null>(null);
const commitPatch = ref('');
const commitDiffStats = ref<DiffStats | null>(null);
const commitDetailsMessage = ref('');
const diffStats = ref<DiffStats | null>(null);
const diffTruncated = ref(false);
const diffLineLimit = ref(3500);
const worktrees = ref<GitWorktree[]>([]);
const worktreeBusy = ref(false);
const worktreeError = ref('');
const worktreeReview = ref<GitWorktreeReview | null>(null);
const checkpoint = ref<GitCheckpoint | null>(null);
const mergePlan = ref<GitMergePlan | null>(null);
const mergeAudit = ref<GitMergeAudit | null>(null);
const checkpointProfile = ref<'go' | 'go-race'>('go');
const newWorktreeName = ref('');
const newWorktreeBranch = ref('');
const newWorktreeRef = ref('');
const newWorktreeTaskId = ref('');
const diffLines = computed(() => content.value.split('\n'));
const visibleDiffLines = computed(() => diffLines.value.slice(0, diffLineLimit.value));
let sequence = 0;
function failure(value: unknown) { return (value as any)?.response?.data?.error || (value as Error)?.message || String(value); }
function relativePath(uri: string) { return uri.startsWith(workspaceUri.value) ? uri.slice(workspaceUri.value.length).replace(/^\//, '') : uri; }
function formatDate(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? '' : date.toLocaleString(); }
function diffClass(line: string) { return line.startsWith('+') && !line.startsWith('+++') ? 'added' : line.startsWith('-') && !line.startsWith('---') ? 'removed' : line.startsWith('@@') ? 'hunk' : ''; }
async function loadMounts() {
  const request = sequence;
  try {
    const response = await apiClient.get('/api/workspaces');
    if (request !== sequence) return;
    mounts.value = Array.isArray(response.data) ? response.data : [];
    if (!workspaceUri.value && mounts.value.length === 1 && mounts.value[0].available) {
      workspaceUri.value = mounts.value[0].rootUri;
      directory.value = workspaceUri.value;
    }
  } catch (cause) { if (request === sequence) error.value = failure(cause); }
}
function changeWorkspace() { worktrees.value = []; worktreeReview.value = null; checkpoint.value = null; mergePlan.value = null; mergeAudit.value = null; worktreeError.value = ''; selectedCommit.value = null; commitPatch.value = ''; directory.value = workspaceUri.value; nextCursor.value = ''; files.value = []; if (props.tab.kind === 'files') void loadFiles(false); else void loadGit(); }
async function loadFiles(append: boolean) {
  const request = ++sequence;
  if (!workspaceUri.value) { files.value = []; directory.value = ''; return; }
  if (!directory.value || (directory.value !== workspaceUri.value && !directory.value.startsWith(`${workspaceUri.value}/`))) directory.value = workspaceUri.value;
  loading.value = true; error.value = '';
  try {
    const response = await apiClient.get('/api/workspaces/list', { params: { uri: directory.value, limit: 200, ...(append && nextCursor.value ? { cursor: nextCursor.value } : {}) } });
    if (request !== sequence) return;
    const entries: FileEntry[] = Array.isArray(response.data?.entries) ? response.data.entries : [];
    files.value = append ? [...files.value, ...entries] : entries;
    nextCursor.value = response.data?.hasMore ? String(response.data?.nextCursor || '') : '';
  } catch (cause) { if (request === sequence) error.value = failure(cause); }
  finally { if (request === sequence) loading.value = false; }
}
function loadMore() { if (!loading.value && nextCursor.value) void loadFiles(true); }
function goUp() {
  const root = workspaceUri.value;
  const current = directory.value.replace(/\/+$/, '');
  const next = current.slice(0, current.lastIndexOf('/'));
  directory.value = next === root || next.startsWith(`${root}/`) ? next : root;
  nextCursor.value = '';
  void loadFiles(false);
}
function openEntry(entry: FileEntry) {
  if (entry.type === 'directory') { directory.value = entry.uri; nextCursor.value = ''; void loadFiles(false); return; }
  if (entry.readable === false) { error.value = '此文件不可读取'; return; }
  store.openTab({ kind: 'file', title: entry.name, resourceUri: entry.uri, workspaceUri: workspaceUri.value });
}
async function loadFile(append: boolean) {
  if (!props.tab.resourceUri) return;
  const request = ++sequence; loading.value = true; error.value = '';
  const offset = append ? fileOffset.value : 0;
  try {
    const response = await apiClient.get('/api/workspaces/read', { params: { uri: props.tab.resourceUri, offset, maxBytes: 262144, rawBytes: 1 } });
    if (request !== sequence) return;
    const next = parseWorkbenchDocumentChunk(response.data, append ? fileContent : null, offset);
    fileContent = next;
    fileIsText.value = next.isText;
    content.value = next.text;
    fileOffset.value = next.offset;
    fileHasMore.value = next.hasMore;
  } catch (cause) { if (request === sequence) error.value = failure(cause); }
  finally { if (request === sequence) loading.value = false; }
}
function changeGitWorkspace() {worktreeReview.value=null;checkpoint.value=null;void loadGit();}
async function loadGit() {
  worktreeReview.value = null; checkpoint.value = null; mergePlan.value = null; mergeAudit.value = null;
  if (!workspaceUri.value) { gitStatus.value = null; changes.value = []; commits.value = []; return; }
  const request = ++sequence; loading.value = true; error.value = '';
  try {
    const [status, log, treeList] = await Promise.all([
      apiClient.post('/api/workspaces/git/status', { workspaceUri: workspaceUri.value, includeIgnored: false }),
      apiClient.post('/api/workspaces/git/log', { workspaceUri: workspaceUri.value, limit: 50 }),
      apiClient.post('/api/workspaces/git/worktrees', { workspaceUri: workspaceUri.value }),
    ]);
    if (request !== sequence) return;
    gitStatus.value = status.data;
    changes.value = Array.isArray(status.data?.entries) ? status.data.entries : [];
    commits.value = Array.isArray(log.data?.entries) ? log.data.entries : [];
    worktrees.value = Array.isArray(treeList.data?.entries) ? treeList.data.entries : [];
    if (props.tab.kind === 'diff' && props.tab.resourceUri) await loadDiff(props.tab.resourceUri, request);
  } catch (cause) { if (request === sequence) error.value = failure(cause); }
  finally { if (request === sequence) loading.value = false; }
}
async function loadDiff(path: string, request: number) {
  const response = await apiClient.post('/api/workspaces/git/diff', { workspaceUri: workspaceUri.value, mode: 'worktree', base: '', target: '', paths: [path], maxBytes: 2097152 });
  if (request !== sequence) return;
  diffLineLimit.value = 3500;
  content.value = (response.data?.files || []).map((item: any) => item.patch || '[二进制内容或无变更]').join('\n');
  diffStats.value = response.data?.stats || null;
  diffTruncated.value = response.data?.truncated === true;
}
async function inspectCommit(commit: GitCommit) {
  selectedCommit.value = commit;
  commitPatch.value = '';
  commitDiffStats.value = null;
  commitDetailsMessage.value = '正在加载提交变更…';
  const parent = commit.parents?.[0];
  if (!parent) { commitDetailsMessage.value = '根提交没有父提交，当前 Diff 接口无法直接比较根提交。'; return; }
  const request = ++sequence;
  try {
    const response = await apiClient.post('/api/workspaces/git/diff', { workspaceUri: workspaceUri.value, mode: 'worktree', base: parent, target: commit.hash, paths: [], maxBytes: 2097152 });
    if (request !== sequence) return;
    commitPatch.value = (response.data?.files || []).map((file: any) => file.patch || '[二进制文件]').join('\n');
    commitDiffStats.value = response.data?.stats || null;
    commitDetailsMessage.value = response.data?.truncated ? '部分提交变更已被截断' : '该提交没有可展示的文本差异';
  } catch (cause) { if (request === sequence) commitDetailsMessage.value = failure(cause); }
}
async function createWorktree() {
  if (!workspaceUri.value || worktreeBusy.value) return;
  if (newWorktreeTaskId.value && !/^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$/.test(store.conversationId)) { worktreeError.value = '请先建立正式对话，再关联任务 ID'; return; }
  worktreeBusy.value = true;
  worktreeError.value = '';
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/create', {
      workspaceUri: workspaceUri.value,
      name: newWorktreeName.value,
      branch: newWorktreeBranch.value,
      ref: newWorktreeRef.value || 'HEAD',
      ...(newWorktreeTaskId.value ? { taskId: newWorktreeTaskId.value, conversationId: store.conversationId } : {}),
    });
    if (!response.data?.workspaceUri) throw new Error('服务端未注册 Worktree 工作区');
    newWorktreeName.value = ''; newWorktreeBranch.value = ''; newWorktreeRef.value = ''; newWorktreeTaskId.value = '';
    store.openTab({ kind: 'files', title: response.data.branch || 'Worktree', workspaceUri: response.data.workspaceUri, resourceUri: response.data.workspaceUri });
    await loadGit();
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
async function reviewWorktree(tree: GitWorktree) {
  if (!tree.id || !tree.taskBinding || worktreeBusy.value) return;
  worktreeBusy.value = true; worktreeError.value = ''; worktreeReview.value = null; checkpoint.value = null; mergePlan.value = null; mergeAudit.value = null;
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/review', {
      workspaceUri: workspaceUri.value, id: tree.id, expectedHead: tree.head,
      taskId: tree.taskBinding.taskId, conversationId: tree.taskBinding.conversationId,
    });
    worktreeReview.value = response.data as GitWorktreeReview;
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
function checkpointArguments() {
  const tree = worktreeReview.value?.entry;
  if (!tree?.id || !tree.taskBinding || !workspaceUri.value) throw new Error('请先审查有效的任务工作树');
  return { workspaceUri: workspaceUri.value, id: tree.id, taskId: tree.taskBinding.taskId, conversationId: tree.taskBinding.conversationId };
}
async function previewApprovedMerge() {
  if (!checkpoint.value?.approvedAt || worktreeBusy.value) return;
  mergePlan.value = null; mergeAudit.value = null; worktreeBusy.value = true; worktreeError.value = '';
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/merges/preview', checkpointArguments());
    mergePlan.value = response.data as GitMergePlan;
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
async function readMergeAudit() {
  if (worktreeBusy.value) return;
  worktreeBusy.value = true; worktreeError.value = '';
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/merges/audit', checkpointArguments());
    mergeAudit.value = response.data as GitMergeAudit;
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
async function applyApprovedMerge() {
  const plan = mergePlan.value;
  if (!plan || !checkpoint.value?.approvedAt || worktreeBusy.value) return;
  const answer = window.prompt(`即将把 ${plan.sourceHead.slice(0, 12)} 快进合并到本机 ${plan.targetBranch} (${plan.targetHead.slice(0, 12)})。请准确输入目标分支名称确认：`);
  if (answer !== plan.targetBranch) return;
  worktreeBusy.value = true; worktreeError.value = '';
  mergePlan.value = null;
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/merges/apply', { ...checkpointArguments(), planId: plan.planId }, { timeout: 50000 });
    mergeAudit.value = response.data as GitMergeAudit;
    await loadGit();
    mergeAudit.value = response.data as GitMergeAudit;
  } catch (cause) { worktreeError.value = `${failure(cause)}；如果合并操作已启动，请先核对目标 HEAD 和合并审计，禁止盲目重试。`; }
  finally { worktreeBusy.value = false; }
}
async function prepareCheckpoint() {
  if (worktreeBusy.value || worktreeReview.value?.reviewIncomplete) return;
  worktreeBusy.value = true; worktreeError.value = ''; mergePlan.value = null;
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/checkpoints/prepare', checkpointArguments());
    checkpoint.value = response.data as GitCheckpoint;
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
async function runCheckpointTest() {
  if (!checkpoint.value || worktreeBusy.value) return;
  if (!window.confirm(`确认在此工作树运行 ${checkpointProfile.value}？测试代码可执行仓库内的程序，最长 90 秒。`)) return;
  worktreeBusy.value = true; worktreeError.value = '';
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/checkpoints/test', { ...checkpointArguments(), revision: checkpoint.value.revision, profile: checkpointProfile.value }, { timeout: 105000 });
    checkpoint.value = response.data as GitCheckpoint;
  } catch (cause) { worktreeError.value = failure(cause); checkpoint.value = null; }
  finally { worktreeBusy.value = false; }
}
async function approveCheckpoint() {
  if (!checkpoint.value || worktreeBusy.value) return;
  if (!window.confirm('确认人工批准当前提交及测试证据？这将创建不可自动撤销的审批记录，但不会合并代码。')) return;
  worktreeBusy.value = true; worktreeError.value = '';
  try {
    const response = await apiClient.post('/api/workspaces/git/worktrees/checkpoints/approve', { ...checkpointArguments(), revision: checkpoint.value.revision });
    checkpoint.value = response.data as GitCheckpoint;
  } catch (cause) { worktreeError.value = failure(cause); checkpoint.value = null; }
  finally { worktreeBusy.value = false; }
}
function openWorktreeTerminal(tree: GitWorktree) {
  if (!tree.workspaceUri || !tree.managed) return;
  store.openTab({ kind: 'terminal', workspaceUri: tree.workspaceUri, taskId: tree.taskBinding?.taskId, title: tree.taskBinding?.taskId ? `任务终端 · ${tree.taskBinding.taskId.slice(0, 24)}` : 'Worktree 终端', forceNew: true });
}
function openWorktree(tree: GitWorktree) {
  if (!tree.workspaceUri) return;
  store.openTab({ kind: 'files', title: tree.branch || 'Worktree', workspaceUri: tree.workspaceUri, resourceUri: tree.workspaceUri });
}
async function releaseWorktree(tree: GitWorktree) {
  if (!tree.id || !tree.taskBinding || worktreeBusy.value) return;
  if (!window.confirm(`解除任务「${tree.taskBinding.taskId}」的 Worktree 绑定？不会删除文件或 Git 分支。`)) return;
  worktreeBusy.value = true; worktreeError.value = '';
  try {
    await apiClient.post('/api/workspaces/git/worktrees/release', { workspaceUri: workspaceUri.value, id: tree.id, taskId: tree.taskBinding.taskId, conversationId: tree.taskBinding.conversationId });
    await loadGit();
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
async function removeWorktree(tree: GitWorktree) {
  if (!tree.managed || !tree.id || worktreeBusy.value) return;
  if (!window.confirm(`安全删除 Worktree「${tree.branch || tree.id}」？服务端将拒绝删除有未提交修改的工作树。`)) return;
  worktreeBusy.value = true;
  worktreeError.value = '';
  try {
    await apiClient.post('/api/workspaces/git/worktrees/remove', { workspaceUri: workspaceUri.value, id: tree.id });
    await loadGit();
  } catch (cause) { worktreeError.value = failure(cause); }
  finally { worktreeBusy.value = false; }
}
function openDiff(uri: string) { store.openTab({ kind: 'diff', title: uri.split('/').pop() || 'Diff', resourceUri: uri, workspaceUri: workspaceUri.value }); }
function refresh() { if (props.tab.kind === 'files') void loadFiles(false); else if (props.tab.kind === 'file') void loadFile(false); else if (props.tab.kind === 'git' || props.tab.kind === 'diff') void loadGit(); }
function clearResourceState() {
  sequence++;
  loading.value = false;
  error.value = '';
  mounts.value = [];
  files.value = [];
  directory.value = '';
  nextCursor.value = '';
  content.value = '';
  fileContent = null;
  fileOffset.value = 0;
  fileHasMore.value = false;
  fileIsText.value = true;
  gitStatus.value = null;
  changes.value = [];
  commits.value = [];
  selectedCommit.value = null;
  commitPatch.value = '';
  commitDiffStats.value = null;
  commitDetailsMessage.value = '';
  diffStats.value = null;
  diffTruncated.value = false;
  worktrees.value = [];
  worktreeReview.value = null;
  checkpoint.value = null;
  mergePlan.value = null;
  mergeAudit.value = null;
  worktreeError.value = '';
}
watch(() => [props.tab.id, props.tab.kind, props.tab.workspaceUri, props.tab.resourceUri, props.tab.targetDeviceId], () => {
  clearResourceState();
  workspaceUri.value = props.tab.workspaceUri;
  void initialize();
});
async function initialize() {
  const request = sequence;
  if (props.tab.kind === 'file' && !props.tab.targetDeviceId) { await loadFile(false); return; }
  if (!needsWorkbenchWorkspaceData(props.tab.kind, props.tab.targetDeviceId)) return;
  await loadMounts();
  if (request !== sequence) return;
  if (props.tab.kind === 'files') await loadFiles(false);
  else if (props.tab.kind === 'git' || props.tab.kind === 'diff') await loadGit();
}
let activatedOnce = false;
onActivated(() => {
  if (!activatedOnce) { activatedOnce = true; return; }
  sequence++;
  loading.value = false;
  void initialize();
});
onDeactivated(() => { sequence++; loading.value = false; });
onBeforeUnmount(() => { sequence++; });
void initialize();
</script>
<style scoped>
.resource-view {display:flex;flex-direction:column;gap:8px;min-height:0;height:100%;overflow:auto;padding:12px;box-sizing:border-box;color:var(--el-text-color-primary)}.toolbar {display:flex;align-items:center;gap:6px;min-width:0}.toolbar input,.toolbar select {flex:1;min-width:0;border:1px solid var(--surface-border);padding:8px;border-radius:7px;background:var(--workbench-bg);color:inherit}.toolbar button,.external,.load-more {border:1px solid var(--surface-border);border-radius:7px;background:transparent;color:inherit;padding:7px 10px;cursor:pointer}.toolbar button:disabled {opacity:.4;cursor:not-allowed}.path {font-size:11px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--el-text-color-secondary)}.entry {padding:10px 6px;display:flex;gap:10px;align-items:center;width:100%;text-align:left;border:0;border-bottom:1px solid var(--surface-border);background:transparent;color:inherit;cursor:pointer;overflow-wrap:anywhere}.entry span:nth-child(2){flex:1}.entry small{flex-shrink:0;color:var(--el-text-color-secondary)}.entry:hover{background:var(--el-fill-color-light)}.muted{font-size:12px;color:var(--el-text-color-secondary);padding:12px 0}.status{font-size:12px;color:var(--el-text-color-secondary)}.error,.conflict{color:var(--el-color-danger)}.code{flex:1;min-height:0;background:var(--el-fill-color-light);padding:12px;white-space:pre;overflow:auto;font-size:12px;line-height:1.5;border-radius:8px}.summary,.section-title{font-size:13px;font-weight:600;padding:8px 0}.summary small{font-weight:400}.section-title{border-bottom:1px solid var(--surface-border)}.commit{padding:10px 0;display:flex;flex-direction:column;gap:3px;border-bottom:1px solid var(--surface-border);font-size:12px}.commit small{color:var(--el-text-color-secondary)}.commit-button{border:0;width:100%;background:transparent;text-align:left;color:inherit;cursor:pointer}.commit-button:hover{background:var(--el-fill-color-light)}.commit-detail{padding:12px;border:1px solid var(--surface-border);border-radius:7px;display:flex;flex-direction:column;gap:6px}.commit-detail .code{flex:none;max-height:340px}.added{color:var(--el-color-success)}.removed{color:var(--el-color-danger)}.hunk{color:var(--el-color-primary)}iframe{width:100%;flex:1;min-height:220px;border:1px solid var(--surface-border);border-radius:8px}.external{align-self:flex-start}
</style>

<style scoped>
.worktree-controls{flex-wrap:wrap}.worktree-controls input{flex:1 1 110px}.worktree-item{display:flex;gap:8px;padding:9px;border:1px solid var(--surface-border);border-radius:8px;align-items:center}.worktree-description{display:flex;flex-direction:column;gap:3px;min-width:0;flex:1}.worktree-description small{font-size:11px;color:var(--el-text-color-secondary)}.worktree-item button{flex-shrink:0;padding:6px;border:1px solid var(--surface-border);border-radius:5px;background:transparent;color:inherit;cursor:pointer}.worktree-item button:disabled{opacity:.35;cursor:not-allowed}
</style>
