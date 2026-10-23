<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
const emit = defineEmits(['add_RenewData'])
const dialogFormVisible = ref(false)

// 新增验证规则
const newCategory = ref<string>('')
const subMitForm = () => {
  if(authStore.isAuthenticated && authStore.user) {
    axios.post(`/api/article/category/add?uuid=${authStore.user.uuid}`,
      {
        categoryName: newCategory.value
      },
      {
        headers: {
          'Authorization': localStorage.getItem('token') || ''
        }
      }
    )
    .then((res) => {
      if(res.data.code === 200){
        ElMessage.success('新增成功')
        dialogFormVisible.value = false
        emit('add_RenewData')
        newCategory.value = ''
      } else {
        ElMessageBox.alert(res.data.msg, '提示', {
          type: 'error',
          confirmButtonText: '确定',
        })
      }
    })
    .catch((err) => {
        ElMessageBox.alert(err.msg, '提示', {
          type: 'error',
          confirmButtonText: '确定',
        })
    })
  }
  dialogFormVisible.value = false
}
</script>
<template>
  <div id="addStudent" @click="dialogFormVisible = true">新增分类</div>
  <el-dialog v-model="dialogFormVisible" title="请填写分类信息" width="500">
    <el-input v-model="newCategory" placeholder="请输入分类名称" />
    <template #footer>
      <div class="dialog-footer">
        <el-button @click="dialogFormVisible = false">取消</el-button>
        <el-button type="primary" @click="subMitForm"> 提交 </el-button>
      </div>
    </template>
  </el-dialog>
</template>
<style scoped>
#addStudent {
  user-select: none;
  width: 80px;
  height: calc(var(--NavigationMenuHeight) - 24px);
  background-color: red;
  border-radius: var(--BorderRadius);
  background-color: rgb(97, 141, 247);
  color: white;
  /* border: 1px solid var(--BoxBorderColor); */
  box-shadow: 0px 0px 6px var(--BoxShadowColor);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: 0.3s;
  &:hover {
    background-color: rgb(59, 107, 220);
  }
}
</style>
