<template>
  <div class="user-settings-page">
    <header class="page-header">
      <div>
        <h2>用户信息</h2>
        <p>管理你的账户资料与登录安全</p>
      </div>
      <el-button
        type="danger"
        plain
        :icon="SwitchButton"
        :loading="logoutLoading"
        @click="handleLogout"
      >
        退出登录
      </el-button>
    </header>

    <div v-loading="loading" class="settings-content">
      <section class="profile-overview">
        <div class="profile-main">
          <span class="account-avatar">
            <img v-if="appStore.avatar" :src="appStore.avatar" alt="用户头像" />
            <el-icon v-else><UserFilled /></el-icon>
          </span>
          <div class="account-copy">
            <div class="account-name-row">
              <h3>{{ userInfo.username || "管理员" }}</h3>
              <span class="role-badge">{{ roleLabel }}</span>
            </div>
            <p>当前登录账户</p>
          </div>
          <div class="avatar-actions">
            <input
              ref="avatarFileInput"
              class="avatar-file-input"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              @change="handleAvatarChange"
            />
            <el-button
              size="small"
              :loading="avatarUpdating"
              @click="avatarFileInput?.click()"
              >更换头像</el-button
            >
            <el-button
              v-if="appStore.avatar"
              size="small"
              text
              @click="removeAvatar"
              >恢复默认</el-button
            >
          </div>
        </div>

        <div class="profile-edit-grid">
          <el-form label-position="top" class="profile-form" @submit.prevent="saveProfile">
            <div class="profile-field-row">
              <el-form-item label="昵称">
                <el-input v-model="userInfo.nickname" maxlength="100" show-word-limit placeholder="用于界面展示的昵称" />
              </el-form-item>
              <el-form-item label="用户称呼">
                <el-input v-model="userInfo.userLabel" maxlength="100" show-word-limit placeholder="AI 对你的称呼" />
              </el-form-item>
            </div>
            <el-form-item label="个人简介">
              <el-input
                v-model="userInfo.bio"
                type="textarea"
                :rows="3"
                maxlength="1000"
                show-word-limit
                placeholder="可选的个人简介"
              />
            </el-form-item>
            <div class="profile-form-actions">
              <el-button type="primary" native-type="submit" :loading="profileSaving">保存资料</el-button>
            </div>
          </el-form>
        </div>

        <div class="account-meta">
          <div class="meta-item">
            <span class="meta-icon"
              ><el-icon><Key /></el-icon
            ></span>
            <div>
              <span>账户 ID</span>
              <strong>{{ userInfo.id || "—" }}</strong>
            </div>
          </div>
          <div class="meta-item">
            <span class="meta-icon"
              ><el-icon><Calendar /></el-icon
            ></span>
            <div>
              <span>创建时间</span>
              <strong>{{ formatDate(userInfo.createdTime) }}</strong>
            </div>
          </div>
          <div class="meta-item">
            <span class="meta-icon"
              ><el-icon><Clock /></el-icon
            ></span>
            <div>
              <span>最近登录</span>
              <strong>{{ formatDate(userInfo.lastLoginTime) }}</strong>
            </div>
          </div>
        </div>
      </section>

      <section class="security-card">
        <div class="section-heading">
          <span class="section-icon"
            ><el-icon><Lock /></el-icon
          ></span>
          <div>
            <h3>登录与安全</h3>
            <p>定期更新密码有助于保护你的账户</p>
          </div>
        </div>

        <div class="security-layout">
          <el-form
            ref="passwordFormRef"
            :model="passwordForm"
            :rules="passwordRules"
            label-position="top"
            class="password-form"
            @submit.prevent="submitPassword"
          >
            <el-form-item label="当前密码" prop="oldPassword">
              <el-input
                v-model="passwordForm.oldPassword"
                type="password"
                show-password
                autocomplete="current-password"
                placeholder="请输入当前密码"
              />
            </el-form-item>
            <div class="new-password-row">
              <el-form-item label="新密码" prop="newPassword">
                <el-input
                  v-model="passwordForm.newPassword"
                  type="password"
                  show-password
                  autocomplete="new-password"
                  placeholder="至少 6 位"
                />
              </el-form-item>
              <el-form-item label="确认新密码" prop="confirmPassword">
                <el-input
                  v-model="passwordForm.confirmPassword"
                  type="password"
                  show-password
                  autocomplete="new-password"
                  placeholder="再次输入新密码"
                />
              </el-form-item>
            </div>
            <div class="form-actions">
              <el-button type="primary" native-type="submit" :loading="saving"
                >保存新密码</el-button
              >
            </div>
          </el-form>

          <aside class="security-guide">
            <h4>修改密码前</h4>
            <ul>
              <li>
                <el-icon><CircleCheck /></el-icon
                ><span>新密码至少包含 6 个字符</span>
              </li>
              <li>
                <el-icon><CircleCheck /></el-icon
                ><span>避免使用容易猜到的连续字符</span>
              </li>
              <li>
                <el-icon><CircleCheck /></el-icon
                ><span>保存后请使用新密码登录</span>
              </li>
            </ul>
          </aside>
        </div>
      </section>

      <section class="security-card session-card">
        <div class="section-heading session-heading">
          <span class="section-icon"><el-icon><Monitor /></el-icon></span>
          <div class="session-heading-copy">
            <h3>登录设备与 Session</h3>
            <p>查看当前账户的全部登录会话，并可撤销指定设备或批量退出。</p>
          </div>
          <div class="session-actions">
            <el-button size="small" :loading="sessionsLoading" @click="loadSessions">刷新</el-button>
            <el-button size="small" :disabled="sessionBusy" @click="handleRevokeOthers">退出其他设备</el-button>
            <el-button size="small" type="danger" plain :disabled="sessionBusy" @click="handleLogoutAll">退出全部设备</el-button>
          </div>
        </div>

        <el-empty v-if="!sessionsLoading && sessions.length === 0" description="暂无有效登录 Session" :image-size="64" />
        <el-table v-else v-loading="sessionsLoading" :data="sessions" class="session-table" size="small">
          <el-table-column label="设备" min-width="180">
            <template #default="{ row }">
              <div class="session-device">
                <strong>{{ row.deviceName || "未知设备" }}</strong>
                <el-tag v-if="row.current" size="small" type="success">当前设备</el-tag>
              </div>
              <small>{{ row.userAgent || "未记录 User-Agent" }}</small>
            </template>
          </el-table-column>
          <el-table-column prop="ipAddress" label="IP" min-width="130" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }"><el-tag size="small" :type="row.status === 'active' ? 'success' : 'info'">{{ row.status || "unknown" }}</el-tag></template>
          </el-table-column>
          <el-table-column label="最近活动" min-width="150">
            <template #default="{ row }">{{ formatDate(row.lastActiveAt || row.lastRefreshedAt || row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="110" align="right">
            <template #default="{ row }">
              <el-button v-if="!row.current" link type="danger" :loading="sessionBusy === row.sessionId" @click="handleRevokeSession(row)">撤销</el-button>
              <span v-else class="muted-action">本机</span>
            </template>
          </el-table-column>
        </el-table>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import {
  ElMessage,
  ElMessageBox,
  type FormInstance,
  type FormRules,
} from "element-plus";
import {
  Calendar,
  CircleCheck,
  Clock,
  Key,
  Lock,
  Monitor,
  SwitchButton,
  UserFilled,
} from "@element-plus/icons-vue";
import { apiClient } from "@/composables/useApi";
import { forceCleanupSession } from "@/stores/refresh-coordinator";
import {
  listSessions,
  logoutAll,
  revokeOtherSessions,
  revokeSession,
  type SessionInfo,
} from "@/stores/session-client";
import { useAppStore } from "@/stores/app";

