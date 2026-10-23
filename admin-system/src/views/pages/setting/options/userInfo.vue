<script setup lang="ts">
import axios, { AxiosError } from 'axios';
import { ref } from 'vue';
import { ElMessage, type FormItemRule } from 'element-plus';  // 添加FormItemRule类型导入
import { useAuthStore } from '@/stores/authStore'
const { logout } = useAuthStore()
const userInfo = ref(JSON.parse(localStorage.getItem('user') || '{}'))

// 新增密码修改相关代码
const dialogVisible = ref(false)
const form = ref({
  newPassword: '',
  confirmPassword: '' // 移除 oldPassword 字段
})
const validatePassword = (rule: FormItemRule, value: string, callback: (error?: Error) => void) => {
  if (!value) {
    callback(new Error('请再次输入新密码'))
  } else if (value !== form.value.newPassword) {
    callback(new Error('两次输入密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' },
    { pattern: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '必须包含字母和数字', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    { validator: validatePassword, trigger: 'blur' }
  ]
}

// 添加表单引用
const formRef = ref()
const confirmEditPasswd = ref(false)
const handlePasswordChange = async () => {
  try {
    // 添加表单预验证
    await formRef.value.validate()

    confirmEditPasswd.value = true
    const response = await axios.post('/api/auth/modify/pwd', {
      username: userInfo.value.username,
      password: form.value.newPassword
    }, {
      headers: {
        Authorization: localStorage.getItem('token')
      }
    })
    if (response.data.code === 200) {
      ElMessage.success('密码修改成功，请重新登录')
      dialogVisible.value = false
      form.value = {  newPassword: '', confirmPassword: '' }
      confirmEditPasswd.value = false
      logout()
      // 刷新页面
      window.location.reload()
    } else {
      ElMessage.error(response.data.message)
      confirmEditPasswd.value = false
    }
  } catch (err) {
    // 将 any 类型替换为 AxiosError 类型检查
    if (err instanceof AxiosError) {
      ElMessage.error(err.response?.data?.message || '密码修改失败')
    } else {
      ElMessage.error('请正确填写所有字段')
    }
  }
}
</script>

<template>
  <div class="userInfo">
    <div class="userAvatar">
      <img src="/img/logo.jpg" alt="">
    </div>
    <div class="userName">账户：{{ userInfo.username }}</div>
    <div class="power">所属账户：{{ userInfo.role }}</div>
    <div class="editPasswd" @click="dialogVisible = true">
      修改密码
    </div>
  </div>
  <el-dialog v-model="dialogVisible" title="修改密码" width="30%">
    <!-- 添加 ref 属性 -->
    <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
      <el-form-item label="新密码" prop="newPassword">
        <el-input v-model="form.newPassword" type="password" show-password />
      </el-form-item>
      <el-form-item label="确认密码" prop="confirmPassword">
        <el-input v-model="form.confirmPassword" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handlePasswordChange" :loading="confirmEditPasswd">确认修改</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<style scoped>
.userInfo{
  width: 100%;
  height: 60px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  display: flex;
  align-items: center;
  padding: 0 10px;
  position: relative;
  .userAvatar{
    --wh: 40px;
    width: var(--wh);
    height: var(--wh);
    img{
      width: 100%;
      height: 100%;
      border-radius: 50%;
      box-shadow: var(--Main_boxShadow);
    }
  }
  .userName{
    margin-left: 10px;
    font-size: 18px;
    color: #444;
  }
  .power{
    margin-left: 10px;
    right: 10px;
    font-size: 16px;
    color: #444;
  }
}
.editPasswd{
  height: 36px;
  width: 80px;
  background-color: rgba(97, 141, 247, 1);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  position: absolute;
  right: 16px;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: .3s ease;
  &:hover{
    background-color: rgba(59, 107, 220, 1);
  }
}
</style>
