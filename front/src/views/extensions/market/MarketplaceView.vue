<template>
  <div class="marketplace-page">
    <ExtensionPageHeader
      title="插件市场"
      description="按分类发现、搜索、安装和更新可信插件。"
      parent-title="扩展中心"
      parent-path="/extensions"
    >
      <template #actions>
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </template>
    </ExtensionPageHeader>

    <div class="market-toolbar">
      <div class="category-strip">
        <button :class="{ active: category === '' }" @click="selectCategory('')">全部</button>
        <button
          v-for="item in categories"
          :key="item.id"
          :class="{ active: category === item.id }"
          @click="selectCategory(item.id)"
        >
          {{ item.name }}
        </button>
      </div>
      <el-input
        v-model="search"
        clearable
        placeholder="搜索名称、扩展 ID、发布者或能力"
        class="market-search"
        @keyup.enter="load"
        @clear="load"
      >
        <template #append>
          <el-button @click="load">搜索</el-button>
        </template>
      </el-input>
    </div>

    <el-alert v-if="error" type="error" show-icon :closable="false" :title="error" />

    <div v-loading="loading" class="market-grid">
      <article v-for="item in listings" :key="item.listing.id" class="market-card">
        <div class="market-card__head">
          <span class="market-icon">{{ item.listing.displayName.slice(0, 1) }}</span>
          <div class="market-title">
            <h2>{{ item.listing.displayName }}</h2>
            <p>{{ item.listing.extensionId }}</p>
          </div>
          <el-tag :type="trustType(item.listing.trustLevel)">{{ trustLabel(item.listing.trustLevel) }}</el-tag>
        </div>
        <p class="market-summary">{{ item.listing.summary }}</p>
        <div class="market-meta">
          <span>{{ item.category?.name || item.listing.categoryId }}</span>
          <span>v{{ item.listing.latestVersion }}</span>
          <span>{{ item.listing.installCount }} 次安装</span>
          <span>安全分 {{ item.listing.securityScore }}</span>
        </div>
        <div class="market-actions">
          <el-button text @click="openDetails(item)">详情</el-button>
          <el-button type="primary" :disabled="!item.installable || installedVersion(item) === item.listing.latestVersion" :loading="installing === item.listing.id" @click="install(item)">
            {{ installedVersion(item) === item.listing.latestVersion ? "已安装" : installedVersion(item) ? "更新" : "安装" }}
          </el-button>
        </div>
      </article>
      <el-empty v-if="!loading && listings.length === 0" description="没有匹配的插件" />
    </div>

    <el-dialog v-model="detailVisible" :title="selected?.listing.displayName" width="720px">
      <template v-if="selected">
        <p class="detail-summary">{{ selected.listing.description || selected.listing.summary }}</p>
        <el-descriptions :column="2" border>
          <el-descriptions-item label="扩展 ID">{{ selected.listing.extensionId }}</el-descriptions-item>
          <el-descriptions-item label="发布者">{{ selected.listing.publisherId }}</el-descriptions-item>
          <el-descriptions-item label="分类">{{ selected.category?.name }}</el-descriptions-item>
          <el-descriptions-item label="信任级别">{{ trustLabel(selected.listing.trustLevel) }}</el-descriptions-item>
          <el-descriptions-item label="最新版本">{{ selected.listing.latestVersion }}</el-descriptions-item>
          <el-descriptions-item label="安全评分">{{ selected.listing.securityScore }}</el-descriptions-item>
        </el-descriptions>
        <h3>版本</h3>
        <el-table :data="selected.versions" size="small" border>
          <el-table-column prop="version" label="版本" width="130" />
          <el-table-column prop="status" label="状态" width="120" />
          <el-table-column prop="packageHash" label="SHA-256" min-width="260" />
        </el-table>
      </template>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
        <el-button v-if="selected" type="primary" :disabled="!selected.installable" @click="install(selected)">安装最新版本</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../components/ExtensionPageHeader.vue";
import { fetchExtensionCenterView } from "../api";
import {
  installMarketListing,
  listMarketCategories,
  listMarketListings,
  type MarketplaceCategory,
  type MarketplaceListingView,
} from "@/api/agent-platform";
import { originalErrorMessage } from "@/runtime/original-error";

