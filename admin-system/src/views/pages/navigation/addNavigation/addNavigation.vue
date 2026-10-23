<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import axios from 'axios'
const category_options = ref<string[]>([])
// 获取数据
const getData = () => {
  axios
    .get('/api/navigation/category', {
      params: {
        page: 1,
        pagesize: 999,
        uuid: authStore.user?.uuid,
      },
      headers: {
        Authorization: authStore.token,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        category_options.value = res.data.data
      } else {
        ElMessageBox.alert(res.data.message, `加载失败，请稍后再试`, {
          confirmButtonText: '确定',
          type: 'error',
        })
        ElMessage.error(`加载失败，请稍后再试：{${res.data.message}}`)
      }
    })
    .catch((err) => {
      ElMessageBox.alert(err, `加载失败，请稍后再试`, {
        confirmButtonText: '确定',
        type: 'error',
      })
      ElMessage.error(`加载失败，请稍后再试：{${err.message}}`)
    })
}
getData()
type Navigation = {
  name: string
  description: string
  url: string
  cover: string
  category: string
  status: number
}
const navigation = ref<Navigation>({
  name: '',
  description: '',
  url: '',
  cover: '',
  category: '',
  status: 1,
})
// 处理网站封面
const uploadCoverInput = ref<HTMLInputElement>()
const stringImg = ref<string>(navigation.value.cover)
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
// 上网站封面
const submitNavigateCover = () => {
  axios
    .post(`/api/navigation/uploadCover?uuid=${authStore.user?.uuid}`, imgFormData, {
      headers: {
        'Content-Type': 'multipart/form-data',
        Authorization: authStore.token,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('图片上传成功')
        navigation.value.cover = res.data.data
        stringImg.value = navigation.value.cover
      }
    })
    .catch((error) => {
      ElMessage.error('图片上传失败: ' + (error.response?.data?.message || error.message))
    })
}
const clearNavigation = () => {
  navigation.value = {
    name: '',
    description: '',
    url: '',
    cover: '',
    category: '',
    status: 1,
  }
  stringImg.value = ''
  uploadCoverInput.value = undefined
  imgFormData.delete('file')
}
// 新增网站
const submitNavigate = () => {
  if (!navigation.value.category) {
    ElMessage.error('请选择网站分类')
    return
  }
  if (!navigation.value.name) {
    ElMessage.error('请输入网站名称')
    return
  }
  if (!navigation.value.description) {
    ElMessage.error('请输入网站描述')
    return
  }
  if (navigation.value.url === '') {
    ElMessage.error('请输入网站URl地址')
    return
  }
  if (stringImg.value === '') {
    ElMessage.error('请上传网站封面')
    return
  }

  ElMessageBox.alert('确定新增网站吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
    .then((action) => {
      if (action === 'confirm') {
        axios
          .post(
            `/api/navigation/add?uuid=${authStore.user?.uuid}`,
            {
              ...navigation.value,
            },
            {
              headers: {
                Authorization: authStore.token,
              },
            },
          )
          .then((res) => {
            if (res.data.code === 200) {
              ElMessage.success('新增成功')
              ElMessageBox.alert('发布成功，是否清空编辑区？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'info',
              }).then((action) => {
                if (action === 'confirm') {
                  clearNavigation()
                }
              })
            } else {
              ElMessageBox.alert(res.data.msg, '发布失败', {
                confirmButtonText: '确定',
                type: 'error',
              })
            }
          })
          .catch((err) => {
            ElMessageBox.alert(err.msg, '发布失败', {
              confirmButtonText: '确定',
              type: 'error',
            })
          })
      } else {
        ElMessage.error('取消发布')
        return
      }
    })
    .catch(() => {
      ElMessage.error('取消发布')
      return
    })
}
</script>

<template>
  <div class="addNavigation">
    <div class="pageTitle">· 站点导航 - 新增站点</div>
    <div class="navigateInfo">
      <div class="navigateInfoTitle">
        <div class="navigateTitle">
          <span style="color: red">*</span>
          站点名称
          <el-input
            v-model="navigation.name"
            style="width: 320px; margin-left: 10px"
            placeholder="请输入站点名称"
            clearable
            size="default"
          />
          <el-button class="releaseBtn" type="primary" size="default" @click="submitNavigate"
            >发布</el-button
          >
          <el-button class="releaseBtn" type="danger" size="default" @click="clearNavigation"
            >清空全部</el-button
          >
        </div>
        <div class="navigateOtherInfo">
          <div class="navigateDescription">
            <span style="color: red">*</span>
            站点描述
            <el-input
              placeholder="请输入站点描述"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="navigation.description"
            />
          </div>
        </div>
        <div class="navigateCover">
          <div>
            <span><span style="color: red">*</span>站点封面：</span>
            <input
              type="file"
              accept="image/*"
              ref="uploadCoverInput"
              @change="handleCoverChange"
            />
          </div>
          <img class="navigateCoverImg" :src="stringImg" alt="" />
        </div>
        <div class="navigateUrl">
          <span style="color: red">*</span>
          站点地址
          <el-input
            placeholder="请输入站点完整URl地址"
            clearable
            size="default"
            style="width: 380px; margin-left: 10px"
            v-model="navigation.url"
          />
        </div>
        <div class="category">
          <span><span style="color: red">*</span> 网站分类：</span>
          <el-select
            v-model="navigation.category"
            placeholder="请选择站点分类"
            style="width: 160px"
          >
            <el-option label="默认分类" value="默认分类" />
            <el-option
              v-for="(item, index) in category_options"
              v-show="item !== '默认分类'"
              :key="index"
              :label="item"
              :value="item"
            />
            <template #footer>
              <el-input
                v-model="navigation.category"
                class="option-input"
                placeholder="input option name"
                size="small"
                maxLength="10"
              />
            </template>
          </el-select>
        </div>
        <div class="navigateType">
          <div class="visibleTypes">
            <span><span style="color: red">*</span>可见性：</span>
            <el-radio-group v-model="navigation.status">
              <el-radio :value="0">公开</el-radio>
              <el-radio :value="1">私密</el-radio>
            </el-radio-group>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.addNavigation {
  width: 100%;
  padding: 16px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  .pageTitle {
    font-size: 18px;
    color: var(--Main_font_SecondColor);
  }
  .navigateInfo {
    width: 100%;
    margin-top: 10px;
    .navigateInfoTitle {
      --marginTopBottom: 16px;
      width: 100%;
      .navigateTitle {
        display: flex;
        align-items: center;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .releaseBtn {
          margin-left: 10px;
        }
      }
      .navigateOtherInfo {
        display: flex;
        align-items: center;
        width: 100%;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .navigateDescription {
          display: flex;
          align-items: center;
          margin-right: 10px;
          color: var(--Main_font_SecondColor);
          &:last-child {
            margin-right: 0;
          }
        }
      }
      .navigateCover {
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        input {
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
        .navigateCoverImg {
          max-height: 80px;
          border-radius: var(--BorderRadius);
          border: 1.5px solid var(--Main_BoxBorderColor);
          margin-top: 10px;
        }
      }
      .navigateUrl{
        color: var(--Main_font_SecondColor);
      }
      .category {
        display: flex;
        align-items: center;
        margin-right: 10px;
        margin-top: 20px;
        color: var(--Main_font_SecondColor);
        &:last-child {
          margin-right: 0;
        }
      }
      .navigateType {
        margin: var(--marginTopBottom) 0;
        .visibleTypes {
          margin: 10px 0;
          display: flex;
          align-items: center;
          color: var(--Main_font_SecondColor);
        }
      }
    }
  }
}
</style>
