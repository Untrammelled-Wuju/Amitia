<template>
  <div class="space-settings">
    <header class="settings-header">
      <div>
        <h2>个人空间</h2>
        <p>资料保存在当前 U-Ai Space 中；设备认证与个人资料相互独立。</p>
      </div>
      <el-button :loading="saving || avatarProcessing" type="primary" @click="saveProfile">保存资料</el-button>
    </header>

    <div class="settings-grid" v-loading="loading">
      <section class="settings-card profile-card">
        <div class="section-heading">
          <el-icon><UserFilled /></el-icon>
          <div>
            <h3>本地资料</h3>
            <p>这里只保存昵称、头像、简介和偏好，不承担认证职责。</p>
          </div>
        </div>

        <el-form label-position="top">
          <el-form-item label="显示名称">
            <el-input v-model="profile.displayName" maxlength="100" placeholder="例如：无拘" />
          </el-form-item>
          <el-form-item label="用户标签">
            <el-input v-model="profile.userLabel" maxlength="100" placeholder="可选" />
          </el-form-item>
          <el-form-item label="简介">
            <el-input v-model="profile.bio" type="textarea" :rows="5" maxlength="1000" show-word-limit />
          </el-form-item>
          <el-form-item label="头像">
            <div class="avatar-setting">
              <button
                type="button"
                class="avatar-preview"
                :title="profile.avatar ? '点击更换头像' : '点击上传头像'"
                @click="triggerAvatarUpload"
              >
                <img v-if="profile.avatar" :src="profile.avatar" alt="个人头像" />
                <span v-else>+</span>
              </button>
              <input
                ref="avatarInputRef"
                type="file"
                accept="image/*"
                hidden
                @change="handleAvatarFile"
              />
              <div class="avatar-actions">
                <el-button :loading="avatarProcessing" @click="triggerAvatarUpload">上传头像</el-button>
                <el-button v-if="profile.avatar" text type="danger" @click="profile.avatar = ''">移除头像</el-button>
              </div>
            </div>
          </el-form-item>
          <el-form-item label="头像地址">
            <el-input v-model="profile.avatar" placeholder="本地文件或可访问的图片地址" />
          </el-form-item>
        </el-form>
      </section>

      <section class="settings-card identity-card">
        <div class="section-heading">
          <el-icon><Connection /></el-icon>
          <div>
            <h3>Space 身份</h3>
            <p>SpaceID 只负责数据归属，不作为密码，也不代表具体设备。</p>
          </div>
        </div>
        <dl class="identity-list">
          <div><dt>Space ID</dt><dd>{{ identity.spaceId || "未读取" }}</dd></div>
          <div><dt>Instance ID</dt><dd>{{ identity.instanceId || "未读取" }}</dd></div>
          <div><dt>运行模式</dt><dd>{{ deploymentLabel }}</dd></div>
        </dl>
      </section>

      <section class="settings-card device-card">
        <div class="section-heading">
          <el-icon><Monitor /></el-icon>
          <div>
            <h3>当前设备</h3>
            <p>DeviceID 用于设备路由；Cloud 认证由 Device Credential 完成。</p>
          </div>
        </div>
        <dl class="identity-list">
          <div><dt>Device ID</dt><dd>{{ mesh.deviceId || "未获取" }}</dd></div>
          <div><dt>Runtime ID</dt><dd>{{ mesh.runtimeId || "未获取" }}</dd></div>
          <div><dt>连接状态</dt><dd>{{ mesh.state || (deployment.mode === "cloud" ? "未配对" : "本地模式") }}</dd></div>
        </dl>
        <div class="device-actions">
          <el-button @click="router.push('/settings/devices')">管理设备</el-button>
          <el-button
            v-if="deployment.mode === 'cloud' && mesh.state && mesh.state !== 'unprovisioned'"
            type="danger"
            plain
            @click="disconnectCloud"
          >
            解除当前设备配对
          </el-button>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { Connection, Monitor, UserFilled } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";
import type { DeploymentModeConfig } from "@/runtime/runtime-types";
import { useAppStore } from "@/stores/app";

