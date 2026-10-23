<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import { ElInput, ElMessage } from 'element-plus'
import { getEncryptData } from '@/utils/jsencrypt'
import axios from 'axios'
const password = ref('')
const password2 = ref('')
const loadding = ref(false)
const saveClick = async () => {
  if (
    password.value.length < 6 ||
    password.value.length > 20 ||
    password2.value.length < 6 ||
    password2.value.length > 20
  ) {
    ElMessage.error('密码长度不能小于6位')
    return
  }
  if (password.value === '') {
    ElMessage.error('密码不能为空')
    return
  }
  if (password.value !== password2.value) {
    ElMessage.error('两次输入的密码不一致')
    return
  }
  loadding.value = true
  try {
    const encryptedData: string = (await getEncryptData(password.value)) as string
    editPassword(encryptedData)
  } catch (err) {
    console.log(err)
    ElMessage.error('加密数据失败')
    return
  }
}
const editPassword = (pwd: string) => {
  axios
    .put(
      `/api/user/editPassword?uuid=${authStore.user?.uuid}`,
      {
        password: pwd,
      },
      {
        headers: {
          Authorization: authStore.token,
        },
      },
    )
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('密码修改成功, 请重新登录')
        setTimeout(() => {
          authStore.logout()
        }, 3000)
      } else {
        console.log(res)
        ElMessage.error('密码修改失败')
      }
    })
    .catch((err) => {
      console.log(err)
      ElMessage.error('密码修改失败')
    })
    .finally(() => {
      loadding.value = false
      password.value = ''
      password2.value = ''
    })
}
</script>
<template>
  <div id="settingEditPassword">
    <div class="item">
      <div class="title">登录账号:</div>
      <ElInput class="input" v-model="password" placeholder="请输入新密码" />
    </div>
    <div class="item">
      <div class="title">确认密码:</div>
      <ElInput class="input" v-model="password2" placeholder="请确认新密码" />
    </div>
    <div class="saveBtnBox">
      <ElButton type="primary" @click="saveClick" :loading="loadding">保存</ElButton>
    </div>
  </div>
</template>
<style scoped>
#settingEditPassword {
  width: 100%;
  padding: 16px 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  .item {
    display: flex;
    align-items: center;
    margin-top: 10px;
    color: var(--Main_font_SecondColor);
    &:first-child {
      margin-top: 20px;
    }
    .title {
      font-size: 16px;
      width: 80px;
    }
    .input {
      width: 300px;
      margin-left: 10px;
    }
    &.marginTop {
      margin-top: 38px;
    }
    &.lowColor {
      .title {
        font-size: 14px !important;
      }
      font-size: 14px !important;
    }
  }
  .userAvatar {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    .imgBox {
      display: flex;
      align-items: flex-end;
      margin: 10px 0;
      .img {
        height: 120px;
        width: auto;
        img {
          height: 100%;
          width: auto;
          border-radius: var(--BorderRadius);
        }
      }
      .btn {
        margin-left: 10px;
      }
    }
  }
  .saveBtnBox {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 58px;
  }
}
</style>
