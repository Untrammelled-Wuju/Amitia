<template>
  <div id="local-profile" class="space-settings" v-loading="loading">
    <section class="profile-overview" aria-label="个人资料概览">
      <button type="button" class="avatar-preview" :title="profile.avatar ? '更换头像' : '上传头像'" @click="triggerAvatarUpload">
        <img v-if="profile.avatar" :src="profile.avatar" alt="个人头像" />
        <el-icon v-else :size="30"><UserFilled /></el-icon>
        <span class="avatar-edit-hint">更换</span>
      </button>
      <div class="profile-overview-text">
        <div class="profile-identity-line">
          <strong>{{ profile.displayName.trim() || "设置你的昵称" }}</strong>
          <el-tag size="small" effect="plain" :type="profileCompletion === 4 ? 'success' : 'info'">
            {{ profileCompletion === 4 ? "资料已完善" : "待完善" }}
          </el-tag>
        </div>
        <span>{{ profile.userLabel.trim() || "还没有设置用户标签" }}</span>
        <p>{{ profile.bio.trim() || "添加一段个人简介，让角色更了解你。" }}</p>
        <div class="avatar-actions">
          <el-button :loading="avatarProcessing" @click="triggerAvatarUpload">更换头像</el-button>
          <el-button v-if="profile.avatar" text type="danger" @click="profile.avatar = ''">移除</el-button>
        </div>
        <input ref="avatarInputRef" type="file" accept="image/*" hidden @change="handleAvatarFile" />
      </div>
      <div class="profile-completion">
        <strong>{{ profileCompletion }}<small>/4</small></strong>
        <span>资料完整度</span>
        <div class="profile-completion-track"><i :style="{ width: `${profileCompletion * 25}%` }"></i></div>
      </div>
    </section>

    <div class="profile-layout">
      <section class="profile-section" aria-labelledby="profile-section-title">
        <div class="section-heading">
          <div class="section-heading-icon"><el-icon><UserFilled /></el-icon></div>
          <div>
            <h3 id="profile-section-title">基本信息</h3>
            <p>这些内容会用于个人资料展示，不会修改角色的独立设定。</p>
          </div>
        </div>
        <el-form label-position="top" class="profile-form">
          <div class="profile-field-row">
            <el-form-item label="显示名称">
              <el-input v-model="profile.displayName" maxlength="100" placeholder="请输入昵称" />
            </el-form-item>
            <el-form-item label="用户标签">
              <el-input v-model="profile.userLabel" maxlength="100" placeholder="例如：开发者、写作、游戏" />
            </el-form-item>
          </div>
          <el-form-item label="个人简介">
            <el-input v-model="profile.bio" type="textarea" :rows="5" maxlength="1000" show-word-limit placeholder="简单介绍一下自己" />
          </el-form-item>
        </el-form>
        <details class="profile-advanced">
          <summary>高级头像设置</summary>
          <el-form label-position="top">
            <el-form-item label="头像地址">
              <el-input v-model="profile.avatar" placeholder="本地文件或图片地址" />
            </el-form-item>
          </el-form>
        </details>
      </section>

      <aside class="profile-aside">
        <section class="profile-summary-card">
          <div class="summary-card-heading">
            <el-icon><CircleCheck /></el-icon>
            <h3>资料状态</h3>
          </div>
          <dl>
            <div><dt>显示名称</dt><dd>{{ profile.displayName.trim() ? "已填写" : "未填写" }}</dd></div>
            <div><dt>用户标签</dt><dd>{{ profile.userLabel.trim() ? "已填写" : "未填写" }}</dd></div>
            <div><dt>个人简介</dt><dd>{{ profile.bio.trim() ? "已填写" : "未填写" }}</dd></div>
            <div><dt>个人头像</dt><dd>{{ profile.avatar ? "已设置" : "未设置" }}</dd></div>
          </dl>
        </section>
        <section class="profile-actions">
          <div>
            <strong>保存修改</strong>
            <span>修改后需要保存才会同步到个人空间。</span>
          </div>
          <el-button type="primary" :loading="saving || avatarProcessing" @click="saveProfile">保存资料</el-button>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { CircleCheck, UserFilled } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { useAppStore } from "@/stores/app";

