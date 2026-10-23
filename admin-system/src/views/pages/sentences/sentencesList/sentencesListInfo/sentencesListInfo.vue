<script setup lang="ts">
import axios from 'axios'
import { ElPagination, ElMessage, ElMessageBox } from 'element-plus'
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
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  status: '0',
  isDeleted: '',
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
  content: string
  createdTime: string
  deletedTime: string
  id: number
  isDeleted: number
  isTop: number
  status: number
  updatedTime: string
}
const infoData = ref<InfoDataItem[]>([])
// 获取数据
const getData = () => {
  loading.value = true
  emit('ChangeIndexAndSize', currentPage.value, pageSize.value)
  axios
    .get(`/api/sentence/list`, {
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

// 编辑
const dialogFormVisible = ref(false)
// 处理编辑按钮点击事件
const sentenceId = ref<number>(0)
const handleSaveEditSentenceIsloading = ref<boolean | false>(false)
// 提交修改信息表单
type editSentenceInterface = {
  id: number
  content: string
  status: number | null
  isTop: number | null
}
const editSentence = ref<editSentenceInterface>({
  id: -1,
  content: '',
  status: null,
  isTop: null,
})
const subMitForm = () => {
  handleSaveEditSentenceIsloading.value = true
  axios
    .put(
      `/api/sentence/update/${sentenceId.value}?uuid=${authStore.user?.uuid}`,
      {
        ...editSentence.value,
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
      handleSaveEditSentenceIsloading.value = false
    })
}
// 删除
const handleDeleteSentence = (id: number) => {
  ElMessageBox.confirm('确认删除该信息吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .delete(`/api/sentence/delete/${id}`, {
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
const handleEditSentence = (id: number) => {
  sentenceId.value = id
  dialogFormVisible.value = true
  editSentence.value.id = id
  const index = infoData.value.findIndex((item) => item.id === id)
  editSentence.value.content = infoData.value[index].content
  editSentence.value.status = infoData.value[index].status
  editSentence.value.isTop = infoData.value[index].isTop
}
</script>

<template>
  <div id="studentInfo">
    <el-table :data="infoData" border style="width: 100%" max-height="660" v-loading="loading">
      <el-table-column prop="id" label="句子ID" min-width="80" />
      <el-table-column prop="content" label="句子内容" min-width="500" />
      <el-table-column prop="status" label="可见状态" min-width="90">
        <template #default="scope">
          {{ scope.row.status === 0 ? '公开' : '私密' }}
        </template>
      </el-table-column>
      <el-table-column prop="createdTime" label="发布时间" min-width="200" />
      <el-table-column prop="updatedTime" label="更新时间" min-width="200" />
      <el-table-column label="操作" min-width="280px" fixed="right">
        <template #default="scope">
          <!-- <el-button
            type="default"
            size="default"
            @click="handleComment(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >评论管理</el-button
          > -->
          <el-button
            type="default"
            size="default"
            @click="handleEditSentence(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >修改</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleDeleteSentence(scope.row.id)"
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
    <el-dialog v-model="dialogFormVisible" title="请输入信息" width="500">
      <el-form :model="editSentence" ref="formRef" label-width="120px">
        <el-form-item label="句子内容" prop="content">
          <el-input
            v-model="editSentence.content"
            placeholder="请输入句子内容"
            type="textarea"
            :rows="4"
            maxlength="100"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="是否置顶" prop="isTop">
          <el-radio-group v-model="editSentence.isTop">
            <el-radio :value="0">否</el-radio>
            <el-radio :value="1">是</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="可见状态" prop="status">
          <el-radio-group v-model="editSentence.status">
            <el-radio :value="0">公开</el-radio>
            <el-radio :value="1">私密</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogFormVisible = false">取消</el-button>
          <el-button type="primary" @click="subMitForm" :loading="handleSaveEditSentenceIsloading">
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
