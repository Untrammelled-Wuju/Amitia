<script setup lang="ts">
import axios from 'axios'
import { ElTable, ElTableColumn, ElPagination, ElMessage, ElMessageBox } from 'element-plus'
import { ref, watch } from 'vue'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
const emit = defineEmits(['ChangeIndexAndSize'])
const props = defineProps({
  menuPropsData: {
    type: Object,
    default: () => {},
  },
})
// 菜单数据
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
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '2',
  isDeleted: '',
  IsComment: '',
  categoryId: '',
  tagId: '',
  startTime: '',
  endTime: '',
  title: '',
  orderWay: '',
})
// 分页数据
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0) // 总数据量
const loading = ref<boolean | true>(true) // 加载状态
// 列表数据
const infoData = ref([])
// 获取数据
const getData = () => {
  loading.value = true
  emit('ChangeIndexAndSize', currentPage.value, pageSize.value)
  axios
    .get(`/api/article/list`, {
      params: {
        uuid: JSON.parse(localStorage.getItem('user') || '{}').uuid,
        page: currentPage.value,
        pagesize: pageSize.value,
        ...menuData.value,
      },
      headers: {
        Authorization: localStorage.getItem('token'),
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        infoData.value = res.data.data.list
        total.value = res.data.data.total
      } else {
        ElMessageBox.alert(res.data.msg, `加载失败，请稍后再试`, {
          confirmButtonText: '确定',
          type: 'error',
        })
        ElMessage.error(`加载失败，请稍后再试：{${res.data.msg}}`)
      }
    })
    .catch((err) => {
      ElMessageBox.alert(err, `加载失败，请稍后再试`, {
        confirmButtonText: '确定',
        type: 'error',
      })
      ElMessage.error(`加载失败，请稍后再试：{${err.msg}}`)
    })
    .finally(() => {
      loading.value = false
    })
}
const useGetData = () => {
  setTimeout(() => {
    getData()
  }, 300)
}
useGetData()
watch(
  () => props.menuPropsData,
  (newValue) => {
    menuData.value = newValue as MenuDataModel
    loading.value = true
    useGetData()
  },
  {
    deep: true,
  },
)
// 监听分页变化
watch([currentPage, pageSize], () => {
  loading.value = true
  useGetData()
})

// 编辑
const dialogFormVisible = ref(false)
// 处理编辑按钮点击事件
const articleIndexId = ref(0)
const newArticleTitle = ref('')
const handleEditArticle = (data: { id: number; title: string }): void => {
  articleIndexId.value = data.id
  newArticleTitle.value = data.title
  dialogFormVisible.value = true
}
// 提交修改信息表单

// 发布
const handleDeleteForever = (id: number) => {
  ElMessageBox.confirm('确认删除该文章吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .delete(`/api/article/delete/${id}`, {
            params: {
              uuid: authStore.user?.uuid,
            },
            headers: {
              Authorization: authStore.token || '',
            },
          })
          .then((res) => {
            if (res.data.code === 200) {
              ElMessage.success('删除成功')
              useGetData()
            } else {
              ElMessage.error(res.data.msg)
            }
          })
          .catch((err) => {
            ElMessageBox.alert(err.message, '提示', {
              confirmButtonText: '确定',
            })
          })
      } else {
        ElMessage.info('已取消删除')
      }
    },
  })
}
</script>

<template>
  <div id="studentInfo">
    <el-table :data="infoData" border style="width: 100%" max-height="660" v-loading="loading">
      <el-table-column prop="id" label="文章ID" min-width="80" />
      <el-table-column prop="title" label="文章标题" min-width="200" />
      <el-table-column prop="description" label="文章摘要" min-width="200" />
      <el-table-column prop="cover" label="文章封面" min-width="200">
        <template #default="scope">
          <div style="width: 100px; aspect-ratio: 16/9; border-radius: 8px">
            <el-image
              :src="scope.row.cover"
              style="height: 100%"
              :preview-src-list="[scope.row.cover]"
              :initial-index="0"
              hide-on-click-modal
              preview-teleported
              lazy
            />
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="views" label="浏览量" min-width="100" />
      <el-table-column prop="category" label="文章分类" min-width="100" />
      <el-table-column prop="tags" label="文章标签" min-width="180">
        <template #default="scope">
          <el-tag type="primary" v-for="(tag, index) in scope.row.tags" :key="index">{{
            tag
          }}</el-tag>
          <el-tag type="primary" v-if="scope.row.tags === null">无</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createdTime" label="发布时间" min-width="200" />
      <el-table-column prop="updatedTime" label="更新时间" min-width="200" />
      <el-table-column label="操作" min-width="220px" fixed="right">
        <template #default="scope">
          <el-button
            type="default"
            size="default"
            @click="handleEditArticle(scope.row)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >修改</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleDeleteForever(scope.row.id)"
            style="margin-left: 10px"
            :title="`${scope.row.name} ${scope.row.id}`"
            >删除</el-button
          >
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      background
      layout="total, sizes, prev, pager, next, jumper"
      :total="total"
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      style="margin-top: 20px; justify-content: center"
      :page-sizes="[10, 20, 50, 100, 200]"
    />
  </div>
</template>
<style scoped>
#studentInfo {
  width: 100%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
}
</style>
