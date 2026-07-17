<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <Teleport to="body">
    <div v-if="visible" class="search-overlay" @click.self="close">
      <div class="search-panel">
        <div class="search-input-wrap">
          <el-icon class="search-icon"><Search /></el-icon>
          <input
            ref="inputRef"
            v-model="query"
            class="search-input"
            placeholder="搜索功能、角色..."
            @keydown="onKeydown"
          />
          <kbd class="search-kbd">ESC</kbd>
        </div>
        <div v-if="results.length" class="search-results">
          <div v-for="(group, gi) in results" :key="gi">
            <div class="result-group-label">{{ group.label }}</div>
            <div
              v-for="(item, ii) in group.items"
              :key="ii"
              class="result-item"
              :class="{ 'is-active': activeGroup === gi && activeIndex === ii }"
              @click="select(group, ii)"
              @mouseenter="activeGroup = gi; activeIndex = ii"
            >
              <div class="result-icon" :class="group.iconClass">
                <el-icon><component :is="item.icon" /></el-icon>
              </div>
              <div class="result-body">
                <span class="result-label">{{ item.label }}</span>
                <span v-if="item.desc" class="result-desc">{{ item.desc }}</span>
              </div>
            </div>
          </div>
        </div>
        <div v-else-if="query" class="search-empty">未找到匹配结果</div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from "vue"
import { useRouter } from "vue-router"
import {
  Search, UserFilled, Odometer, ChatDotRound, Connection,
  ChatDotSquare, Setting, Clock, Upload, DataAnalysis, Histogram,
  Collection, List, Timer, Notebook, Files,
} from "@element-plus/icons-vue"
import { apiClient } from "../composables/useApi"

const router = useRouter()

const visible = ref(false)
const query = ref("")
const inputRef = ref<HTMLInputElement | null>(null)
const activeGroup = ref(0)
const activeIndex = ref(0)
const characters = ref<any[]>([])

const pageItems = [
  { label: "聊天", desc: "AI 对话", to: "/chat", icon: ChatDotRound },
  { label: "运行", desc: "服务运行状态", to: "/dashboard/run", icon: Odometer },
  { label: "数据", desc: "使用数据统计", to: "/dashboard/data", icon: DataAnalysis },
  { label: "微信连接", desc: "微信消息接入", to: "/wechat", icon: Connection },
  { label: "QQ 连接", desc: "QQ 消息接入", to: "/qq", icon: ChatDotSquare },
  { label: "角色管理", desc: "AI 角色编辑", to: "/character", icon: UserFilled },
  { label: "日程提醒", desc: "主动陪伴提醒", to: "/reminders", icon: Clock },
  { label: "记忆总览", desc: "记忆数据管理", to: "/memory-manager", icon: Collection },
  { label: "情景记忆", desc: "情景记录查看", to: "/episodic", icon: List },
  { label: "记忆图谱", desc: "记忆关系图", to: "/graph", icon: Histogram },
  { label: "时间线", desc: "记忆时间线", to: "/memory-timeline", icon: Timer },
  { label: "用户画像", desc: "用户特征画像", to: "/profiles", icon: Notebook },
  { label: "世界书", desc: "世界观设定", to: "/world-book", icon: Files },
  { label: "聊天记录", desc: "历史对话", to: "/logs", icon: ChatDotRound },
  { label: "导入记录", desc: "导入批次", to: "/import", icon: Upload },
  { label: "设置", desc: "系统配置", to: "/settings", icon: Setting },
]

type ResultItem = { label: string; desc?: string; to?: string; icon: any; id?: string }
type ResultGroup = { label: string; iconClass: string; items: ResultItem[] }

const results = computed<ResultGroup[]>(() => {
  const q = query.value.trim().toLowerCase()
  const groups: ResultGroup[] = []

  if (q) {
    const matchedPages = pageItems.filter(
      (p) => p.label.toLowerCase().includes(q) || (p.desc && p.desc.toLowerCase().includes(q))
    )
    if (matchedPages.length) {
      groups.push({ label: "页面", iconClass: "group-page", items: matchedPages })
    }

    const matchedChars = characters.value.filter(
      (c: any) => (c.name || "").toLowerCase().includes(q) || (c.identity || "").toLowerCase().includes(q)
    )
    if (matchedChars.length) {
      groups.push({
        label: "角色",
        iconClass: "group-char",
        items: matchedChars.map((c: any) => ({
          label: c.name,
          desc: c.identity || c.description || "",
          icon: UserFilled,
          id: c.id,
        })),
      })
    }
  }

  return groups
})

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    close()
    return
  }
  if (e.key === "ArrowDown") {
    e.preventDefault()
    moveSelection(1)
    return
  }
  if (e.key === "ArrowUp") {
    e.preventDefault()
    moveSelection(-1)
    return
  }
  if (e.key === "Enter") {
    e.preventDefault()
    const group = results.value[activeGroup.value]
    if (group) {
      select(group, activeIndex.value)
    }
  }
}

