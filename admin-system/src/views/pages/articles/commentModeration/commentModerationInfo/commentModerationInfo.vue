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
  name: string
  orderWay: string
}
const menuData = ref<MenuDataModel>({
  name: '',
  orderWay: '1',
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
    .get('/api/article/comment/moderation/list', {
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
// 导出单独刷新功能
defineExpose({
  useGetData,
})
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

const handlePassComment = (id: number) => {
  axios
    .put(
      `/api/article/comment/moderation/pass/${id}?uuid=${authStore.user?.uuid}`,
      {},
      {
        headers: {
          Authorization: authStore.token,
        },
      },
    )
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('通过成功')
        useGetData()
      } else {
        ElMessageBox.alert(res.data.msg, '提示', {
          type: 'error',
          confirmButtonText: '确定',
        })
      }
    })
    .catch((err) => {
      ElMessageBox.alert(err.message, '提示', {
        confirmButtonText: '确定',
      })
    })
}
// 删除
const handleFailComment = (id: number) => {
  ElMessageBox.confirm('确认删除该信息吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .delete(
            `/api/article/comment/moderation/fail/${id}`,{
              params: {
                uuid: authStore.user?.uuid,
              },
              headers: {
                Authorization: authStore.token,
              }
            })
          .then((res) => {
            if (res.data.code === 200) {
              ElMessage.success('不通过（删除）成功')
              useGetData()
            } else {
              ElMessageBox.alert(res.data.msg, '提示', {
                type: 'error',
                confirmButtonText: '确定',
              })
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
// 打回
const handleBackComment = (id: number) => {
  ElMessageBox.confirm('确认将该信息打回到待审核列表吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .put(
            `/api/article/comment/moderation/back/${id}?uuid=${authStore.user?.uuid}`,
            {},
            {
              headers: {
                Authorization: authStore.token,
              },
            },
            )
          .then((res) => {
            if (res.data.code === 200) {
              ElMessage.success('已打回到待审核列表')
              useGetData()
            } else {
              ElMessageBox.alert(res.data.msg, '提示', {
                type: 'error',
                confirmButtonText: '确定',
              })
            }
            })
          .catch((err) => {
            ElMessageBox.alert(err.message, '提示', {
              confirmButtonText: '确定',
            })
          })
      } else {
        ElMessage.info('已取消打回')
      }
    },
  })
}
</script>

<template>
  <div id="studentInfo">
    <el-table :data="infoData" border style="width: 100%" max-height="710" v-loading="loading">
      <el-table-column prop="id" label="ID" min-width="80" />
      <el-table-column prop="articleId" label="文章ID" min-width="80" />
      <el-table-column prop="content" label="内容" min-width="200" />
      <el-table-column prop="qq" label="QQ" min-width="120" />
      <el-table-column prop="email" label="邮箱" min-width="180" />
      <el-table-column prop="createdTime" label="发送时间" min-width="180" />
      <el-table-column label="操作" min-width="220px" fixed="right">
        <template #default="scope">
          <el-button
            type="primary"
            size="default"
            @click="handlePassComment(scope.row.id)"
            :title="`${scope.row.name}`"
            >通过</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleFailComment(scope.row.id)"
            style="margin-left: 10px"
            :title="`${scope.row.name}`"
            v-show="scope.row.status === 1"
            >不通过(删除)</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleBackComment(scope.row.id)"
            style="margin-left: 10px"
            :title="`${scope.row.name}`"
            v-show="scope.row.status === 0"
            >打回</el-button
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