const appStore = useAppStore();
const loading = ref(false);
const saving = ref(false);
const avatarProcessing = ref(false);
const avatarInputRef = ref<HTMLInputElement>();
const profile = reactive({
  schemaVersion: 1,
  displayName: "",
  avatar: "",
  bio: "",
  userLabel: "",
  preferences: {} as Record<string, unknown>,
});
const profileCompletion = computed(() =>
  [profile.displayName, profile.userLabel, profile.bio, profile.avatar].filter((value) => value.trim()).length
);
async function loadAll() {
  loading.value = true;
  try {
    const profileRes = await apiClient.get("/api/space/profile");
    Object.assign(profile, profileRes.data?.data || profileRes.data || {});
    appStore.setAvatar(String(profile.avatar || ""));
  } catch (error: any) {
    ElMessage.error(error?.message || "个人空间信息加载失败");
  }

  loading.value = false;
}

async function saveProfile() {
  saving.value = true;
  try {
    const res = await apiClient.put("/api/space/profile", {
      schemaVersion: 1,
      displayName: profile.displayName.trim(),
      avatar: profile.avatar.trim(),
      bio: profile.bio.trim(),
      userLabel: profile.userLabel.trim(),
      preferences: profile.preferences || {},
    });
    Object.assign(profile, res.data?.data || res.data || profile);
    appStore.setAvatar(String(profile.avatar || ""));
    ElMessage.success("个人资料已保存");
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || error?.message || "个人资料保存失败");
  } finally {
    saving.value = false;
  }
}

function triggerAvatarUpload() {
  avatarInputRef.value?.click();
}

async function handleAvatarFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  if (!file.type.startsWith("image/")) {
    ElMessage.warning("请选择图片文件");
    return;
  }
  if (file.size > 3 * 1024 * 1024) {
    ElMessage.warning("头像图片不能超过 3 MB");
    return;
  }
  avatarProcessing.value = true;
  try {
    profile.avatar = await resizeAvatar(file);
    ElMessage.success("头像已载入，保存资料后生效");
  } catch (error: any) {
    ElMessage.error(error?.message || "头像处理失败");
  } finally {
    avatarProcessing.value = false;
  }
}

function resizeAvatar(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();
    const cleanup = () => URL.revokeObjectURL(objectUrl);
    image.onload = () => {
      try {
        const maxSide = 1024;
        const scale = Math.min(1, maxSide / Math.max(image.naturalWidth, image.naturalHeight));
        const width = Math.max(1, Math.round(image.naturalWidth * scale));
        const height = Math.max(1, Math.round(image.naturalHeight * scale));
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext("2d");
        if (!context) throw new Error("当前环境无法处理头像图片");
        context.drawImage(image, 0, 0, width, height);
        cleanup();
        resolve(canvas.toDataURL(file.type === "image/png" ? "image/png" : "image/jpeg", 0.85));
      } catch (error) {
        cleanup();
        reject(error);
      }
    };
    image.onerror = () => {
      cleanup();
      reject(new Error("无法解析所选头像图片"));
    };
    image.src = objectUrl;
  });
}

onMounted(loadAll);
</script>

