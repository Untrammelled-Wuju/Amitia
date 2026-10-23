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
const handleUploadAvatar = (res: upLoadImgResDataType) => {
  if (res.code === 200) {
    const imageUrl = res.data
    webInfo.value.avatar = imageUrl
    ElMessage.success('上传成功-点击保存后才会生效哦~')
  } else {
    ElMessageBox.alert(res.msg, '上传失败', {
      confirmButtonText: '确定',
      type: 'error',
    })
  }
}
const handleUploadWechat = (res: upLoadImgResDataType) => {
  if (res.code === 200) {
    const imageUrl = res.data
    webInfo.value.Wechat = imageUrl
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
  <div id="baseWebInfo">
    <div class="webInfo">
      <h3>-- 网站信息：</h3>
      <el-form :model="webInfo" ref="formRef" label-width="100" label-position="left">
        <el-form-item label="网站版本" prop="version">
          <el-input v-model="webInfo.version" placeholder="请输入网站版本" />
        </el-form-item>
        <el-form-item label="网站名称" prop="name">
          <el-input v-model="webInfo.name" placeholder="请输入网站名称" />
        </el-form-item>
        <el-form-item label="网站描述" prop="desc">
          <el-input v-model="webInfo.desc" placeholder="请输入网站描述" />
        </el-form-item>
        <el-form-item label="网站标题" prop="title">
          <el-input v-model="webInfo.title" placeholder="请输入网站标题" />
        </el-form-item>
        <el-form-item label="网站备案号" prop="recordnumber">
          <el-input v-model="webInfo.recordnumber" placeholder="请输入网站备案号" />
        </el-form-item>
        <el-form-item label="网站创建时间" prop="createdTime">
          <el-input v-model="webInfo.createdTime" placeholder="请输入网站创建时间" />
        </el-form-item>
      </el-form>
    </div>
    <div class="authorInfo">
      <h3>-- 网站展示作者信息：</h3>
      <el-form :model="webInfo" ref="formRef" label-width="120" label-position="left">
        <el-form-item label="网站作者" prop="author">
          <el-input v-model="webInfo.author" placeholder="请输入网站作者" />
        </el-form-item>
        <el-form-item label="座右铭" prop="proverb">
          <el-input v-model="webInfo.proverb" placeholder="请输入座右铭" />
        </el-form-item>
        <el-form-item label="QQ" prop="qq">
          <el-input v-model="webInfo.qq" placeholder="请输入QQ" />
        </el-form-item>
        <el-form-item label="微信二维码" prop="Wechat">
          <el-upload
            class="avatar-uploader"
            :show-file-list="false"
            :headers="{ Authorization: authStore.token }"
            :action="`/webInfo/upLoadImg?uuid=${authStore.user?.uuid}`"
            :on-success="handleUploadWechat"
            :on-error="handleUploadWechat"
          >
            <el-button size="small" type="primary">点击上传</el-button>
          </el-upload>
        </el-form-item>
        <el-form-item label="微信名片地址" prop="wechatImg">
          <el-input v-model="webInfo.Wechat" placeholder="请上传" disabled />
        </el-form-item>
        <el-form-item label="哔哩哔哩" prop="bilibili">
          <el-input v-model="webInfo.bilibili" placeholder="请输入哔哩哔哩UUID" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="webInfo.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="码云" prop="gitee">
          <el-input v-model="webInfo.gitee" placeholder="请输入码云账号" />
        </el-form-item>
        <el-form-item label="GitHub" prop="github">
          <el-input v-model="webInfo.github" placeholder="请输入GitHub账号" />
        </el-form-item>
        <el-form-item label="Telegram" prop="telegram">
          <el-input v-model="webInfo.telegram" placeholder="请输入Telegram" />
        </el-form-item>
      </el-form>
    </div>
    <div class="authorAvatar">
      <h3 class="top">-- 网站展示作者头像 --</h3>
      <el-image
        :src="webInfo.avatar"
        style="width: 100px; height: 100px; margin-bottom: 10px"
        lazy
        hide-on-click-modal
        preview-teleported
        :preview-src-list="[webInfo.avatar]"
      />
      <el-upload
        class="avatar-uploader"
        :show-file-list="false"
        :headers="{ Authorization: authStore.token }"
        :action="`/webInfo/upLoadImg?uuid=${authStore.user?.uuid}`"
        :on-success="handleUploadAvatar"
        :on-error="handleUploadAvatar"
      >
        <el-button size="small" type="primary">点击上传</el-button>
      </el-upload>
      <h3>-- 微信二维码 --</h3>
      <el-image
        :src="webInfo.Wechat"
        style="width: 120px; height: auto; margin-bottom: 10px"
        lazy
        hide-on-click-modal
        preview-teleported
        :preview-src-list="[webInfo.Wechat]"
      />
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
#baseWebInfo {
  width: 100%;
  display: flex;
  justify-content: space-evenly;
  flex-wrap: wrap;
  padding: 5px 10px;
  /* padding-top: 5vh; */
  .authorAvatar {
    display: flex;
    flex-direction: column;
    align-items: center;
    color: var(--Main_font_SecondColor);
  }
  .webInfo {
    color: var(--Main_font_SecondColor);
  }
  .authorInfo {
    margin-left: 20px;
    color: var(--Main_font_SecondColor);
  }
  .authorAvatar,
  .webInfo,
  .authorInfo {
    h3 {
      margin: 20px 0;
      &.top {
        margin-top: 10px;
      }
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
