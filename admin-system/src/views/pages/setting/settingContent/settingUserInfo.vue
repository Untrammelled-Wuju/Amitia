<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import { ElInput, ElButton, ElMessage } from 'element-plus'
import axios from 'axios'
interface userinfoInterface {
  avatar: string
  uuid: string
  username: string
  sign: string
  gender: number
  email: string
  created_at: string
  last_login_at: string
  role: string
  token: string
  updated_at: string
}
const userInfo = ref<userinfoInterface>({
  avatar: authStore.user?.avatar || '',
  uuid: authStore.user?.uuid || '',
  username: authStore.user?.username || '',
  sign: authStore.user?.sign || '',
  gender: authStore.user?.gender || 0,
  email: authStore.user?.email || '',
  created_at: authStore.user?.created_at || '',
  last_login_at: authStore.user?.last_login_at || '',
  role: authStore.user?.role || '',
  token: authStore.user?.token || '',
  updated_at: authStore.user?.updated_at || '',
})

const fileInputRef = ref<HTMLInputElement | null>(null)

const handleFileChange = (e: Event) => {
  const input = e.target as HTMLInputElement
  if (input.files && input.files[0]) {
    const file = input.files[0]
    // 显示图片预览
    const reader = new FileReader()
    reader.onload = (e) => {
      userInfo.value.avatar = e.target?.result as string
    }
    reader.readAsDataURL(file)
    // 上传头像到服务器
    uploadAvatar(file)
  }
}
const uploadAvatar = (file: File) => {
  fileInputRef.value?.click()
  const formData = new FormData()
  formData.append('file', file)
  axios
    .post(`/api/user/updateAvatar?uuid=${authStore.user?.uuid}`, formData, {
      headers: {
        Authorization: authStore.user?.token || '',
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('头像更新成功')
        userInfo.value.avatar = res.data.data
        authStore.updateUserInfo(authStore.token, userInfo.value)
      } else {
        ElMessage.error('头像更新失败')
      }
    })
}
const handleUpdataAvatar = () => {
  fileInputRef.value?.click()
}
const handleSave = () => {
  axios
    .put(
      `/api/user/update?uuid=${authStore.user?.uuid}`,
      {
        username: userInfo.value.username,
        sign: userInfo.value.sign,
        // gender: userInfo.value.gender,
        email: userInfo.value.email,
      },
      {
        headers: {
          Authorization: authStore.user?.token || '',
        },
      },
    )
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('信息更新成功')
        authStore.updateUserInfo(res.data.data.token, res.data.data)
      } else {
        ElMessage.error('信息更新失败')
      }
    })
}
</script>
<template>
  <div id="settingUserInfo">
    <div class="userAvatar">
      <div class="title">头像：</div>
      <div class="imgBox">
        <div class="img">
          <img :src="userInfo.avatar" alt="" />
        </div>
        <ElButton class="btn" type="primary" @click="handleUpdataAvatar">修改</ElButton>
        <input
          type="file"
          ref="fileInputRef"
          accept="image/*"
          style="display: none"
          @change="handleFileChange"
        />
      </div>
    </div>
    <div class="item">
      <div class="title ">登录账号:</div>
      <ElInput class="input" v-model="userInfo.username" placeholder="请输入用户名" />
    </div>
    <div class="item lowColor">
      <div class="title">用户ID:</div>
      <div class="input">{{ userInfo.uuid }}</div>
    </div>
    <div class="item">
      <div class="title">签名:</div>
      <ElInput class="input" v-model="userInfo.sign" placeholder="请输入签名" />
    </div>
    <div class="item">
      <div class="title">邮箱:</div>
      <ElInput class="input" v-model="userInfo.email" placeholder="请输入邮箱" />
    </div>
    <div class="item marginTop lowColor">
      <div class="title">创 建 于 :</div>
      <div class="input">{{ userInfo.created_at }}</div>
    </div>
    <div class="item lowColor">
      <div class="title">上次登录:</div>
      <div class="input">{{ userInfo.last_login_at }}</div>
    </div>
    <div class="item lowColor">
      <div class="title">角色权限:</div>
      <div class="input">{{ userInfo.role }}</div>
    </div>
    <div class="item lowColor">
      <div class="title">上次更新:</div>
      <div class="input">{{ userInfo.updated_at }}</div>
    </div>
    <div class="saveBtnBox">
      <ElButton type="primary" @click="handleSave">保存</ElButton>
    </div>
  </div>
</template>
<style scoped>
#settingUserInfo {
  width: 100%;
  padding: 16px 20px;
  .item,
  .userAvatar {
    width: 100%;
    display: flex;
    align-items: center;
    margin-top: 10px;
    color: var(--Main_font_SecondColor);
    &:first-child {
      margin-top: 0;
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
