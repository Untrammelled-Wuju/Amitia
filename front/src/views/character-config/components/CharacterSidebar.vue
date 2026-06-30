<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <aside class="char-sidebar">
    <div class="sidebar-header">
      <h3>角色列表</h3>
      <el-button :icon="Plus" size="small" type="primary" @click="emit('create')">新建</el-button>
    </div>

    <div class="templates-section">
      <el-button type="primary" :icon="Plus" size="small" @click="emit('openTemplates')" style="width:100%">
        从模板创建
      </el-button>
    </div>

    <div class="divider"></div>

    <div class="char-list">
      <div
        v-for="c in characters"
        :key="c.id"
        class="char-list-item"
        :class="{ active: selectedId === c.id, 'is-active': c.isActive }"
        @click="emit('select', c)"
      >
        <div class="cli-main">
          <el-avatar :size="28">{{ c.name?.charAt(0) }}</el-avatar>
          <span class="cli-name">{{ c.name }}</span>
          <el-tag v-if="c.isActive" type="success" size="small" effect="dark">当前</el-tag>
        </div>
        <div class="cli-actions" v-if="selectedId === c.id">
          <el-button text size="small" @click.stop="emit('copy', c)" title="复制">
            <el-icon><CopyDocument /></el-icon>
          </el-button>
          <el-button text size="small" type="danger" @click.stop="emit('delete', c)" title="删除">
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
      </div>
      <el-empty v-if="characters.length === 0" description="还没有角色" :image-size="50" />
    </div>
  </aside>
</template>

<script setup lang="ts">
import { Plus, CopyDocument, Delete } from "@element-plus/icons-vue"
defineProps<{ characters: any[]; selectedId: string }>()
const emit = defineEmits<{
  (e: "create"): void
  (e: "openTemplates"): void
  (e: "select", c: any): void
  (e: "copy", c: any): void
  (e: "delete", c: any): void
}>()
</script>
