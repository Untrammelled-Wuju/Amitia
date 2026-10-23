<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"
import { ElMessage, ElMessageBox, type UploadRequestOptions } from "element-plus"

import { errorMessage } from "@/api/client"
import {
  createRelease,
  deleteRelease,
  fetchRelease,
  fetchReleases,
  publishRelease,
  rollbackDesktop,
  updateRollout,
  uploadArtifact,
  validateRelease,
} from "@/api/releases"
import type { Channel, CreateReleasePayload, Product, ReleaseArtifact, UpdateRelease } from "@/api/types"
import StatusTag from "@/components/StatusTag.vue"

const props = defineProps<{
  product: Product
}>()

const loading = ref(false)
const releasing = ref(false)
const createVisible = ref(false)
const detailVisible = ref(false)
const rolloutVisible = ref(false)
const selectedRelease = ref<UpdateRelease | null>(null)
const releases = ref<UpdateRelease[]>([])
const filterChannel = ref("")
const filterStatus = ref("")
const uploadProgress = reactive<Record<string, number>>({})

const form = reactive<CreateReleasePayload>({
  product: props.product,
  channel: "stable",
  version: "",
  versionCode: 0,
  releaseName: "",
  releaseNotes: "",
  mandatory: false,
  minSupportedVersionCode: 0,
  rolloutPercentage: 100,
  abi: "arm64-v8a",
})

const rolloutForm = reactive({
  rolloutPercentage: 100,
})

const productLabel = computed(() => (props.product === "desktop" ? "桌面端" : "手机端"))
const publishAction = computed(() => "发布到更新通道")

const uploadKinds = computed(() => {
  if (props.product === "desktop") {
    return [
      { kind: "installer", label: "Windows 安装包", hint: "AmitiaSetup-版本-x64.exe" },
      { kind: "blockmap", label: "差分更新文件", hint: "AmitiaSetup-版本-x64.exe.blockmap" },
    ]
  }
  return [{ kind: "apk", label: "Android Release APK", hint: "arm64-v8a 正式签名 APK" }]
})

const productStats = computed(() => {
  const published = releases.value.filter((item) => item.status === "published").length
  const ready = releases.value.filter((item) => item.status === "ready").length
  const drafts = releases.value.filter((item) => item.status === "draft" || item.status === "failed").length
  return { published, ready, drafts }
})

function resetCreateForm() {
  Object.assign(form, {
    product: props.product,
    channel: "stable",
    version: "",
    versionCode: 0,
    releaseName: "",
    releaseNotes: "",
    mandatory: false,
    minSupportedVersionCode: 0,
    rolloutPercentage: 100,
    abi: "arm64-v8a",
  })
}