function moveSelection(delta: number) {
  const groups = results.value
  if (!groups.length) return

  const currentGroup = groups[activeGroup.value]
  if (!currentGroup) {
    activeGroup.value = 0
    activeIndex.value = 0
    return
  }

  const newIndex = activeIndex.value + delta
  if (newIndex >= 0 && newIndex < currentGroup.items.length) {
    activeIndex.value = newIndex
  } else if (newIndex < 0) {
    if (activeGroup.value > 0) {
      activeGroup.value--
      activeIndex.value = groups[activeGroup.value].items.length - 1
    }
  } else {
    if (activeGroup.value < groups.length - 1) {
      activeGroup.value++
      activeIndex.value = 0
    }
  }
}

function select(group: ResultGroup, index: number) {
  const item = group.items[index]
  if (!item) return
  if (item.to) {
    router.push(item.to)
  } else if (item.id) {
    router.push(`/character/${item.id}`)
  }
  close()
}

function open() {
  visible.value = true
  query.value = ""
  activeGroup.value = 0
  activeIndex.value = 0
  fetchCharacters()
  nextTick(() => inputRef.value?.focus())
}

function close() {
  visible.value = false
}

async function fetchCharacters() {
  try {
    const res = await apiClient.get("/api/characters")
    const chars = res.data?.data || res.data
    if (Array.isArray(chars)) {
      characters.value = chars
    }
  } catch {
  }
}

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === "k") {
    e.preventDefault()
    if (visible.value) {
      close()
    } else {
      open()
    }
  }
}

watch(visible, (v) => {
  if (!v) {
    query.value = ""
  }
})

onMounted(() => {
  window.addEventListener("keydown", onGlobalKeydown)
})

onUnmounted(() => {
  window.removeEventListener("keydown", onGlobalKeydown)
})

defineExpose({ open, close })
</script>

<style scoped>
.search-overlay {
  position: fixed;
  inset: 0;
  z-index: 900;
  background: color-mix(in srgb, var(--tp-page) 35%, transparent);
  display: flex;
  justify-content: center;
  padding-top: 15vh;
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
}

.search-panel {
  width: min(560px, 92vw);
  max-height: 60vh;
  background: var(--tp-glass-bg-strong);
  border: 1px solid var(--tp-glass-border);
  border-radius: 14px;
  box-shadow: var(--tp-shadow-float);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.search-input-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--console-border-soft);
}

.search-icon {
  font-size: 16px;
  color: var(--console-text-muted);
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  border: 0;
  outline: none;
  background: transparent;
  font-size: 15px;
  color: var(--console-text);
  min-width: 0;
}

.search-input::placeholder {
  color: var(--console-text-muted);
}

.search-kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 20px;
  padding: 0 6px;
  border-radius: 4px;
  background: var(--console-search-bg);
  color: var(--console-text-muted);
  font-size: 11px;
  font-family: inherit;
  flex-shrink: 0;
}

.search-results {
  flex: 1;
  overflow-y: auto;
  padding: 6px 8px;
}

.result-group-label {
  padding: 8px 10px 2px;
  font-size: 11px;
  font-weight: 600;
  color: var(--console-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.12s;
}

.result-item:hover,
.result-item.is-active {
  background: var(--nav-hover-bg);
}

.result-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 7px;
  flex-shrink: 0;
  font-size: 14px;
}

.group-page .result-icon {
  background: var(--console-search-bg);
  color: var(--console-text-secondary);
}

.group-char .result-icon {
  background: var(--tp-info-soft);
  color: var(--tp-info);
}

.result-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 1px;
}

.result-label {
  font-size: 13px;
  color: var(--console-text);
  font-weight: 500;
}

.result-desc {
  font-size: 11px;
  color: var(--console-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.search-empty {
  padding: 32px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--console-text-muted);
}
</style>