const loading = ref(false);
const error = ref("");
const categories = ref<MarketplaceCategory[]>([]);
const listings = ref<MarketplaceListingView[]>([]);
const category = ref("");
const search = ref("");
const installing = ref("");
const detailVisible = ref(false);
const selected = ref<MarketplaceListingView | null>(null);
const installedVersions = ref<Record<string, string>>({});

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const [categoryItems, listingItems, extensions] = await Promise.all([
      listMarketCategories(),
      listMarketListings({ categoryId: category.value, q: search.value, platform: platformName() }),
      fetchExtensionCenterView(),
    ]);
    categories.value = categoryItems;
    listings.value = listingItems;
    const versions: Record<string, string> = {};
    for (const card of [
      ...extensions.installed,
      ...extensions.updates,
      ...extensions.needsAction,
      ...extensions.localImports,
      ...extensions.dev,
    ]) {
      versions[card.extensionId] = card.version;
    }
    installedVersions.value = versions;
  } catch (cause) {
    error.value = originalErrorMessage(cause);
  } finally {
    loading.value = false;
  }
}

function selectCategory(value: string) {
  category.value = value;
  load();
}

function openDetails(item: MarketplaceListingView) {
  selected.value = item;
  detailVisible.value = true;
}

async function install(item: MarketplaceListingView) {
  const previousVersion = installedVersion(item);
  installing.value = item.listing.id;
  try {
    await installMarketListing(item.listing.id, item.listing.latestVersion);
    ElMessage.success(previousVersion ? "插件更新已提交" : "插件安装已提交");
    detailVisible.value = false;
  } catch (cause) {
    ElMessage.error(originalErrorMessage(cause));
  } finally {
    installing.value = "";
  }
}

function installedVersion(item: MarketplaceListingView) {
  return installedVersions.value[item.listing.extensionId] || "";
}

function platformName() {
  const value = navigator.userAgent.toLowerCase();
  if (value.includes("android")) return "android";
  if (value.includes("iphone") || value.includes("ipad")) return "ios";
  if (value.includes("windows")) return "windows";
  if (value.includes("linux")) return "linux";
  return "macos";
}

function trustLabel(value: string) {
  return ({ platform: "平台", official: "官方", verified: "已验证", trusted: "可信", user_trusted: "用户信任", community: "社区" } as Record<string, string>)[value] || value;
}

function trustType(value: string) {
  if (value === "official" || value === "platform") return "success";
  if (value === "verified" || value === "trusted") return "primary";
  return "info";
}

onMounted(load);
</script>

<style scoped>
.marketplace-page {
  display: grid;
  gap: 18px;
}

.market-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 12px;
  border: 1px solid var(--surface-border);
  border-radius: 14px;
  background: var(--surface-bg);
}

.category-strip {
  display: flex;
  min-width: 0;
  gap: 6px;
  overflow-x: auto;
}

.category-strip button {
  flex: 0 0 auto;
  padding: 8px 13px;
  border: 0;
  border-radius: 999px;
  color: var(--text-secondary);
  background: var(--surface-soft);
  cursor: pointer;
}

.category-strip button.active {
  color: white;
  background: var(--accent);
}

.market-search {
  width: min(420px, 40vw);
}

.market-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  min-height: 220px;
}

.market-card {
  display: grid;
  align-content: start;
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--surface-border);
  border-radius: 16px;
  background: var(--surface-bg);
}

.market-card__head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.market-icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 13px;
  color: white;
  font-weight: 700;
  background: linear-gradient(145deg, var(--accent), color-mix(in srgb, var(--accent) 45%, #111));
}

.market-title {
  min-width: 0;
  flex: 1;
}

.market-title h2 {
  margin: 0;
  font-size: 16px;
}

.market-title p,
.market-summary,
.market-meta,
.detail-summary {
  color: var(--text-secondary);
}

.market-title p {
  margin: 3px 0 0;
  overflow: hidden;
  font: 11px ui-monospace, SFMono-Regular, monospace;
  text-overflow: ellipsis;
}

.market-summary {
  min-height: 44px;
  margin: 0;
  line-height: 1.55;
}

.market-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
  font-size: 11px;
}

.market-meta span {
  padding: 3px 8px;
  border-radius: 999px;
  background: var(--surface-soft);
}

.market-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

h3 {
  margin: 22px 0 10px;
}

@media (max-width: 1100px) {
  .market-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .market-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .market-search {
    width: 100%;
  }

  .market-grid {
    grid-template-columns: 1fr;
  }
}
</style>
