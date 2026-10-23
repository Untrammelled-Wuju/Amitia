<script setup lang="ts">
type Emits = {
  (e: 'fullScreen', value: boolean): void
  (e: 'downloadArticle'): void  // 新增下载事件类型
  (e: 'save'): void  // 新增保存事件类型
  (e: 'prevStep'): void  // 新增上一步事件类型
  (e: 'nextStep'): void  // 新增下一步事件类型
  (e: 'clearInput'): void  // 新增清空输入框事件类型
  (e: 'bold'): void  // 新增加粗事件类型
  (e: 'incline'): void  // 新增斜体事件类型
  (e: 'crossline'): void  // 新增删除线事件类型
  (e: 'underline'): void  // 新增下划线事件类型
  (e: 'left'): void  // 新增左对齐事件类型
  (e: 'title', level: number): void  // 新增标题事件类型
  (e: 'center'): void  // 新增居中对齐事件类型
  (e: 'right'): void  // 新增右对齐事件类型
  (e: 'orderedList'): void  // 新增有序列表事件类型
  (e: 'unorderedList'): void  // 新增无序列表事件类型
  (e: 'link'): void  // 新增链接事件类型
  (e: 'img',desc:string, url:string): void  // 新增图片事件类型
  (e: 'code'): void  // 新增代码块事件类型
  (e: 'table'): void  // 新增表格事件类型\
}
const emit = defineEmits<Emits>()
import fullScreenIcon from './textEditorHeaderIcons/fullScreenIcon.vue'
import exitFullScreenIcon from './textEditorHeaderIcons/exitFullScreenIcon.vue'
import boldIcon from './textEditorHeaderIcons/boldIcon.vue'
import inclineIcon from './textEditorHeaderIcons/inclineIcon.vue'
import titleIcon from './textEditorHeaderIcons/titleIcon.vue'
import underlineIcon from './textEditorHeaderIcons/underlineIcon.vue'
import crosslineIcon from './textEditorHeaderIcons/crosslineIcon.vue'
import leftIcon from './textEditorHeaderIcons/leftIcon.vue'
import centerIcon from './textEditorHeaderIcons/centerIcon.vue'
import rightIcon from './textEditorHeaderIcons/rightIcon.vue'
import orderedListIcon from './textEditorHeaderIcons/orderedListIcon.vue'
import unorderedListIcon from './textEditorHeaderIcons/unorderedListIcon.vue'
import linkIcon from './textEditorHeaderIcons/linkIcon.vue'
import imgIcon from './textEditorHeaderIcons/imgIcon.vue'
import codeIcon from './textEditorHeaderIcons/codeIcon.vue'
import tableIcon from './textEditorHeaderIcons/tableIcon.vue'
import previousStepIcon from './textEditorHeaderIcons/previousStepIcon.vue'
import nextStepIcon from './textEditorHeaderIcons/nextStepIcon.vue'
import delIcon from './textEditorHeaderIcons/delIcon.vue'
import saveIcon from './textEditorHeaderIcons/saveIcon.vue'
import downloadIcon from './textEditorHeaderIcons/downloadIcon.vue'
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import axios from 'axios'

// 全屏编辑
const isFullScreen = ref(false)
const fullScreen = () => {
  isFullScreen.value = !isFullScreen.value
  emit('fullScreen', isFullScreen.value)
}
// 下载文章
const downloadArticle = (value: string) => {
  const link = document.createElement('a')
  const nowDate =  new Date()
  const dateString = `${nowDate.getFullYear()}-${nowDate.getMonth() + 1}-${nowDate.getDate()} ${nowDate.getHours()}点${nowDate.getMinutes().toString().padStart(2, '0')}分`
  link.download = `Article_${dateString}.md`
  link.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(value)
  link.click()
}
// 清空输入框内容
const clearInput = () => {
  ElMessageBox.confirm('确定清空输入框内容吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(() => {
    emit('clearInput')
  }).catch(() => {
    return
  })
}
defineExpose({
  downloadArticle
})