type UserInfo = {
  id?: number;
  username: string;
  role: string;
  nickname: string;
  userLabel: string;
  bio: string;
  createdTime: string;
  lastLoginTime: string;
};

const loading = ref(false);
const saving = ref(false);
const profileSaving = ref(false);
const logoutLoading = ref(false);
const avatarUpdating = ref(false);
const sessionsLoading = ref(false);
const sessionBusy = ref<string | boolean>(false);
const sessions = ref<SessionInfo[]>([]);
const avatarFileInput = ref<HTMLInputElement>();
const passwordFormRef = ref<FormInstance>();
const appStore = useAppStore();
const router = useRouter();
const userInfo = reactive<UserInfo>({
  username: "",
  role: "",
  nickname: "",
  userLabel: "",
  bio: "",
  createdTime: "",
  lastLoginTime: "",
});
const passwordForm = reactive({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});

const roleLabel = computed(() =>
  userInfo.role === "admin" ? "管理员" : userInfo.role || "普通用户",
);

const passwordRules: FormRules = {
  oldPassword: [{ required: true, message: "请输入当前密码", trigger: "blur" }],
  newPassword: [
    { required: true, message: "请输入新密码", trigger: "blur" },
    { min: 6, message: "新密码至少 6 位", trigger: "blur" },
  ],
  confirmPassword: [
    { required: true, message: "请再次输入新密码", trigger: "blur" },
    {
      validator: (_rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error("两次输入的新密码不一致"));
          return;
        }
        callback();
      },
      trigger: "blur",
    },
  ],
};