async function loadReleases() {
  loading.value = true
  try {
    releases.value = await fetchReleases({
      product: props.product,
      channel: filterChannel.value,
      status: filterStatus.value,
    })
    if (selectedRelease.value) {
      const match = releases.value.find((item) => item.id === selectedRelease.value?.id)
      if (match) selectedRelease.value = await fetchRelease(match.id)
    }
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function submitCreate() {
  if (!form.version.trim()) {
    ElMessage.error("请输入版本号")
    return
  }
  if (props.product === "android" && form.versionCode <= 0) {
    ElMessage.error("请输入大于 0 的 versionCode")
    return
  }
  releasing.value = true
  try {
    const release = await createRelease({ ...form, product: props.product })
    createVisible.value = false
    resetCreateForm()
    ElMessage.success("发布草稿已创建")
    await loadReleases()
    await openDetail(release.id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    releasing.value = false
  }
}

async function openDetail(id: number) {
  try {
    selectedRelease.value = await fetchRelease(id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function artifactByKind(kind: string) {
  return selectedRelease.value?.artifacts.find((item) => item.kind === kind)
}

async function handleUpload(kind: string, options: UploadRequestOptions) {
  if (!selectedRelease.value) return
  const file = options.file
  uploadProgress[kind] = 0
  try {
    await uploadArtifact(selectedRelease.value.id, kind, file, (percent) => {
      uploadProgress[kind] = percent
    })
    options.onSuccess("ok")
    ElMessage.success(`${file.name} 上传完成`)
    selectedRelease.value = await fetchRelease(selectedRelease.value.id)
    await loadReleases()
  } catch (error) {
    options.onError(error as never)
    ElMessage.error(errorMessage(error))
  } finally {
    delete uploadProgress[kind]
  }
}

function beforeUpload(file: File) {
  if (file.size <= 0) {
    ElMessage.error("文件不能为空")
    return false
  }
  return true
}

async function runValidation() {
  if (!selectedRelease.value) return
  try {
    const result = await validateRelease(selectedRelease.value.id)
    if (result.ready) {
      ElMessage.success("校验通过，可以发布")
    } else {
      ElMessage.error(result.errors.join("；"))
    }
    selectedRelease.value = await fetchRelease(selectedRelease.value.id)
    await loadReleases()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function runPublish() {
  if (!selectedRelease.value) return
  await ElMessageBox.confirm(
    `确认将 ${selectedRelease.value.version} 发布到 ${selectedRelease.value.channel} 通道吗？`,
    "确认发布",
    {
      confirmButtonText: "确认发布",
      cancelButtonText: "取消",
      type: "warning",
    },
  )
  try {
    await publishRelease(selectedRelease.value.id)
    ElMessage.success("发布完成")
    selectedRelease.value = await fetchRelease(selectedRelease.value.id)
    await loadReleases()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function runDelete(release: UpdateRelease) {
  await ElMessageBox.confirm(`确认删除 ${release.version} 的发布草稿吗？`, "删除草稿", {
    confirmButtonText: "删除",
    cancelButtonText: "取消",
    type: "warning",
  })
  try {
    await deleteRelease(release.id)
    if (selectedRelease.value?.id === release.id) {
      selectedRelease.value = null
      detailVisible.value = false
    }
    ElMessage.success("已删除")
    await loadReleases()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function openRollout(release: UpdateRelease) {
  selectedRelease.value = release
  rolloutForm.rolloutPercentage = release.rolloutPercentage
  rolloutVisible.value = true
}

async function submitRollout() {
  if (!selectedRelease.value) return
  try {
    await updateRollout(selectedRelease.value.id, rolloutForm.rolloutPercentage)
    rolloutVisible.value = false
    ElMessage.success("灰度配置已更新")
    selectedRelease.value = await fetchRelease(selectedRelease.value.id)
    await loadReleases()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function switchDesktopPointer(release: UpdateRelease) {
  await ElMessageBox.confirm(
    "该操作会把桌面端最新更新指针切换到此版本。已经安装更高版本的客户端不会自动降级。",
    "切换最新指针",
    {
      confirmButtonText: "切换",
      cancelButtonText: "取消",
      type: "warning",
    },
  )
  try {
    await rollbackDesktop(release.id)
    ElMessage.success("桌面端最新指针已切换")
    await loadReleases()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

function formatTime(value: string | null) {
  if (!value) return "-"
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value))
}

function formatBytes(value: number) {
  if (!value) return "-"
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(2)} GB`
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(2)} MB`
  return `${(value / 1024).toFixed(1)} KB`
}

function artifactSize(items: ReleaseArtifact[]) {
  return items.reduce((total, item) => total + item.size, 0)
}

function handleRowDoubleClick(row: UpdateRelease) {
  openDetail(row.id)
}

watch(
  () => props.product,
  () => {
    resetCreateForm()
    selectedRelease.value = null
    detailVisible.value = false
    loadReleases()
  },
)

onMounted(loadReleases)
</script>

<template>
  <div class="page-stack">
    <section class="page-intro">
      <div>
        <h2>{{ productLabel }}更新发布</h2>
        <p>
          {{
            product === "desktop"
              ? "上传安装包和差分文件，校验后写入现有 electron-updater 通道。"
              : "上传正式 APK，服务端生成签名清单并写入现有 Android 更新通道。"
          }}
        </p>
      </div>
      <el-button type="primary" size="large" @click="createVisible = true">
        <el-icon><Plus /></el-icon>
        新建发布
      </el-button>
    </section>

    <section class="compact-stats">
      <div><span>已发布</span><strong>{{ productStats.published }}</strong></div>
      <div><span>待发布</span><strong>{{ productStats.ready }}</strong></div>
      <div><span>草稿与失败</span><strong>{{ productStats.drafts }}</strong></div>
    </section>

    <section class="panel full-panel">
      <div class="panel-heading filter-heading">
        <div>
          <h3>发布记录</h3>
          <p>管理版本、制品、更新说明和发布状态。</p>
        </div>
        <div class="filter-actions">
          <el-select v-model="filterChannel" clearable placeholder="全部通道" style="width: 130px" @change="loadReleases">
            <el-option label="stable" value="stable" />
            <el-option label="beta" value="beta" />
            <el-option label="alpha" value="alpha" />
          </el-select>
          <el-select v-model="filterStatus" clearable placeholder="全部状态" style="width: 130px" @change="loadReleases">
            <el-option label="草稿" value="draft" />
            <el-option label="待发布" value="ready" />
            <el-option label="已发布" value="published" />
            <el-option label="已暂停" value="paused" />
            <el-option label="失败" value="failed" />
          </el-select>
          <el-button @click="loadReleases">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </div>
      <el-table v-loading="loading" :data="releases" size="large" @row-dblclick="handleRowDoubleClick">
        <el-table-column prop="version" label="版本" min-width="160">
          <template #default="{ row }">
            <div class="version-cell">
              <strong>{{ row.version }}</strong>
              <span v-if="row.versionCode">versionCode {{ row.versionCode }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="channel" label="通道" width="90" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <StatusTag :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column label="制品大小" width="110">
          <template #default="{ row }">{{ formatBytes(artifactSize(row.artifacts)) }}</template>
        </el-table-column>
        <el-table-column label="灰度" width="80">
          <template #default="{ row }">{{ row.product === "android" ? `${row.rolloutPercentage}%` : "-" }}</template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="160">
          <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="190" align="right">
          <template #default="{ row }">
            <el-button text type="primary" @click="openDetail(row.id)">管理</el-button>
            <el-button
              v-if="row.product === 'android' && ['published', 'paused'].includes(row.status)"
              text
              @click="openRollout(row)"
            >
              灰度
            </el-button>
            <el-button
              v-if="row.product === 'desktop' && row.status === 'published'"
              text
              @click="switchDesktopPointer(row)"
            >
              切换指针
            </el-button>
            <el-button
              v-if="['draft', 'failed'].includes(row.status)"
              text
              type="danger"
              @click="runDelete(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="createVisible" :title="`新建${productLabel}发布`" width="620px" destroy-on-close>
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item label="更新通道" required>
            <el-select v-model="form.channel" style="width: 100%">
              <el-option label="stable" value="stable" />
              <el-option label="beta" value="beta" />
              <el-option v-if="product === 'android'" label="alpha" value="alpha" />
            </el-select>
          </el-form-item>
          <el-form-item label="版本号" required>
            <el-input v-model="form.version" placeholder="例如 26.2.0-beta.2" />
          </el-form-item>
          <el-form-item v-if="product === 'android'" label="versionCode" required>
            <el-input-number v-model="form.versionCode" :min="1" :step="1" style="width: 100%" />
          </el-form-item>
          <el-form-item label="发布名称">
            <el-input v-model="form.releaseName" placeholder="留空时自动生成" />
          </el-form-item>
        </div>
        <el-form-item label="更新说明">
          <el-input
            v-model="form.releaseNotes"
            type="textarea"
            :rows="5"
            placeholder="填写面向用户的本次更新内容"
          />
        </el-form-item>
        <div v-if="product === 'android'" class="form-grid">
          <el-form-item label="灰度比例">
            <el-slider v-model="form.rolloutPercentage" :min="0" :max="100" show-input />
          </el-form-item>
          <el-form-item label="最低支持 versionCode">
            <el-input-number v-model="form.minSupportedVersionCode" :min="0" :max="form.versionCode" style="width: 100%" />
          </el-form-item>
        </div>
        <el-checkbox v-if="product === 'android'" v-model="form.mandatory">强制更新</el-checkbox>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" :loading="releasing" @click="submitCreate">创建草稿</el-button>
      </template>
    </el-dialog>

    <el-drawer v-model="detailVisible" size="620px" :title="selectedRelease ? `${selectedRelease.version} 发布详情` : '发布详情'">
      <div v-if="selectedRelease" class="detail-stack">
        <section class="detail-summary">
          <div>
            <span>当前状态</span>
            <StatusTag :status="selectedRelease.status" />
          </div>
          <div>
            <span>产品与通道</span>
            <strong>{{ selectedRelease.product === "desktop" ? "桌面端" : "手机端" }} / {{ selectedRelease.channel }}</strong>
          </div>
          <div>
            <span>{{ selectedRelease.product === "android" ? "versionCode" : "发布版本" }}</span>
            <strong>{{ selectedRelease.product === "android" ? selectedRelease.versionCode : selectedRelease.version }}</strong>
          </div>
          <div>
            <span>发布时间</span>
            <strong>{{ formatTime(selectedRelease.publishedAt) }}</strong>
          </div>
        </section>

        <section class="drawer-section">
          <div class="section-title">
            <div>
              <h3>发布制品</h3>
              <p>服务端会对实际文件重新计算 SHA-256 和 SHA-512。</p>
            </div>
          </div>
          <div class="artifact-list">
            <article v-for="item in uploadKinds" :key="item.kind" class="artifact-item">
              <div class="artifact-copy">
                <div>
                  <strong>{{ item.label }}</strong>
                  <span>{{ item.hint }}</span>
                </div>
                <template v-if="artifactByKind(item.kind)">
                  <code>{{ artifactByKind(item.kind)?.originalName }}</code>
                  <small>
                    {{ formatBytes(artifactByKind(item.kind)?.size || 0) }} ·
                    SHA-256 {{ artifactByKind(item.kind)?.sha256.slice(0, 16) }}…
                  </small>
                </template>
                <span v-else class="empty-artifact">尚未上传</span>
              </div>
              <el-upload
                :show-file-list="false"
                :http-request="(options: UploadRequestOptions) => handleUpload(item.kind, options)"
                :before-upload="beforeUpload"
                :disabled="['published', 'paused'].includes(selectedRelease.status)"
              >
                <el-button :loading="uploadProgress[item.kind] !== undefined" :disabled="['published', 'paused'].includes(selectedRelease.status)">
                  <el-icon><Upload /></el-icon>
                  {{ artifactByKind(item.kind) ? "替换" : "上传" }}
                </el-button>
              </el-upload>
              <el-progress
                v-if="uploadProgress[item.kind] !== undefined"
                class="artifact-progress"
                :percentage="uploadProgress[item.kind]"
              />
            </article>
          </div>
        </section>

        <section class="drawer-section">
          <div class="section-title">
            <div>
              <h3>更新说明</h3>
              <p>发布后会写入桌面端 latest.yml、beta.yml 或 Android 通道清单。</p>
            </div>
          </div>
          <div class="release-notes">{{ selectedRelease.releaseNotes || "暂无更新说明" }}</div>
        </section>

        <section v-if="selectedRelease.product === 'android'" class="detail-summary two-column">
          <div>
            <span>强制更新</span>
            <strong>{{ selectedRelease.mandatory ? "是" : "否" }}</strong>
          </div>
          <div>
            <span>灰度比例</span>
            <strong>{{ selectedRelease.rolloutPercentage }}%</strong>
          </div>
          <div>
            <span>最低支持版本</span>
            <strong>{{ selectedRelease.minSupportedVersionCode }}</strong>
          </div>
          <div>
            <span>ABI</span>
            <strong>{{ selectedRelease.abi }}</strong>
          </div>
        </section>
      </div>
      <template #footer>
        <div v-if="selectedRelease" class="drawer-footer">
          <el-button @click="detailVisible = false">关闭</el-button>
          <el-button
            v-if="['draft', 'failed', 'ready'].includes(selectedRelease.status)"
            @click="runValidation"
          >
            校验制品
          </el-button>
          <el-button
            v-if="['draft', 'failed', 'ready'].includes(selectedRelease.status)"
            type="primary"
            @click="runPublish"
          >
            {{ publishAction }}
          </el-button>
          <el-button
            v-if="selectedRelease.product === 'android' && ['published', 'paused'].includes(selectedRelease.status)"
            type="primary"
            @click="openRollout(selectedRelease)"
          >
            调整灰度
          </el-button>
        </div>
      </template>
    </el-drawer>

    <el-dialog v-model="rolloutVisible" title="调整手机端灰度" width="440px">
      <p class="dialog-note">设置为 0% 可暂停新设备获取该版本，已安装用户不会被降级。</p>
      <el-slider v-model="rolloutForm.rolloutPercentage" :min="0" :max="100" show-input />
      <template #footer>
        <el-button @click="rolloutVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRollout">保存灰度</el-button>
      </template>
    </el-dialog>
  </div>
</template>