const showTitleOptions = ref(false)
const showImgOptions = ref(false)
const imgDescription = ref('')
const internalLink = ref('')
const showAddInternalLink = ref(false)
const showUploadImg = ref(false)
const handleImg = (index: number) => {
  if(index === 1){
    showAddInternalLink.value = true
  }else if(index === 2){
    showUploadImg.value = true
  }
}
const handleAddInternalLink = () => {
  emit('img',imgDescription.value,internalLink.value)
  showAddInternalLink.value = false
  internalLink.value = ''
  imgDescription.value = ''
}
const uploadImgInput = ref<HTMLInputElement>()
const handleUploadImg = () => {
  const originalFile = uploadImgInput.value?.files?.[0];
  if (!originalFile) {
    ElMessage.error('请选择要上传的图片');
    return;
  }
  if (imgDescription.value.trim() === '') {
    ElMessage.error('请输入图片描述');
    return;
  }

  // 获取文件扩展名
  const fileExtension = originalFile.name.split('.').pop();
  // 创建新的File对象，使用描述作为文件名
  const file = new File(
    [originalFile],
    `${imgDescription.value}.${fileExtension}`,
    { type: originalFile.type }
  );

  const formData = new FormData();
  formData.append('file', file);
  axios.post(`/api/article/uploadImg?uuid=${JSON.parse(localStorage.getItem('user') || '{}').uuid}`, formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
      'Authorization': localStorage.getItem('token') || ''
    }
  })
  .then(res => {
    if(res.data.code === 200){
      ElMessage.success('图片上传成功');
      emit('img',imgDescription.value,res.data.data)
      setTimeout(() => {
        showUploadImg.value = false;
        imgDescription.value = '';
        if (uploadImgInput.value) {
          uploadImgInput.value.value = '';
        }
      }, 500);
    }
  })
  .catch(error => {
    ElMessage.error('图片上传失败: ' + (error.response?.data?.message || error.message));
  });
}
</script>


