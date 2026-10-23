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
  isTop: string
  status: string
  isDeleted: string
  category: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  isTop: '',
  status: '',
  isDeleted: '',
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
  content: string
  createdTime: string
  deletedTime: string
  id: number
  isDeleted: number
  isTop: number
  status: number
  tags: string
  updatedTime: string
}
const infoData = ref<InfoDataItem[]>([])
// 获取数据
const getData = () => {
  loading.value = true
  emit('ChangeIndexAndSize', currentPage.value, pageSize.value)
  axios
    .get(`/api/idea/list`, {
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
const ideaId = ref<number>(0)
const handleSaveEditIdeaIsloading = ref<boolean | false>(false)
// 提交修改信息表单
type editIdeaInterface = {
  id: number
  category: string
  tags: string
  content: string
  status: number | null
  isTop: number | null
}
const editIdea = ref<editIdeaInterface>({
  id: -1,
  category: '',
  tags: '',
  content: '',
  status: null,
  isTop: null,
})
const subMitForm = () => {
  handleSaveEditIdeaIsloading.value = true
  axios
    .put(
      `/api/idea/update/${ideaId.value}?uuid=${authStore.user?.uuid}`,
      {
        ...editIdea.value,
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
      handleSaveEditIdeaIsloading.value = false
    })
}
// 删除
const handleDeleteIdea = (id: number) => {
  ElMessageBox.confirm('确认删除该信息吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
    callback: (action: string) => {
      if (action === 'confirm') {
        axios
          .delete(`/api/idea/delete/${id}`, {
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
const handleEditIdea = (id: number) => {
  ideaId.value = id
  dialogFormVisible.value = true
  editIdea.value.id = id
  const index = infoData.value.findIndex((item) => item.id === id)
  editIdea.value.category = infoData.value[index].category
  editIdea.value.tags = infoData.value[index].tags
  editIdea.value.content = infoData.value[index].content
  editIdea.value.status = infoData.value[index].status
  editIdea.value.isTop = infoData.value[index].isTop
}
</script>

<template>
  <div id="studentInfo">
    <el-table :data="infoData" border style="width: 100%" max-height="660" v-loading="loading">
      <el-table-column prop="id" label="闪念ID" min-width="80" />
      <el-table-column prop="category" label="闪念分类" min-width="200" />
      <el-table-column prop="content" label="闪念内容" min-width="500" />
      <el-table-column prop="tags" label="闪念标签" min-width="180">
        <template #default="scope">
          <el-tag type="primary" v-for="(tag, index) in scope.row.tags.split(';')" :key="index">{{
            tag
          }}</el-tag>
          <el-tag type="primary" v-if="scope.row.tags === null">无</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="tags" label="可见状态" min-width="90">
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
            @click="handleEditIdea(scope.row.id)"
            :title="`${scope.row.name} ${scope.row.id}`"
            >修改</el-button
          >
          <el-button
            type="danger"
            size="default"
            @click="handleDeleteIdea(scope.row.id)"
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
      <el-form :model="editIdea" ref="formRef" label-width="120px">
        <el-form-item label="闪念分类" prop="category">
          <el-input v-model="editIdea.category" placeholder="请输入闪念分类" />
        </el-form-item>
        <el-form-item label="闪念标签" prop="tags">
          <el-input v-model="editIdea.tags" placeholder="请输入闪念标签，多个标签用分号隔开" />
        </el-form-item>
        <el-form-item label="闪念内容" prop="content">
          <el-input
            v-model="editIdea.content"
            placeholder="请输入闪念内容"
            type="textarea"
            :rows="4"
            maxlength="100"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="是否置顶" prop="isTop">
          <el-radio-group v-model="editIdea.isTop">
            <el-radio :value="0">否</el-radio>
            <el-radio :value="1">是</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="可见状态" prop="status">
          <el-radio-group v-model="editIdea.status">
            <el-radio :value="0">公开</el-radio>
            <el-radio :value="1">私密</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogFormVisible = false">取消</el-button>
          <el-button type="primary" @click="subMitForm" :loading="handleSaveEditIdeaIsloading">
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
