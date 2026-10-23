<script setup lang="ts">
import axios from 'axios'
import { ElTable, ElTableColumn, ElPagination, ElMessage, ElMessageBox, ElNotification, ElDialog } from 'element-plus'
import { ref, watch } from 'vue'
import { useAuthStore } from '@/stores/authStore'
import router from '@/router'
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
        uuid: authStore.user?.uuid,
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
const tagIndexId = ref(0)
const newtagName = ref('')
const handleEditArticle = (id: number): void => {
  router.push(`/articles/editArticle/${id}`)
}
const handleSaveEditArticleIsloading = ref(false)
// 提交修改信息表单
const subMitForm = () => {
  handleSaveEditArticleIsloading.value = true
  axios
    .put(
      `/api/article/tag/update/${tagIndexId.value}?uuid=${authStore.user?.uuid}`,
      {
        tagName: newtagName.value,
      },
      {
        headers: {
          Authorization: authStore.token || '',
        },
      },
    )
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('修改成功')
        dialogFormVisible.value = false
        loading.value = true
        useGetData()
      } else {
        ElMessageBox.alert(res.data.message, '提示', {
          confirmButtonText: '确定',
        })
      }
    })
    .catch((err) => {
      ElMessageBox.alert(err.message, '提示', {
        confirmButtonText: '确定',
      })
    })
    .finally(() => {
      loading.value = false
      handleSaveEditArticleIsloading.value = false
    })
}

// 删除
const handleDeleteStudent = (id: number) => {
  ElMessageBox.confirm('确认删除该信息吗？', '提示', {
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

const handleComment = (id: number) => {
  console.log(id)
}

const statusMap: { [key: number]: string } = {
  0: '公开',
  1: '私密',
  2: '草稿箱'
}

// 新增文章类型映射表
const typeMap: { [key: number]: string } = {
  0: '普通文章',
  1: '转载文章',
  2: '关于本站',
  3: '版权协议',
  4: '隐私政策',
}

function sanitizeFilename(str: string): string {
  return str
    .replace(/[\\/:*?"<>|\s]/g, '_')
    .replace(/_{2,}/g, '_')
    .substring(0, 50)
}

function downloadFile(data: string, filename: string, type = 'text/markdown') {
  const blob = new Blob([data], { type })
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  window.URL.revokeObjectURL(url)
  document.body.removeChild(a)
}

const exportDialogVisible = ref(false)
const exportLoading = ref(false)

const exportArticle = (id: number) => {
  exportDialogVisible.value = true
  exportLoading.value = true

  axios
    .get(`/api/article/${id}`, {
      params: {
        uuid: authStore.user?.uuid,
        id: id,
      },
      headers: {
        Authorization: authStore.token
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        const article = res.data.data
        article.tags = res.data.data.tagsId

        const exportData = {
          title: article.title,
          description: article.description,
          content: article.content,
          status: statusMap[article.status] || '未知状态',
          isComment: article.isComment === 1 ? '是' : '否',
          isTop: article.isTop === 1 ? '是' : '否',
          isCarousel: article.isCarousel === 1 ? '是' : '否',
          author: article.author,
          type: typeMap[article.type] || '未知类型',
          url: article.url || '无'
        }

        // 优化文件名生成：使用文章标题和描述
        const safeTitle = sanitizeFilename(exportData.title)
        const safeDescription = sanitizeFilename(exportData.description)
        const filename = `article_${safeTitle}${safeDescription ? '-' + safeDescription : ''}.md`

        // 转换为Markdown格式
        const markdown = `# ${exportData.title}\n\n**摘要**: ${exportData.description}\n**状态**: ${exportData.status}\n**是否评论**: ${exportData.isComment}\n**是否置顶**: ${exportData.isTop}\n**是否轮播**: ${exportData.isCarousel}\n**作者**: ${exportData.author}\n**类型**: ${exportData.type}\n**链接**: ${exportData.url}\n\n---\n\n${exportData.content}`

        downloadFile(markdown, filename)
      } else {
        ElNotification.error({
          title: '获取文章信息失败',
          type: 'error',
          duration: 3000,
          message: res.data.msg,
        })
      }
    })
    .catch((err) => {
      console.log(err)
      ElNotification.error({
        title: '错误',
        type: 'error',
        duration: 3000,
        message: '获取文章信息失败',
      })
    })
    .finally(() => {
      exportLoading.value = false
      exportDialogVisible.value = false
    })
}
</script>

<template>
  <div id="studentInfo">
    <el-table
      :data="infoData"
      border
      :style="{ width: `100%` }"
      max-height="100%"
      v-loading="loading"
    >
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
      <el-table-column prop="status" label="可见状态" min-width="200">
        <template #default="scope">
          {{ scope.row.status === 0 ? '公开' : '私密' }}
        </template>
      </el-table-column>
      <el-table-column prop="createdTime" label="发布时间" min-width="200" />
      <el-table-column prop="updatedTime" label="更新时间" min-width="200" />
      <el-table-column label="操作" min-width="300px" fixed="right">
        <template #default="scope">
          <el-button
            type="default"
            size="default"
            @click="handleComment(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >评论</el-button
          >
          <el-button
            type="default"
            size="default"
            @click="handleEditArticle(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >修改</el-button
          >
          <el-button
            type="default"
            size="default"
            @click="exportArticle(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >导出</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleDeleteStudent(scope.row.id)"
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
    <!-- 修改信息弹窗 -->
    <el-dialog v-model="dialogFormVisible" title="请输入信息" width="500">
      <el-input v-model="newtagName" placeholder="请输入标签名" />
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogFormVisible = false">取消</el-button>
          <el-button type="primary" @click="subMitForm" :loading="handleSaveEditArticleIsloading">
            提交
          </el-button>
        </div>
      </template>
    </el-dialog>
    <!-- 导出文章弹窗 -->
    <el-dialog v-model="exportDialogVisible" title="导出文章" :close-on-click-modal="false" :show-close="false">
      <div style="text-align: center; padding: 20px;">
        <p style="color: var(--Main_font_SecondColor);">正在导出文章，请稍候...</p>
      </div>
    </el-dialog>
  </div>
</template>
<style scoped>
#studentInfo {
  width: 100%;
  background-color: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
}
</style>