function formatDate(value: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

async function createAvatar(file: File) {
  const sourceUrl = URL.createObjectURL(file);
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => {
      const element = new Image();
      element.onload = () => resolve(element);
      element.onerror = () => reject(new Error("图片读取失败"));
      element.src = sourceUrl;
    });
    const sourceSize = Math.min(image.naturalWidth, image.naturalHeight);
    const sourceX = (image.naturalWidth - sourceSize) / 2;
    const sourceY = (image.naturalHeight - sourceSize) / 2;
    const canvas = document.createElement("canvas");
    canvas.width = 256;
    canvas.height = 256;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("头像处理失败");
    context.drawImage(
      image,
      sourceX,
      sourceY,
      sourceSize,
      sourceSize,
      0,
      0,
      256,
      256,
    );
    return canvas.toDataURL("image/webp", 0.86);
  } finally {
    URL.revokeObjectURL(sourceUrl);
  }
}

async function handleAvatarChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  if (!["image/jpeg", "image/png", "image/webp"].includes(file.type)) {
    ElMessage.warning("请选择 JPG、PNG 或 WebP 图片");
    return;
  }
  if (file.size > 5 * 1024 * 1024) {
    ElMessage.warning("头像图片不能超过 5 MB");
    return;
  }
  avatarUpdating.value = true;
  try {
    appStore.setAvatar(await createAvatar(file));
    ElMessage.success("头像已更新");
  } catch (error: any) {
    ElMessage.error(error?.message || "头像更新失败");
  } finally {
    avatarUpdating.value = false;
  }
}

function removeAvatar() {
  appStore.removeAvatar();
  ElMessage.success("已恢复默认头像");
}

async function handleLogout() {
  try {
    await ElMessageBox.confirm("退出后需要重新登录，确定继续吗？", "退出登录", {
      confirmButtonText: "退出登录",
      cancelButtonText: "取消",
      type: "warning",
      confirmButtonClass: "el-button--danger",
    });
  } catch {
    return;
  }
  logoutLoading.value = true;
  try {
    await apiClient.post("/api/auth/logout");
  } catch {}
  forceCleanupSession();
  await router.replace("/login");
}

async function loadUserInfo() {
  loading.value = true;
  try {
    const res = await apiClient.get("/api/auth/me");
    const data = res.data?.data || res.data;
    Object.assign(userInfo, data || {});
  } catch (error: any) {
    ElMessage.error(error?.message || "用户信息加载失败");
  } finally {
    loading.value = false;
  }
}

async function saveProfile() {
  profileSaving.value = true;
  try {
    const res = await apiClient.put("/api/auth/me", {
      nickname: userInfo.nickname.trim(),
      userLabel: userInfo.userLabel.trim(),
      bio: userInfo.bio.trim(),
    });
    const data = res.data?.data || res.data;
    Object.assign(userInfo, data || {});
    ElMessage.success("用户资料已保存");
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || error?.message || "用户资料保存失败");
  } finally {
    profileSaving.value = false;
  }
}