<style scoped>
.space-settings { width: min(100%, 1040px); min-width: 0; display: grid; gap: 20px; }
.section-heading p { margin: 7px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.profile-overview { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 24px; padding: 28px; border-radius: 24px; background: #2C2C2E; }
.avatar-preview { position: relative; width: 104px; height: 104px; flex: 0 0 auto; display: grid; place-items: center; overflow: hidden; padding: 0; cursor: pointer; border: 0; border-radius: 28px; background: color-mix(in srgb, var(--workbench-bg) 72%, var(--ac-color-primary)); color: var(--text-secondary); }
.avatar-preview:hover { background: color-mix(in srgb, var(--workbench-bg) 62%, var(--ac-color-primary)); }
.avatar-preview img { width: 100%; height: 100%; object-fit: cover; }
.avatar-edit-hint { position: absolute; inset: auto 6px 6px; padding: 3px 7px; border-radius: 999px; background: color-mix(in srgb, var(--workbench-bg) 82%, transparent); color: var(--text-primary); font-size: 10px; opacity: 0; transition: opacity var(--ac-transition-fast); }
.avatar-preview:hover .avatar-edit-hint { opacity: 1; }
.profile-overview-text { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.profile-identity-line { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; }
.profile-overview-text strong { font-size: 21px; overflow-wrap: anywhere; }
.profile-overview-text > span { color: var(--text-secondary); font-size: 13px; }
.profile-overview-text p { max-width: 620px; margin: 2px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.65; overflow-wrap: anywhere; }
.avatar-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; margin-top: 8px; }
.profile-completion { min-width: 118px; display: grid; justify-items: end; gap: 5px; align-self: stretch; align-content: center; padding-left: 22px; }
.profile-completion strong { font-size: 28px; font-weight: 680; letter-spacing: -0.04em; }
.profile-completion strong small { color: var(--text-secondary); font-size: 13px; font-weight: 500; }
.profile-completion span { color: var(--text-secondary); font-size: 12px; }
.profile-completion-track { width: 100%; height: 4px; overflow: hidden; border-radius: 999px; background: color-mix(in srgb, var(--workbench-bg) 72%, transparent); }
.profile-completion-track i { display: block; height: 100%; border-radius: inherit; background: var(--ac-color-primary); transition: width var(--ac-transition-fast); }
.profile-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(250px, 320px); align-items: start; gap: 18px; }
.profile-section { padding: 26px; border-radius: 20px; background: var(--surface-bg-elevated); }
.section-heading { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 22px; }
.section-heading-icon { width: 34px; height: 34px; flex: 0 0 auto; display: grid; place-items: center; border-radius: 10px; background: var(--control-hover-bg); color: var(--ac-color-primary); }
.section-heading h3 { margin: 0; font-size: 16px; }
.profile-field-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.profile-form :deep(.el-form-item) { min-width: 0; margin-bottom: 20px; }
.profile-advanced { margin-top: 2px; padding: 14px 0 0; }
.profile-advanced summary { cursor: pointer; color: var(--text-secondary); font-size: 13px; }
.profile-advanced .el-form { padding-top: 16px; }
.profile-aside { display: grid; gap: 14px; }
.profile-summary-card { padding: 22px; border-radius: 18px; background: var(--surface-bg-elevated); }
.summary-card-heading { display: flex; align-items: center; gap: 8px; margin-bottom: 16px; color: var(--ac-color-primary); }
.summary-card-heading h3 { margin: 0; color: var(--text-primary); font-size: 14px; }
.profile-summary-card dl { display: grid; gap: 0; margin: 0; }
.profile-summary-card dl div { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 9px 0; }
.profile-summary-card dt { color: var(--text-secondary); font-size: 12px; }
.profile-summary-card dd { margin: 0; color: var(--text-primary); font-size: 12px; font-weight: 560; }
.profile-actions { display: grid; gap: 16px; padding: 4px 2px 0; }
.profile-actions div { display: grid; gap: 5px; }
.profile-actions strong { font-size: 14px; }
.profile-actions span { color: var(--text-secondary); font-size: 12px; line-height: 1.55; }
.profile-actions .el-button { width: 100%; }
@media (max-width: 900px) { .profile-layout { grid-template-columns: minmax(0, 1fr); } .profile-aside { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 700px) { .profile-overview { grid-template-columns: auto minmax(0, 1fr); padding: 20px; gap: 16px; } .profile-completion { grid-column: 1 / -1; justify-items: stretch; padding: 14px 0 0; } .profile-section { padding: 20px; } .profile-aside { grid-template-columns: minmax(0, 1fr); } .profile-field-row { grid-template-columns: minmax(0, 1fr); gap: 0; } }
</style>