const router = useRouter();
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
const identity = reactive({ spaceId: "", instanceId: "" });
const mesh = reactive({ state: "", deviceId: "", runtimeId: "" });
const deployment = reactive<DeploymentModeConfig>({ mode: "local" });

const deploymentLabel = computed(() =>
  deployment.mode === "cloud" ? `云端 · ${deployment.serverURL || "未配置地址"}` : "本地",
);

async function loadAll() {
  loading.value = true;
  try {
    const [profileRes, infoRes, deploymentConfig] = await Promise.all([
      apiClient.get("/api/space/profile"),
      apiClient.get("/api/public/core/info"),
      getDeploymentConfig(),
    ]);
    Object.assign(profile, profileRes.data?.data || profileRes.data || {});
    appStore.setAvatar(String(profile.avatar || ""));
    const info = infoRes.data?.data || infoRes.data || {};
    identity.spaceId = String(info.spaceId || "");
    identity.instanceId = String(info.instanceId || info.cloudId || "");
    Object.assign(deployment, deploymentConfig);
  } catch (error: any) {
    ElMessage.error(error?.message || "个人空间信息加载失败");
  }

  if (window.amitiaDesktop?.getMeshStatus) {
    try {
      const status = await window.amitiaDesktop.getMeshStatus();
      if (status) Object.assign(mesh, status);
    } catch {}
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

async function disconnectCloud() {
  if (!window.amitiaDesktop?.deprovisionMesh) return;
  try {
    await ElMessageBox.confirm(
      "这会删除当前设备保存的 Cloud Device Credential。个人 Space 数据不会被删除。",
      "解除设备配对",
      { confirmButtonText: "解除配对", cancelButtonText: "取消", type: "warning" },
    );
  } catch {
    return;
  }
  try {
    await window.amitiaDesktop.deprovisionMesh();
    mesh.state = "unprovisioned";
    ElMessage.success("当前设备已解除云端配对");
  } catch (error: any) {
    ElMessage.error(error?.message || "解除配对失败");
  }
}

onMounted(loadAll);
</script>

<style scoped>
.space-settings { padding: 24px; max-width: 1180px; margin: 0 auto; }
.settings-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; margin-bottom: 20px; }
.settings-header h2, .section-heading h3 { margin: 0; }
.settings-header p, .section-heading p { margin: 6px 0 0; color: var(--el-text-color-secondary); }
.settings-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.settings-card { padding: 20px; border: 1px solid var(--el-border-color-lighter); border-radius: 14px; background: var(--el-bg-color); }
.profile-card { grid-row: span 2; }
.section-heading { display: flex; gap: 12px; align-items: flex-start; margin-bottom: 18px; }
.section-heading .el-icon { margin-top: 3px; font-size: 20px; }
.avatar-setting { display: flex; align-items: center; gap: 14px; min-height: 88px; }
.avatar-preview { width: 88px; height: 88px; flex: 0 0 auto; padding: 0; border: 1px dashed var(--el-border-color); border-radius: 22px; background: var(--el-fill-color-light); color: var(--el-color-primary); display: grid; place-items: center; overflow: hidden; cursor: pointer; transition: border-color .2s ease, transform .2s ease; }
.avatar-preview:hover { border-color: var(--el-color-primary); transform: translateY(-1px); }
.avatar-preview img { width: 100%; height: 100%; object-fit: cover; }
.avatar-preview span { font-size: 30px; line-height: 1; }
.avatar-actions { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; }
.identity-list { display: grid; gap: 12px; margin: 0; }
.identity-list > div { display: grid; grid-template-columns: 100px 1fr; gap: 12px; }
.identity-list dt { color: var(--el-text-color-secondary); }
.identity-list dd { margin: 0; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; overflow-wrap: anywhere; }
.device-actions { display: flex; gap: 10px; margin-top: 20px; }
@media (max-width: 820px) { .settings-grid { grid-template-columns: 1fr; } .profile-card { grid-row: auto; } .avatar-setting { align-items: flex-start; } }
</style>
