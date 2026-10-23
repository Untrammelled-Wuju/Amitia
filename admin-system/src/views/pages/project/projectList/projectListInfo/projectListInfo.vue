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
  isTop: number
  isDeleted: string
  startTime: string
  endTime: string
  value: string
  orderWay: 'ASC' | 'DESC'
}
const menuData = ref<MenuDataModel>({
  status: '0',
  isTop: 1,
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
  category: string
  cover: string
  createdTime: string
  deletedTime: string
  description: string
  id: number
  isDeleted: number
  name: string
  type: number
  status: number
  updatedTime: string
  url: string
  isTop: number
}
const infoData = ref<InfoDataItem[]>([])
// 获取数据
const getData = () => {
  loading.value = true
  emit('ChangeIndexAndSize', currentPage.value, pageSize.value)
  axios
    .get(`/api/project/list`, {
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
type editNavigationInterface = {
  id: number
  category: string
  cover: string
  description: string
  name: string
  type: number
  status: number | null
  url: string
  isTop: number
}
const editNavigation = ref<editNavigationInterface>({
  id: -1,
  category: '',
  cover: '',
  description: '',
  name: '',
  type: 0,
  status: null,
  url: '',
  isTop: 0,
})
const subMitForm = () => {
  handleSaveEditIdeaIsloading.value = true
  axios
    .put(
      `/api/project/update/${ideaId.value}?uuid=${authStore.user?.uuid}`,
      {
        ...editNavigation.value,
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
          .delete(`/api/project/delete/${id}`, {
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
  editNavigation.value.id = id
  const index = infoData.value.findIndex((item) => item.id === id)
  editNavigation.value.category = infoData.value[index].category
  editNavigation.value.cover = infoData.value[index].cover
  editNavigation.value.description = infoData.value[index].description
  editNavigation.value.name = infoData.value[index].name
  editNavigation.value.type = infoData.value[index].type
  editNavigation.value.status = infoData.value[index].status
  editNavigation.value.isTop = infoData.value[index].isTop
  editNavigation.value.url = infoData.value[index].url
}

// 处理项目封面
const uploadCoverInput = ref<HTMLInputElement>()
const stringImg = ref<string>('')
const imgFormData = new FormData()
const handleCoverChange = () => {
  const file = uploadCoverInput.value?.files?.[0]
  if (file) {
    imgFormData.delete('file')
    imgFormData.append('file', file)
    const reader = new FileReader()
    reader.readAsDataURL(file)
    reader.onload = (e) => {
      stringImg.value = e.target?.result as string
    }
    submitNavigateCover()
  }
}
// 上项目封面
const submitNavigateCover = () => {
  axios
    .post(`/api/project/uploadCover?uuid=${authStore.user?.uuid}`, imgFormData, {
      headers: {
        'Content-Type': 'multipart/form-data',
        Authorization: authStore.token,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('图片上传成功')
        editNavigation.value.cover = res.data.data
        stringImg.value = editNavigation.value.cover
      }
    })
    .catch((error) => {
      ElMessage.error('图片上传失败: ' + (error.response?.data?.message || error.message))
    })
}
</script>

<template>
  <div id="studentInfo">
    <el-table :data="infoData" border style="width: 100%" max-height="660" v-loading="loading">
      <el-table-column prop="id" label="项目ID" min-width="80" />
      <el-table-column prop="category" label="项目分类" min-width="100" />
      <el-table-column prop="name" label="项目名称" min-width="200" />
      <el-table-column prop="description" label="项目描述" min-width="200" />
      <el-table-column prop="url" label="项目地址" min-width="200">
        <template #default="scope">
          <a
            class="link"
            :href="scope.row.url"
            target="_blank"
            >{{ scope.row.url }}</a
          >
        </template>
      </el-table-column>
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
      <el-table-column prop="type" label="项目类型" min-width="200">
        <template #default="scope">
          {{ scope.row.status === 0 ? '私有' : scope.row.status === 1 ? '开源' : '企业' }}
        </template>
      </el-table-column>
      <el-table-column prop="status" label="可见状态" min-width="200">
        <template #default="scope">
          {{ scope.row.status === 0 ? '公开' : '私密' }}
        </template>
      </el-table-column>
      <el-table-column prop="isTop" label="是否指定" min-width="200">
        <template #default="scope">
          {{ scope.row.status === 0 ? '置顶' : '不置顶' }}
        </template>
      </el-table-column>
      <el-table-column prop="createdTime" label="发布时间" min-width="200" />
      <el-table-column prop="updatedTime" label="更新时间" min-width="200" />
      <el-table-column label="操作" min-width="280px" fixed="right">
        <template #default="scope">
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
      <el-form :model="editNavigation" ref="formRef" label-width="120px">
        <el-form-item label="项目封面" prop="cover">
          <div style="width: 100px; aspect-ratio: 16/9; border-radius: 8px">
            <img
              :src="editNavigation.cover"
              style="width: 100%; height: 100%; object-fit: contain"
            />
          </div>
        </el-form-item>
        <el-form-item label="修改封面" prop="cover">
          <input
            id="upLoadButton"
            type="file"
            accept="image/*"
            ref="uploadCoverInput"
            @change="handleCoverChange"
          />
        </el-form-item>
        <el-form-item label="项目名称" prop="name">
          <el-input v-model="editNavigation.name" placeholder="请输入项目名称" />
        </el-form-item>
        <el-form-item label="项目描述" prop="description">
          <el-input
            v-model="editNavigation.description"
            placeholder="请输入项目描述"
            type="textarea"
            :rows="4"
            maxlength="100"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="项目地址" prop="url">
          <el-input v-model="editNavigation.url" placeholder="请输入项目地址" />
        </el-form-item>
        <el-form-item label="项目类型" prop="type">
          <el-radio-group v-model="editNavigation.type">
            <el-radio :value="0">私有</el-radio>
            <el-radio :value="1">开源</el-radio>
            <el-radio :value="2">企业</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="可见状态" prop="status">
          <el-radio-group v-model="editNavigation.status">
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
#upLoadButton {
  cursor: pointer;
  &::file-selector-button {
    padding: 6px 10px;
    background-color: #409eff;
    border: 1px solid #409eff;
    border-radius: 3px;
    cursor: pointer;
    color: #fff;
    font-size: 12px;
  }
}
#studentInfo {
  width: 100%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  margin-top: 12px;
  padding: 10px;
  box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.1);
  .link{
    text-decoration: underline;
    color: var(--Main_font_SecondColor);
  }
}
</style>