<template>
  <div class="writeArticle__header">
    <div class="leftIcons">
      <div class="item"><saveIcon @click="$emit('save')"/></div>
      <div class="item"><previousStepIcon @click="$emit('prevStep')"/></div>
      <div class="item"><nextStepIcon @click="$emit('nextStep')"/></div>
      <div class="item"><delIcon @click="clearInput"/></div>
      <hr class="item-hr">
      <div class="item"><boldIcon @click="$emit('bold')"/></div>
      <div class="item"><inclineIcon @click="$emit('incline')"/></div>
      <div class="item" @mouseenter="showTitleOptions = true" @mouseleave="showTitleOptions = false">
        <titleIcon />
        <div class="custom-select">
          <ul class="options" v-show="showTitleOptions">
            <li
              v-for="level in 6"
              :key="level"
              @click="$emit('title', level)"
              @mousedown.prevent
            >
              {{ level }} 级标题
            </li>
          </ul>
        </div>
      </div>
      <div class="item"><underlineIcon @click="$emit('underline')"/></div>
      <div class="item"><crosslineIcon @click="$emit('crossline')"/></div>
      <hr class="item-hr">
      <div class="item"><leftIcon @click="$emit('left')"/></div>
      <div class="item"><centerIcon @click="$emit('center')"/></div>
      <div class="item"><rightIcon @click="$emit('right')"/></div>
      <hr class="item-hr">
      <div class="item"><orderedListIcon @click="$emit('orderedList')"/></div>
      <div class="item"><unorderedListIcon @click="$emit('unorderedList')"/></div>
      <hr class="item-hr">
      <div class="item"><linkIcon @click="$emit('link')"/></div>
      <div class="item" @mouseenter="showImgOptions = true" @mouseleave="showImgOptions = false">
        <imgIcon/>
        <div class="custom-select">
          <ul class="options" v-show="showImgOptions">
            <li @click="handleImg(1)">添加图片链接</li>
            <li @click="handleImg(2)">上传本地图片</li>
          </ul>
        </div>
      </div>
      <div class="item"><codeIcon @click="$emit('code')"/></div>
      <div class="item"><tableIcon @click="$emit('table')"/></div>
    </div>
    <div class="rightIcons">
      <div class="item"><downloadIcon @click="$emit('downloadArticle')"/></div>
      <div class="item">
        <exitFullScreenIcon v-if="isFullScreen" @click="fullScreen" />
        <fullScreenIcon v-else @click="fullScreen" />
      </div>
    </div>
  </div>
  <div class="addImgContainer" v-if="showAddInternalLink || showUploadImg">
    <div class="addImgContainer__warp" @click="showAddInternalLink = false;showUploadImg = false"></div>
    <div class="addImgContainer__internalLink" v-if="showAddInternalLink">
      <div class="addImgContainer__content">
        <div class="addImgContainer__content__title">添加图片链接</div>
        <div class="addImgContainer__content__input">
          <input type="text" placeholder="请输入图片表述" v-model="imgDescription"/>
          <input type="text" placeholder="请输入图片链接" v-model="internalLink"/>
        </div>
        <div class="buttonBox">
          <div class="buttonBox__cancel" @click="showAddInternalLink = false">取消</div>
          <div class="buttonBox__add" @click="handleAddInternalLink">添加</div>
        </div>
      </div>
    </div>
    <div class="addImgContainer__uploadImg" v-if="showUploadImg">
      <div class="addImgContainer__content">
        <div class="addImgContainer__content__title">添加图片链接</div>
        <div class="addImgContainer__content__input">
          <input type="text" placeholder="请输入图片表述" v-model="imgDescription" />
          <input ref="uploadImgInput" type="file" accept="image/*" class="uploadImgInput" />
        </div>
        <div class="buttonBox">
          <div class="buttonBox__cancel" @click="showUploadImg = false">取消</div>
          <div class="buttonBox__add" @click="handleUploadImg">添加</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.writeArticle__header {
  display: flex;
  height: 42px;
  border-bottom: 1px solid #ccc;
  padding: 10px;
  position: relative;
  align-items: center;
  justify-content: space-between;
  .leftIcons,
  .rightIcons {
    display: flex;
    align-items: center;
    height: 100%;
    .item{
      --wh: 30px;
      padding: 5px;
      height: var(--wh);
      width: var(--wh);
      cursor: pointer;
      border-radius: 3px;
      transition: .24s;
      &:hover{
        background-color: #e0e0e0;
      }
    }
    .item {
      position: relative;
      .custom-select{
        position: absolute;
        padding: 0;
        margin: 0;
        left: -100%;
        .options{
          padding: 0;
          margin: 0;
          margin-top: 8px;
          border-radius: 6px;
          box-shadow: 0 0 6px rgba(0,0,0,0.1);
          background: white;
          overflow: hidden;
          li{
            margin: 0;
            list-style: none;
            padding: 5px 16px;
            white-space: nowrap;
            font-size: 16px;
            color: #333;
            &:hover{
              background: #f0f0f0;
            }
          }
        }
      }
    }
    .item-hr{
      margin-right: 10px;
      height: 100%;
      width: 1px;
      border-right: 1px solid #ddd;
    }
  }
  .leftIcons{
    .item{
      margin-right: 6px;
      &:last-child{
        margin-right: 0;
      }
    }
  }
  .rightIcons{
    .item{
      margin-left: 6px;
      &:first-child{
        margin-left: 0;
      }
    }
  }
}

.addImgContainer{
  z-index: 2000;
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  .addImgContainer__warp{
    height: 100%;
    width: 100%;
    background: rgba(0,0,0,0.5);
    cursor: pointer;
  }
  .addImgContainer__content{
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 500px;
    height: 300px;
    background: white;
    border-radius: 8px;
    box-shadow: 0 0 6px rgba(0,0,0,0.1);
    padding: 20px;
    display: flex;
    flex-direction: column;
    .addImgContainer__content__title{
      display: flex;
      justify-content: center;
    }
    .addImgContainer__content__input{
      display: flex;
      flex-direction: column;
      margin: 30px 0;
      input{
        height: 50px;
        border: 1px solid #ccc;
        border-radius: 6px;
        padding: 0 10px;
        margin-bottom: 10px;
        display: flex;
        align-items: center;
      }
      .uploadImgInput{
        cursor: pointer;
      }
    }
    .buttonBox{
      display: flex;
      justify-content: center;
      .buttonBox__cancel{
        margin-right: 28px;
        padding: 6px 16px;
        border-radius: 6px;
        color: #aaa;
        border: 1px solid #ccc;
        cursor: pointer;
        transition: 0.3s ease;
         user-select: none;
        &:hover{
          color: #333;
        }
      }
      .buttonBox__add{
         padding: 6px 16px;
         border-radius: 6px;
         background: rgb(64, 158, 255);
         color: white;
         cursor: pointer;
         transition: 0.3s ease;
         user-select: none;
         &:hover{
          background: rgb(44, 130, 214);
         }
      }
    }
  }
}
</style>