async function loadSessions() {
  sessionsLoading.value = true;
  try {
    sessions.value = await listSessions();
  } catch (error: any) {
    ElMessage.error(error?.message || "登录 Session 加载失败");
  } finally {
    sessionsLoading.value = false;
  }
}

async function handleRevokeSession(session: SessionInfo) {
  if (!session.sessionId || session.current) return;
  try {
    await ElMessageBox.confirm(`确定撤销「${session.deviceName || "未知设备"}」的登录 Session 吗？`, "撤销 Session", {
      confirmButtonText: "撤销",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch {
    return;
  }
  sessionBusy.value = session.sessionId;
  try {
    await revokeSession(session.sessionId);
    ElMessage.success("Session 已撤销");
    await loadSessions();
  } catch (error: any) {
    ElMessage.error(error?.message || "Session 撤销失败");
  } finally {
    sessionBusy.value = false;
  }
}

async function handleRevokeOthers() {
  try {
    await ElMessageBox.confirm("确定退出当前设备之外的全部登录 Session 吗？", "退出其他设备", {
      confirmButtonText: "退出其他设备",
      cancelButtonText: "取消",
      type: "warning",
    });
  } catch {
    return;
  }
  sessionBusy.value = true;
  try {
    const result = await revokeOtherSessions();
    ElMessage.success(`已撤销 ${result?.revokedCount ?? 0} 个 Session`);
    await loadSessions();
  } catch (error: any) {
    ElMessage.error(error?.message || "退出其他设备失败");
  } finally {
    sessionBusy.value = false;
  }
}

async function handleLogoutAll() {
  try {
    await ElMessageBox.confirm("这会撤销当前账户的全部 Session，并立即回到登录页。确定继续吗？", "退出全部设备", {
      confirmButtonText: "全部退出",
      cancelButtonText: "取消",
      type: "warning",
      confirmButtonClass: "el-button--danger",
    });
  } catch {
    return;
  }
  sessionBusy.value = true;
  try {
    await logoutAll();
  } catch (error: any) {
    ElMessage.error(error?.message || "退出全部设备失败");
    sessionBusy.value = false;
    return;
  }
  forceCleanupSession();
  await router.replace("/login");
}

async function submitPassword() {
  if (!passwordFormRef.value) return;
  const valid = await passwordFormRef.value.validate().catch(() => false);
  if (!valid) return;
  saving.value = true;
  try {
    await apiClient.post("/api/auth/change-password", {
      oldPassword: passwordForm.oldPassword,
      newPassword: passwordForm.newPassword,
    });
    passwordForm.oldPassword = "";
    passwordForm.newPassword = "";
    passwordForm.confirmPassword = "";
    passwordFormRef.value.clearValidate();
    ElMessage.success("密码修改成功");
  } catch (error: any) {
    ElMessage.error(error?.message || "密码修改失败");
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void Promise.all([loadUserInfo(), loadSessions()]);
});
</script>

<style scoped>
.user-settings-page {
  width: 100%;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 22px;
}

.page-header h2 {
  margin: 0;
  color: var(--console-text);
  font-size: 24px;
  font-weight: 650;
}

.page-header p {
  margin: 6px 0 0;
  color: var(--console-text-muted);
  font-size: 13px;
}

.settings-content {
  min-height: 500px;
}

.profile-overview,
.security-card {
  border: 1px solid var(--console-border);
  border-radius: 16px;
  background: var(--ac-color-surface);
}

.profile-overview {
  overflow: hidden;
}

.profile-main {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 24px;
}

.account-avatar {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  flex: 0 0 auto;
  border-radius: 15px;
  background: var(--tp-primary);
  color: var(--tp-text-on-primary);
  font-size: 20px;
  overflow: hidden;
}

.account-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.account-copy {
  min-width: 0;
  flex: 1;
}

.account-name-row {
  display: flex;
  align-items: center;
  gap: 9px;
}

.account-name-row h3 {
  margin: 0;
  overflow: hidden;
  color: var(--console-text);
  font-size: 18px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.role-badge {
  border: 1px solid var(--console-border);
  border-radius: 999px;
  background: var(--ac-color-surface);
  color: var(--console-text-muted);
  font-size: 11px;
  line-height: 1;
}

.role-badge {
  padding: 5px 8px;
}

.account-copy p {
  margin: 5px 0 0;
  color: var(--console-text-muted);
  font-size: 12px;
}

.avatar-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}

.avatar-file-input {
  display: none;
}

.profile-edit-grid {
  padding: 0 24px 22px;
}

.profile-form {
  padding: 18px;
  border: 1px solid var(--console-border);
  border-radius: 12px;
  background: var(--el-fill-color-extra-light);
}

.profile-field-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.profile-form-actions {
  display: flex;
  justify-content: flex-end;
}

.account-meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-top: 1px solid var(--console-border-soft);
  background: var(--ac-color-surface);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 17px 20px;
  border-right: 1px solid var(--console-border-soft);
}

.meta-item:last-child {
  border-right: 0;
}

.meta-icon {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  flex: 0 0 auto;
  border-radius: 9px;
  background: var(--ac-color-surface);
  color: var(--console-text-muted);
  font-size: 14px;
}

.meta-item div {
  min-width: 0;
}

.meta-item span,
.meta-item strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-item div > span {
  color: var(--console-text-muted);
  font-size: 11px;
}

.meta-item strong {
  margin-top: 4px;
  color: var(--console-text);
  font-size: 12px;
  font-weight: 550;
}

.security-card {
  margin-top: 18px;
  padding: 24px;
}

.section-heading {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.section-icon {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  border-radius: 10px;
  background: var(--tp-primary-soft);
  color: var(--tp-primary);
  font-size: 17px;
}

.section-heading h3 {
  margin: 0;
  color: var(--console-text);
  font-size: 16px;
  font-weight: 650;
}

.section-heading p {
  margin: 3px 0 0;
  color: var(--console-text-muted);
  font-size: 12px;
}

.security-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.7fr) minmax(220px, 0.8fr);
  gap: 28px;
}

.password-form :deep(.el-form-item__label) {
  color: var(--console-text);
}

.new-password-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.security-guide {
  align-self: start;
  padding: 16px;
  border: 1px solid var(--console-border-soft);
  border-radius: 12px;
  background: var(--ac-color-surface);
}

.security-guide h4 {
  margin: 0 0 13px;
  color: var(--console-text);
  font-size: 13px;
  font-weight: 600;
}

.security-guide ul {
  display: grid;
  gap: 11px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.security-guide li {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--console-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.security-guide li .el-icon {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--tp-success);
}

.form-actions {
  display: flex;
  justify-content: flex-start;
  padding-top: 4px;
}

.session-heading { align-items: flex-start; }
.session-heading-copy { flex: 1; min-width: 0; }
.session-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
.session-table { width: 100%; }
.session-device { display: flex; align-items: center; gap: 8px; min-width: 0; }
.session-device strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.session-table small { display: block; margin-top: 4px; color: var(--console-text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.muted-action { color: var(--console-text-muted); font-size: 12px; }

@media (max-width: 820px) {
  .account-meta,
  .security-layout {
    grid-template-columns: 1fr;
  }

  .meta-item {
    border-right: 0;
    border-bottom: 1px solid var(--console-border-soft);
  }

  .meta-item:last-child {
    border-bottom: 0;
  }
}

@media (max-width: 560px) {
  .profile-field-row {
    grid-template-columns: 1fr;
  }

  .page-header {
    align-items: flex-start;
  }

  .profile-main {
    align-items: center;
    flex-wrap: wrap;
    padding: 18px;
  }

  .avatar-actions {
    width: 100%;
    margin-left: 66px;
  }

  .new-password-row {
    grid-template-columns: 1fr;
    gap: 0;
  }

  .security-card {
    padding: 18px;
  }
}
</style>
