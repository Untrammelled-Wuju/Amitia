<script setup lang="ts">
import { ref } from 'vue'

// 公告示例数据
const notices = ref([
  { title: '', content: '', time: '2025-05-20' },
  { title: '', content: '', time: '2025-05-19' },
  { title: '', content: '', time: '2025-05-18' }
])
</script>

<template>
  <div class="workbench-1-rightContainer">
    <!-- 标题区域 -->
    <div class="notice-header">
      <h3>最新公告</h3>
      <el-button type="primary" text>更多</el-button>
    </div>

    <!-- 公告列表 -->
    <div class="notice-list">
      <div v-for="(notice, index) in notices" :key="index" class="notice-item">
        <div class="notice-content">
          <h4>{{ notice.title }}</h4>
          <p>{{ notice.content }}</p>
        </div>
        <div class="notice-footer">
          <el-tag type="info" size="small">{{ notice.time }}</el-tag>
          <el-button type="primary" size="small" text>查看详情</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.workbench-1-rightContainer {
  /* 保持原有容器样式不变 */
  width: 100%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  padding: 15px; /* 新增内边距 */
}

.notice-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--Main_BoxBorderColor);
  color: var(--Main_font_SecondColor);
}

.notice-list {
  height: calc(100% - 50px); /* 留出标题区域空间 */
}

.notice-item {
  padding: 12px;
  margin-bottom: 10px;
  background: var(--Main_Bg_BoxColor_Secont);
  border-radius: 6px;
  transition: all var(--transitionTime);
}

.notice-item:hover {
  transform: translateX(5px);
  box-shadow: var(--Main_boxShadow);
}

.notice-content h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
  color: var(--Main_font_SecondColor);
}

.notice-content p {
  margin: 0;
  font-size: 13px;
  color: var(--Main_font_SecondColor);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.notice-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 10px;
  .el-tag{
    background-color: var(--Main_Bg_BoxColor);
    border: none;
  }
}
</style>
