<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import axios from 'axios'
type Project = {
  name: string
  description: string
  url: string
  cover: string
  type: number
  status: number
  isTop: number
}
const project = ref<Project>({
  name: '',
  description: '',
  url: '',
  cover: '',
  type: 0,
  status: 1,
  isTop: 1
})
// 处理项目封面
const uploadCoverInput = ref<HTMLInputElement>()
const stringImg = ref<string>(project.value.cover)
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
        project.value.cover = res.data.data
        stringImg.value = project.value.cover
      }
    })
    .catch((error) => {
      ElMessage.error('图片上传失败: ' + (error.response?.data?.message || error.message))
    })
}
const clearProject = () => {
  project.value = {
    name: '',
    description: '',
    url: '',
    cover: '',
    type: 0,
    status: 1,
    isTop: 1
  }
  stringImg.value = ''
  uploadCoverInput.value = undefined
  imgFormData.delete('file')
}
// 新增项目
const submitNavigate = () => {
  if (!project.value.name) {
    ElMessage.error('请输入项目名称')
    return
  }
  if (!project.value.description) {
    ElMessage.error('请输入项目描述')
    return
  }
  if (project.value.url === '') {
    ElMessage.error('请输入项目URl地址')
    return
  }
  if (stringImg.value === '') {
    ElMessage.error('请上传项目封面')
    return
  }

  ElMessageBox.alert('确定新增项目吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
    .then((action) => {
      if (action === 'confirm') {
        axios
          .post(
            `/api/project/add?uuid=${authStore.user?.uuid}`,
            {
              ...project.value,
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
                  clearProject()
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
  <div class="addProject">
    <div class="pageTitle">· 站点导航 - 新增站点</div>
    <div class="projectInfo">
      <div class="projectInfoTitle">
        <div class="projectTitle">
          <span style="color: red">*</span>
          站点名称
          <el-input
            v-model="project.name"
            style="width: 320px; margin-left: 10px"
            placeholder="请输入站点名称"
            clearable
            size="default"
          />
          <el-button class="releaseBtn" type="primary" size="default" @click="submitNavigate"
            >发布</el-button
          >
          <el-button class="releaseBtn" type="danger" size="default" @click="clearProject"
            >清空全部</el-button
          >
        </div>
        <div class="projectOtherInfo">
          <div class="projectDescription">
            <span style="color: red">*</span>
            站点描述
            <el-input
              placeholder="请输入站点描述"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="project.description"
              type="textarea"
              :rows="4"
              maxlength="100"
              show-word-limit
            />
          </div>
        </div>
        <div class="projectCover">
          <div>
            <span><span style="color: red">*</span>站点封面：</span>
            <input
              type="file"
              accept="image/*"
              ref="uploadCoverInput"
              @change="handleCoverChange"
            />
          </div>
          <img class="projectCoverImg" :src="stringImg" alt="" />
        </div>
        <div class="projectUrl">
          <span style="color: red">*</span>
          站点地址
          <el-input
            placeholder="请输入站点完整URl地址"
            clearable
            size="default"
            style="width: 380px; margin-left: 10px"
            v-model="project.url"
          />
        </div>
        <div class="projectType">
          <div class="visibleTypes">
            <span><span style="color: red">*</span>可见性：</span>
            <el-radio-group v-model="project.status">
              <el-radio :value="0">公开</el-radio>
              <el-radio :value="1">私密</el-radio>
            </el-radio-group>
          </div>
          <div class="useTypes">
            <span><span style="color: red">*</span>可见性：</span>
            <el-radio-group v-model="project.type">
              <el-radio :value="0">私有</el-radio>
              <el-radio :value="1">开源</el-radio>
              <el-radio :value="2">企业</el-radio>
            </el-radio-group>
          </div>
          <div class="isTop">
            <span><span style="color: red">*</span>是否置顶：</span>
            <el-radio-group v-model="project.isTop">
              <el-radio :value="0">置顶</el-radio>
              <el-radio :value="1">不置顶</el-radio>
            </el-radio-group>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.addProject {
  width: 100%;
  padding: 16px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  .pageTitle {
    font-size: 18px;
    color: var(--Main_font_SecondColor);
  }
  .projectInfo {
    width: 100%;
    margin-top: 10px;
    .projectInfoTitle {
      --marginTopBottom: 16px;
      width: 100%;
      .projectTitle {
        display: flex;
        align-items: center;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .releaseBtn {
          margin-left: 10px;
        }
      }
      .projectOtherInfo {
        display: flex;
        align-items: center;
        width: 100%;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .projectDescription {
          display: flex;
          align-items: center;
          margin-right: 10px;
          color: var(--Main_font_SecondColor);
          &:last-child {
            margin-right: 0;
          }
        }
      }
      .projectCover {
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
        .projectCoverImg {
          max-height: 80px;
          border-radius: var(--BorderRadius);
          border: 1.5px solid var(--Main_BoxBorderColor);
          margin-top: 10px;
        }
      }
      .projectUrl {
        color: var(--Main_font_SecondColor);
      }
      .projectType {
        margin: var(--marginTopBottom) 0;
        .visibleTypes,
        .useTypes,
        .isTop {
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
