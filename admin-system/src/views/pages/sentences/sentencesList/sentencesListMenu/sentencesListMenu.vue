<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElConfigProvider } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'dayjs/locale/zh-cn'
const shortcuts = [
  {
    text: '上周',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setDate(start.getDate() - 7)
      return [start, end]
    },
  },
  {
    text: '上月',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setMonth(start.getMonth() - 1)
      return [start, end]
    },
  },
  {
    text: '前3个月',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setMonth(start.getMonth() - 3)
      return [start, end]
    },
  },
]

const timeData = ref<Array<string>>([])
interface MenuDataModel {
  isTop: string
  status: string
  isDeleted: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '0',
  isDeleted: '',
  startTime: '',
  endTime: '',
  value: '',
  orderWay: 'DESC',
})

let timeoutId: ReturnType<typeof setTimeout>
const emit = defineEmits(['updateMenuData'])
watch(menuData.value,(newVal) =>{
  clearTimeout(timeoutId)
  timeoutId = setTimeout(() => {
    emit('updateMenuData', newVal)
  }, 300)
})

watch(timeData, (newVal) => {
  if (newVal && newVal.length >= 2) {
    menuData.value.startTime = newVal[0] || ''
    menuData.value.endTime = newVal[1] || ''
  } else {
    menuData.value.startTime = ''
    menuData.value.endTime = ''
  }
})
</script>

<template>
  <div class="ideaListMenu">
    <div class="sorting">
      <div class="publishTime">
        <span>发布时间：</span>
        <el-config-provider :locale="zhCn">
          <el-date-picker
            v-model="timeData"
            type="daterange"
            :shortcuts="shortcuts"
            range-separator="—"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            start-placeholder="起始时间"
            end-placeholder="结束时间"
            clearable
          />
        </el-config-provider>
      </div>
    </div>
    <div class="sorting">
      <div class="isTop">
        <span>是否置顶：</span>
        <el-select
          v-model="menuData.isTop"
          placeholder="是否置顶"
          style="width: 100px"
          clearable
        >
          <el-option
            label="是"
            value="1"
          />
          <el-option
            label="否"
            value="0"
          />
        </el-select>
      </div>
      <div class="search">
        <span>搜索内容：</span>
        <el-input v-model="menuData.value" style="width: 320px;" placeholder="请输入搜索内容" clearable>
          <template #prefix>
            <el-icon class="el-input__icon"><search /></el-icon>
          </template>
        </el-input>
        <el-button type="primary" style="margin-left: 10px">搜索</el-button>
        <el-button type="danger" style="margin-left: 10px">重置</el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ideaListMenu{
  width: 100%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  padding: 0 16px;
  .sorting {
    width: 100%;
    height: 48px;
    display: flex;
    align-items: center;
    .isTop,
    .category,
    .tag,
    .publishTime,
    .sortWay,
    .search {
      display: flex;
      align-items: center;
      margin-right: 20px;
      color: var(--Main_font_SecondColor);
    }
    > div {
      &:last-child {
        margin-right: 0;
      }
    }
  }
}
</style>
