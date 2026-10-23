<script setup lang="ts">
import axios from 'axios'
import { onMounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/authStore'
import { ElMessage, ElMessageBox } from 'element-plus'
const authStore = useAuthStore()

const saveButtonDisabled = ref(true)
const webInfo = ref({
  version: '',
  logo: '',
  name: '',
  desc: '',
  title: '',
  lightBg: '',
  darkBg: '',
  createdTime: '',
  author: '',
  avatar: '',
  proverb: '',
  qq: '',
  Wechat: '',
  bilibili: '',
  email: '',
  gitee: '',
  github: '',
  recordnumber: '',
  telegram: '',
  wechatPay: '',
  alipay: '',
})
onMounted(() => {
  axios.get('/webInfo/info').then((res) => {
    if (res.data.code === 200) {
      const resData = res.data.data
      webInfo.value.version = resData.version
      webInfo.value.logo = resData.logo
      webInfo.value.name = resData.name
      webInfo.value.desc = resData.desc
      webInfo.value.title = resData.title
      webInfo.value.lightBg = resData.lightBg
      webInfo.value.darkBg = resData.darkBg
      webInfo.value.createdTime = resData.createdTime
      webInfo.value.author = resData.author
      webInfo.value.avatar = resData.avatar
      webInfo.value.proverb = resData.proverb
      webInfo.value.qq = resData.qq
      webInfo.value.Wechat = resData.wechat
      webInfo.value.bilibili = resData.bilibili
      webInfo.value.email = resData.email
      webInfo.value.gitee = resData.gitee
      webInfo.value.github = resData.github
      webInfo.value.recordnumber = resData.recordNumber
      webInfo.value.telegram = resData.telegram
      webInfo.value.wechatPay = resData.wechatPay
      webInfo.value.alipay = resData.alipay
    }
  })
})

type upLoadImgResDataType = {
  code: number
  data: string
  msg: string
}
const handleUploadLightBgImg = (res: upLoadImgResDataType) => {
  if (res.code === 200) {
    const imageUrl = res.data
    webInfo.value.lightBg = imageUrl
    ElMessage.success('上传成功-点击保存后才会生效哦~')
  } else {
    ElMessageBox.alert(res.msg, '上传失败', {
      confirmButtonText: '确定',
      type: 'error',
    })
  }
}
const handleUploadDarkBgImg = (res: upLoadImgResDataType) => {
  if (res.code === 200) {
    const imageUrl = res.data
    webInfo.value.darkBg = imageUrl
    ElMessage.success('上传成功-点击保存后才会生效哦~')
  } else {
    ElMessageBox.alert(res.msg, '上传失败', {
      confirmButtonText: '确定',
      type: 'error',
    })
  }
}
let jisu = 0
watch(
  () => webInfo.value,
  () => {
    if (jisu === 0) {
      jisu++
      return
    }
    saveButtonDisabled.value = false
  },
  { deep: true },
)
const saveWebInfo = () => {
  ElMessageBox.confirm('确认保存吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  }).then(() => {
    console.log(webInfo.value)
    axios
      .put(
        `/webInfo/update?uuid=${authStore.user?.uuid}`,
        {
          ...webInfo.value,
        },
        {
          headers: {
            Authorization: authStore.token,
          },
        },
      )
      .then((res) => {
        if (res.data.code === 200) {
          saveButtonDisabled.value = true
          ElMessageBox.alert('保存成功', '提示', {
            confirmButtonText: '确定',
            type: 'success',
            showClose: false,
          })
        } else {
          ElMessageBox.alert(res.data.msg, '提示', {
            confirmButtonText: '确定',
            type: 'error',
            showClose: false,
          })
        }
      })
      .catch((err) => {
        console.log(err)
        ElMessageBox.alert('保存失败', '提示', {
          confirmButtonText: '确定',
          type: 'error',
          showClose: false,
        })
      })
  })
}
</script>
<template>
  <div id="baseWebTheme">
    <div class="webBg">
      <div class="themeLight">
        <h3>-- 网站亮色主题背景 --</h3>
        <el-image
          :src="webInfo.lightBg"
          style="width: auto; height: 160px; margin-bottom: 10px; aspect-ratio: 16/9"
          lazy
          hide-on-click-modal
          preview-teleported
          :preview-src-list="[webInfo.lightBg]"
        />
        <el-upload
          class="avatar-uploader"
          :show-file-list="false"
          :headers="{ Authorization: authStore.token }"
          :action="`/webInfo/upLoadImg?uuid=${authStore.user?.uuid}`"
          :on-success="handleUploadLightBgImg"
          :on-error="handleUploadLightBgImg"
        >
          <el-button size="small" type="primary">点击上传</el-button>
        </el-upload>
      </div>
      <div class="themeDark">
        <h3>-- 网站暗黑主题背景 --</h3>
        <el-image
          :src="webInfo.darkBg"
          style="width: auto; height: 160px; margin-bottom: 10px; aspect-ratio: 16/9"
          lazy
          hide-on-click-modal
          preview-teleported
          :preview-src-list="[webInfo.darkBg]"
        />
        <el-upload
          class="avatar-uploader"
          :show-file-list="false"
          :headers="{ Authorization: authStore.token }"
          :action="`/webInfo/upLoadImg?uuid=${authStore.user?.uuid}`"
          :on-success="handleUploadDarkBgImg"
          :on-error="handleUploadDarkBgImg"
        >
          <el-button size="small" type="primary">点击上传</el-button>
        </el-upload>
      </div>
    </div>
    <div class="btns">
      <el-button :disabled="saveButtonDisabled" type="primary" @click="saveWebInfo"
        >保存信息</el-button
      >
    </div>
  </div>
</template>
<style scoped>
:deep(.el-input) {
  width: 300px;
}
#baseWebTheme {
  width: 100%;
  padding: 5px 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  .webBg {
    color: var(--Main_font_SecondColor);
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    h3 {
      margin: 20px 0;
      width: 100%;
      text-align: center;
      &.top {
        margin-top: 10px;
      }
    }
    .themeLight,
    .themeDark {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      margin: 0 10px;
    }
  }
  .btns {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 10px;
  }
}
</style>
