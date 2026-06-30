<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>用户画像（可选）</h2>
    <p class="step-desc">填写你的个人信息，让 AI 更好地了解你。</p>
    <div class="profile-list">
      <div v-for="(item, idx) in profileList" :key="idx" class="profile-row">
        <el-select v-model="item.category" style="width:120px" size="default">
          <el-option label="基本信息" value="basic" />
          <el-option label="兴趣爱好" value="hobby" />
          <el-option label="工作学习" value="work" />
          <el-option label="生活习惯" value="lifestyle" />
          <el-option label="社交关系" value="social" />
          <el-option label="其他" value="other" />
        </el-select>
        <el-input v-model="item.attributeName" placeholder="属性名" style="width:140px" />
        <el-input v-model="item.attributeValue" placeholder="属性值" style="flex:1" />
        <el-button size="small" type="danger" text @click="profileList.splice(idx, 1)">删除</el-button>
      </div>
    </div>
    <el-button size="small" @click="profileList.push({ category: 'basic', attributeName: '', attributeValue: '' })" style="margin-top:8px">
      + 添加画像
    </el-button>
  </div>
</template>
<script setup lang="ts">
defineProps<{ profileList: { category: string; attributeName: string; attributeValue: string }[] }>()
</script>
