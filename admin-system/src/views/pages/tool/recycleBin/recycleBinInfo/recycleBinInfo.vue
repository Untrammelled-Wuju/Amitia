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
  status: string
  isDeleted: string
  category: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  status: '0',
  isDeleted: '1',
  category: '',
  startTime: '',
  endTime: '',
  value: '',
  orderWay: 'DESC',
})
// 分页数据
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0) // 总数据量
const loading = ref<boolean | true>(true) // 加载状态
// 列表数据
interface InfoDataItem {
  category: string
  cover: string
  createdTime: string
  deletedTime: string
  description: string
  id: number
  isDeleted: number
  name: string
  status: number
  updatedTime: string
  url: string
}
const infoData = ref<InfoDataItem[]>([])
// 获取数据
const getData = () => {
  loading.value = true
  emit('ChangeIndexAndSize', currentPage.value, pageSize.value)
  axios
    .get(`/api/tool/list`, {
      params: {
        uuid: authStore.user?.uuid,
        page: currentPage.value,
        pageSize: pageSize.value,
        ...menuData.value,
      },
      headers: {
        Authorization: authStore.token || '',
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

const handleRecovery = (id: number) => {
  ElMessageBox.confirm('确认恢复该信息吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .put(
            `/api/tool/recoveryDeleted/${id}?uuid=${authStore.user?.uuid}`,
            {},
            {
              headers: {
                Authorization: authStore.token || '',
              },
            },
          )
          .then((res) => {
            if (res.data.code === 200) {
              ElMessage.success('恢复成功')
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
        ElMessage.info('已取消恢复')
      }
    },
  })
}

// 永久删除
const handleDeleteForever = (id: number) => {
  ElMessageBox.confirm('确认永久删除该信息吗？ 该操作无法恢复！', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .delete(`/api/tool/deleteForever/${id}?uuid=${authStore.user?.uuid}`, {
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
  <div id="recycleBinInfo">
    <el-table :data="infoData" border :style="{width: `100%`}" max-height="660" v-loading="loading">
      <el-table-column prop="id" label="网站ID" min-width="80" />
      <el-table-column prop="category" label="网站分类" min-width="100" />
      <el-table-column prop="name" label="网站名称" min-width="200" />
      <el-table-column prop="description" label="网站描述" min-width="200" />
      <el-table-column prop="url" label="网站地址" min-width="200" />
      <el-table-column prop="cover" label="文章封面" min-width="200">
        <template #default="scope">
          <div style="width: 100px; aspect-ratio: 16/9; border-radius: 8px">
            <img :src="scope.row.cover" style="width: 100%; height: 100%; object-fit: contain" />
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="createdTime" label="发布时间" min-width="200" />
      <el-table-column prop="updatedTime" label="更新时间" min-width="200" />
      <el-table-column label="操作" min-width="220px" fixed="right">
        <template #default="scope">
          <el-button
            type="default"
            size="default"
            @click="handleRecovery(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >恢复</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleDeleteForever(scope.row.id)"
            style="margin-left: 10px"
            :title="`${scope.row.name} ${scope.row.id}`"
            >永久删除</el-button
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
#recycleBinInfo {
  width: 100%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
}
</style>
