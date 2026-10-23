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
    .get('/api/article/tag/list', {
      params: {
        page: currentPage.value,
        pagesize: pageSize.value,
        ...menuData.value,
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

// 编辑
const dialogFormVisible = ref(false)
// 处理编辑按钮点击事件
const tagIndexId = ref(0)
const newtagName = ref('')
const handleEdit = (data: { id: number; name: string }): void => {
  tagIndexId.value = data.id
  newtagName.value = data.name
  dialogFormVisible.value = true
}
const handleSaveEditStudentIsloading = ref(false)
// 提交修改信息表单
const subMitForm = () => {
  handleSaveEditStudentIsloading.value = true
  axios
    .put(
      `/api/article/tag/update/${tagIndexId.value}?uuid=${authStore.user?.uuid}`,
      {
        tagName: newtagName.value,
      },
      {
        headers: {
          Authorization: localStorage.getItem('token'),
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
      handleSaveEditStudentIsloading.value = false
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
          .delete(`/api/article/tag/delete/${id}?uuid=${authStore.user?.uuid}`, {
            headers: {
              Authorization: localStorage.getItem('token'),
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
    <el-table :data="infoData" border style="width: 100%" max-height="710" v-loading="loading">
      <el-table-column prop="id" label="ID" min-width="80" />
      <el-table-column prop="name" label="标签名称" min-width="200" />
      <el-table-column prop="count" label="关联数量" min-width="100" />
      <el-table-column prop="createTime" label="创建时间" min-width="180" />
      <el-table-column prop="updateTime" label="更新时间" min-width="180" />
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
          <el-button type="primary" @click="subMitForm" :loading="handleSaveEditStudentIsloading">
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
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
}
</style>
