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
  value: string
  orderWay: string
  isDeleted: string
  startTime: string
  endTime: string
  status: string
}
const menuData = ref<MenuDataModel>({
  value: '',
  orderWay: '',
  isDeleted: '',
  startTime: '',
  endTime: '',
  status: '1',
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
    .get('/api/friendLink/list', {
      params: {
        uuid: authStore.user?.uuid,
        page: currentPage.value,
        pageSize: pageSize.value,
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
type EditFormDataModel = {
  id: number
  description: string
  name: string
  url: string
  avatar: string
  sort: number
  status: number
}
const editFormData = ref<EditFormDataModel>({
  id: 0,
  description: '',
  name: '',
  url: '',
  avatar: '',
  sort: 0,
  status: 1,
})
const handleSaveEditArticleIsloading = ref(false)
// 提交修改信息表单
const subMitForm = () => {
  handleSaveEditArticleIsloading.value = true
  axios
    .put(`/api/friendLink/update/${editFormData.value.id}`, editFormData.value, {
      headers: {
        Authorization: authStore.token || '',
      },
      params: {
        uuid: authStore.user?.uuid,
      },
    })
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
const handleFailComment = (id: number) => {
  ElMessageBox.confirm('确认删除该信息吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .delete(`/api/friendLink/fail/${id}`, {
            params: {
              uuid: authStore.user?.uuid,
            },
            headers: {
              Authorization: authStore.token,
            },
          })
          .then((res) => {
            if (res.data.code === 200) {
              ElMessage.success('删除成功')
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
const handleEdit = (data: {
  id: number
  name: string
  description: string
  url: string
  avatar: string
  sort: number
  status: number
}): void => {
  editFormData.value.id = data.id
  editFormData.value.name = data.name
  editFormData.value.description = data.description
  editFormData.value.url = data.url
  editFormData.value.avatar = data.avatar
  editFormData.value.sort = data.sort
  editFormData.value.status = data.status
  dialogFormVisible.value = true
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
      <el-table-column prop="id" label="友链ID" min-width="80" />
      <el-table-column prop="userName" label="申请用户" min-width="80" />
      <el-table-column prop="name" label="站点名称" min-width="200" />
      <el-table-column prop="description" label="站点描述" min-width="120" />
      <el-table-column prop="url" label="站点地址" min-width="180" />
      <el-table-column prop="avatar" label="站点头像" min-width="180">
        <template #default="scope">
          <div style="width: 100px; aspect-ratio: 16/9; border-radius: 8px">
            <el-image
              :src="scope.row.avatar"
              style="height: 100%"
              :preview-src-list="[scope.row.avatar]"
              :initial-index="0"
              hide-on-click-modal
              preview-teleported
              lazy
            />
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="可见状态" min-width="200">
        <template #default="scope">
          {{ scope.row.status === 1 ? '公开' : '私密' }}
        </template>
      </el-table-column>
      <el-table-column prop="sort" label="站点权重" min-width="200" />
      <el-table-column prop="createdTime" label="发布时间" min-width="200" />
      <el-table-column prop="updatedTime" label="更新时间" min-width="200" />
      <el-table-column label="操作" min-width="220px" fixed="right">
        <template #default="scope">
          <el-button
            type="default"
            size="default"
            @click="handleEdit(scope.row)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >修改</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleFailComment(scope.row.id)"
            style="margin-left: 10px"
            :title="`${scope.row.name}`"
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
    <el-dialog v-model="dialogFormVisible" title="请输入信息" width="500">
      <el-form :model="editFormData">
        <el-form-item label="* 站点名称">
          <el-input v-model="editFormData.name" autocomplete="off" />
        </el-form-item>
        <el-form-item label="* 站点描述">
          <el-input v-model="editFormData.description" autocomplete="off" />
        </el-form-item>
        <el-form-item label="* 站点地址">
          <el-input v-model="editFormData.url" autocomplete="off" />
        </el-form-item>
        <el-form-item label="* 站点头像">
          <el-input v-model="editFormData.avatar" autocomplete="off" />
        </el-form-item>
        <el-form-item label="* 站点权重">
          <el-input v-model="editFormData.sort" autocomplete="off" type="number" />
        </el-form-item>
        <el-form-item label="* 站点可见状态">
          <el-select v-model="editFormData.status" placeholder="可见状态" style="width: 100px">
            <el-option label="公开" value="1" />
            <el-option label="私密" value="0" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogFormVisible = false">取消</el-button>
          <el-button type="primary" @click="subMitForm" :loading="handleSaveEditArticleIsloading">
            提交
          </el-button>
        </div>
      </template>
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
