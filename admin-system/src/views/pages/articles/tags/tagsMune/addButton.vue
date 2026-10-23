<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
const emit = defineEmits(['add_RenewData'])
const dialogFormVisible = ref(false)

// 新增验证规则
const newTags = ref<Array<string>>([])
const subMitForm = () => {
  if(authStore.isAuthenticated && authStore.user){
    axios.post(`/api/article/tag/add?uuid=${authStore.user.uuid}`,
      {
        tags: newTags.value
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
        newTags.value = []
      } else {
        ElMessage.error(res.data.data.msg)
      }
    })
    .catch((err) => {
      ElMessage.error('新增失败',err.msg)
    })
  }
  dialogFormVisible.value = false
}
</script>
<template>
  <div id="addStudent" @click="dialogFormVisible = true">新增标签</div>
  <el-dialog v-model="dialogFormVisible" title="请填写标签信息" width="500">
    <el-input-tag v-model="newTags" trigger="Space" placeholder="空格分隔标签" />
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
