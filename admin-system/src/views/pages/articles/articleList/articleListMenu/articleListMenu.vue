<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElConfigProvider, ElMessage } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'dayjs/locale/zh-cn'
import axios from 'axios'
interface Category {
  id: number
  name: string
}
const category_options = ref<Category[]>([])
axios
  .get('/api/article/category/list', {
    params: {
      page: 1,
      pagesize: 999,
    },
  })
  .then((res) => {
    if (res.data.code === 200) {
      category_options.value = res.data.data.list
    } else {
      ElMessage.error(`加载失败，请稍后再试：{${res.data.message}}`)
    }
  })
  .catch((err) => {
    ElMessage.error(`加载失败，请稍后再试：{${err.message}}`)
  })

interface Tag {
  id: number
  name: string
}
const tag_options = ref<Tag[]>([])
axios
  .get('/api/article/tag/list', {
    params: {
      page: 1,
      pagesize: 999,
    },
  })
  .then((res) => {
    if (res.data.code === 200) {
      tag_options.value = res.data.data.list
    } else {
      ElMessage.error(`加载失败，请稍后再试：{${res.data.message}}`)
    }
  })
  .catch((err) => {
    ElMessage.error(`加载失败，请稍后再试：{${err.message}}`)
  })

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
  IsComment: string
  categoryId: string
  tagId: string
  startTime: string
  endTime: string
  title: string
  orderWay: string
  isCarousel: string
  type: string
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '0',
  isDeleted: '',
  IsComment: '',
  categoryId: '',
  tagId: '',
  startTime: '',
  endTime: '',
  title: '',
  orderWay: '',
  isCarousel: '',
  type: '',
})

let timeoutId: ReturnType<typeof setTimeout>
const emit = defineEmits(['updateMenuData'])
watch(menuData.value, (newVal) => {
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
  <div class="articleListMenu">
    <div class="sorting">
      <div class="item">
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
      <div class="item">
        <span>是否轮播：</span>
        <el-select
          v-model="menuData.isCarousel"
          placeholder="是否轮播"
          style="width: 100px"
          clearable
        >
          <el-option label="是" value="1" />
          <el-option label="否" value="0" />
        </el-select>
      </div>
      <div class="item">
        <span>是否置顶：</span>
        <el-select v-model="menuData.isTop" placeholder="是否置顶" style="width: 100px" clearable>
          <el-option label="是" value="1" />
          <el-option label="否" value="0" />
        </el-select>
      </div>
      <div class="item">
        <span>可见状态：</span>
        <el-select v-model="menuData.status" placeholder="可见状态" style="width: 100px" clearable>
          <el-option label="公开" value="0" />
          <el-option label="私密" value="1" />
        </el-select>
      </div>
    </div>
    <div class="sorting">
      <div class="item">
        <span>文章类型：</span>
        <el-select v-model="menuData.type" placeholder="文章类型" style="width: 100px" clearable>
          <el-option label="普通文章" value="0" />
          <el-option label="转载文章" value="1" />
          <el-option label="关于本站" value="2" />
          <el-option label="版权协议" value="3" />
          <el-option label="隐私政策" value="4" />
        </el-select>
      </div>
      <div class="item">
        <span>文章分类：</span>
        <el-select
          v-model="menuData.categoryId"
          placeholder="请选择文章分类"
          style="width: 160px"
          clearable
        >
          <el-option
            v-for="item in category_options"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </el-select>
      </div>
      <div class="item">
        <span>文章标签：</span>
        <el-select
          v-model="menuData.tagId"
          collapse-tags
          collapse-tags-tooltip
          placeholder="请选择文章标签"
          style="width: 160px"
          clearable
        >
          <el-option
            v-for="item in tag_options"
            :key="item.id"
            :label="item.name"
            :value="item.id"
          />
        </el-select>
      </div>
    </div>
    <div class="sorting">
      <div class="item">
        <span>搜索标题：</span>
        <el-input
          v-model="menuData.title"
          style="width: 320px"
          placeholder="请输入搜索内容"
          clearable
        >
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
.articleListMenu {
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
    .item {
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
