<template>
  <div id="local-profile" class="space-settings" v-loading="loading">
    <header class="space-page-header">
      <h2>资料与头像</h2>
      <p>设置在当前 Space 中展示的个人信息。</p>
    </header>

    <section class="profile-overview" aria-label="头像设置">
      <button type="button" class="avatar-preview" :title="profile.avatar ? '更换头像' : '上传头像'" @click="triggerAvatarUpload">
        <img v-if="profile.avatar" :src="profile.avatar" alt="个人头像" />
        <el-icon v-else :size="30"><UserFilled /></el-icon>
      </button>
      <div class="profile-overview-text">
        <strong>{{ profile.displayName.trim() || "设置你的昵称" }}</strong>
        <span>{{ profile.userLabel.trim() || "个人资料" }}</span>
        <div class="avatar-actions">
          <el-button :loading="avatarProcessing" @click="triggerAvatarUpload">更换头像</el-button>
          <el-button v-if="profile.avatar" text type="danger" @click="profile.avatar = ''">移除</el-button>
        </div>
        <input ref="avatarInputRef" type="file" accept="image/*" hidden @change="handleAvatarFile" />
      </div>
    </section>

    <section class="profile-section" aria-labelledby="profile-section-title">
      <div class="section-heading">
        <h3 id="profile-section-title">基本信息</h3>
        <p>这些信息仅用于你的个人资料展示。</p>
      </div>
      <el-form label-position="top" class="profile-form">
        <div class="profile-field-row">
          <el-form-item label="显示名称">
            <el-input v-model="profile.displayName" maxlength="100" placeholder="请输入昵称" />
          </el-form-item>
          <el-form-item label="用户标签">
            <el-input v-model="profile.userLabel" maxlength="100" placeholder="可选" />
          </el-form-item>
        </div>
        <el-form-item label="个人简介">
          <el-input v-model="profile.bio" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="简单介绍一下自己" />
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
      <div class="profile-actions">
        <span>修改后需要保存才会生效</span>
        <el-button type="primary" :loading="saving || avatarProcessing" @click="saveProfile">保存资料</el-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { UserFilled } from "@element-plus/icons-vue";
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
.space-settings { max-width: 880px; min-width: 0; }
.space-page-header { margin-bottom: 24px; }
.space-page-header h2 { margin: 0; font-size: 23px; font-weight: 650; letter-spacing: -0.025em; }
.space-page-header p, .section-heading p { margin: 7px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.profile-overview { display: flex; align-items: center; gap: 20px; padding: 22px 24px; margin-bottom: 26px; border: 1px solid var(--surface-border); border-radius: 14px; background: var(--surface-bg); }
.avatar-preview { width: 92px; height: 92px; flex: 0 0 auto; display: grid; place-items: center; overflow: hidden; padding: 0; cursor: pointer; border: 1px solid var(--surface-border); border-radius: 24px; background: var(--control-hover-bg); color: var(--text-secondary); }
.avatar-preview:hover { border-color: var(--el-color-primary); }
.avatar-preview img { width: 100%; height: 100%; object-fit: cover; }
.profile-overview-text { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
.profile-overview-text strong { font-size: 18px; overflow-wrap: anywhere; }
.profile-overview-text > span { color: var(--text-secondary); font-size: 13px; }
.avatar-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; margin-top: 8px; }
.profile-section { padding: 24px; border: 1px solid var(--surface-border); border-radius: 14px; background: var(--surface-bg); }
.section-heading { margin-bottom: 20px; }
.section-heading h3 { margin: 0; font-size: 16px; }
.profile-field-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.profile-form :deep(.el-form-item) { min-width: 0; margin-bottom: 20px; }
.profile-advanced { margin-top: 2px; padding: 14px 0; border-top: 1px solid var(--surface-border); }
.profile-advanced summary { cursor: pointer; color: var(--text-secondary); font-size: 13px; }
.profile-advanced .el-form { padding-top: 16px; }
.profile-actions { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 14px; padding-top: 20px; border-top: 1px solid var(--surface-border); }
.profile-actions span { color: var(--text-secondary); font-size: 12px; }
@media (max-width: 700px) { .profile-overview { padding: 18px; gap: 14px; } .profile-section { padding: 18px; } .profile-field-row { grid-template-columns: minmax(0, 1fr); gap: 0; } }
</style>
