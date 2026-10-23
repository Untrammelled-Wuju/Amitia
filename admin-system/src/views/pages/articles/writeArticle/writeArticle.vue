<script setup lang="ts">
import { ref, watch } from 'vue'
import textEditor from './textEditor/textEditor.vue'
import { ElMessage, ElMessageBox, ElNotification } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import axios from 'axios'
import { onMounted } from 'vue'
import { blogInfo } from '@/utils/blogInfo'
interface Category {
  id: number
  name: string
}
const category_options = ref<Category[]>([{ name: '默认分类', id: 1 }])
// 获取数据
const getData = () => {
  axios
    .get('/api/article/category/list', {
      params: {
        page: 1,
        pagesize: 999,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        category_options.value = res.data.data.list
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
interface Tag {
  id: string
  name: string
  count: number
  createAt: string
}
const tag_options = ref<Tag[]>([])
function getTagOptions() {
  axios
    .get('/api/article/tag/list', {
      params: {
        page: 1,
        pagesize: 999,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        tag_options.value = res.data.data.list
      } else {
        ElMessage.error(res.data.msg)
        console.log(res.data)
      }
    })
    .catch((err) => {
      ElMessage.error('获取标签失败', err.msg)
      console.log(err)
    })
}
getTagOptions()
type Article = {
  title: string
  description: string
  categoryId: number
  tags: string[]
  content: string
  status: number
  cover: string
  isComment: number
  isTop: number
  isCarousel: number
  author: string
  type: number
  url: string
}
const article = ref<Article>({
  title: '',
  description: '',
  categoryId: 1,
  tags: [],
  content: localStorage.getItem('temporaryArticle') || '',
  status: 2,
  cover: '',
  isComment: 0,
  isTop: 1,
  isCarousel: 0,
  author: '',
  type: 0,
  url: '',
})
if (localStorage.getItem('articleInfo') != null) {
  article.value = JSON.parse(localStorage.getItem('articleInfo') || 'null')
}
watch(
  () => article.value.type,
  (newType) => {
    if (newType !== 1 && newType !== 0) {
      article.value.isTop = 0
      article.value.isCarousel = 0
      article.value.tags = []
      article.value.categoryId = 1
      article.value.author = blogInfo.value.author
      article.value.url = ''
    }
  },
)

// 2. 监视整个文章对象变化，处理本地存储(使用deep:true深度监视)
watch(
  article,
  (newValue) => {
    localStorage.setItem('articleInfo', JSON.stringify(newValue))
  },
  { deep: true },
)
const newTags = ref<string[]>([])
const addNewTag = () => {
  if (newTags.value.length === 0) {
    ElMessage.error('请输入标签')
    return
  }
  if (authStore.isAuthenticated && authStore.user) {
    axios
      .post(
        `/api/article/tag/add?uuid=${authStore.user.uuid}`,
        {
          tags: newTags.value,
        },
        {
          headers: {
            Authorization: localStorage.getItem('token') || '',
          },
        },
      )
      .then((res) => {
        if (res.data.code === 200) {
          ElMessage.success('新增成功')
          newTags.value = []
          getTagOptions()
        } else {
          ElMessage.error(`新增失败：${res.data.msg}`)
        }
      })
      .catch((err) => {
        ElMessage.error('新增失败', err.msg)
      })
  }
}
const newCategory = ref<string>('')
const addNewCategory = () => {
  axios
    .post(
      `/api/article/category/add?uuid=${authStore.user?.uuid}`,
      {
        categoryName: newCategory.value,
      },
      {
        headers: {
          Authorization: localStorage.getItem('token') || '',
        },
      },
    )
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('新增成功')
        newCategory.value = ''
        getData()
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
// 处理文章封面
const uploadCoverInput = ref<HTMLInputElement>()
const stringImg = ref<string>(article.value.cover)
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
    submitArticleCover()
  }
}
// 上传文章封面
const submitArticleCover = () => {
  axios
    .post(
      `/api/article/uploadCover?uuid=${JSON.parse(localStorage.getItem('user') || '{}').uuid}`,
      imgFormData,
      {
        headers: {
          'Content-Type': 'multipart/form-data',
          Authorization: localStorage.getItem('token') || '',
        },
      },
    )
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('图片上传成功')
        article.value.cover = res.data.data
        stringImg.value = article.value.cover
        localStorage.setItem('articleInfo', JSON.stringify(article.value))
      }
    })
    .catch((error) => {
      ElMessage.error('图片上传失败: ' + (error.response?.data?.message || error.message))
    })
}
const clearArticle = () => {
  article.value = {
    title: '',
    description: '',
    categoryId: 1,
    tags: [],
    content: '',
    status: 2,
    cover: '',
    isComment: 0,
    isTop: 1,
    isCarousel: 0,
    author: '',
    type: 0,
    url: '',
  }
  localStorage.removeItem('articleInfo')
  localStorage.removeItem('temporaryArticle')
  stringImg.value = ''
  uploadCoverInput.value = undefined
  imgFormData.delete('file')
}
// 发布文章
const submitArticle = () => {
  if (!article.value.title) {
    ElMessage.error('请输入文章标题')
    return
  }
  if (!article.value.description) {
    ElMessage.error('请输入文章摘要')
    return
  }
  if (article.value.content === '') {
    ElMessage.error('请输入文章内容')
    return
  }
  if (stringImg.value === '') {
    ElMessage.error('请上传文章封面')
    return
  }
  if (article.value.type === 1) {
    if (article.value.author === '' || article.value.url === '') {
      ElMessage.error('转载文章请标明作者和原文连接')
      return
    }
  }
  if (article.value.type !== 1) {
    article.value.author = blogInfo.value.author
  }
  ElMessageBox.alert('确定发布文章吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
    .then((action) => {
      if (action === 'confirm') {
        axios
          .post(
            `/api/article/add?uuid=${authStore.user?.uuid}`,
            {
              ...article.value,
            },
            {
              headers: {
                Authorization: localStorage.getItem('token') || '',
              },
            },
          )
          .then((res) => {
            if (res.data.code === 200) {
              getCarouseNum()
              ElMessage.success('发布成功')
              ElMessageBox.alert('发布成功，是否清空编辑区？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'info',
              }).then((action) => {
                if (action === 'confirm') {
                  clearArticle()
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
const carouselMaxNum = ref<number>(0)
const carouselNum = ref<number>(0)
const getCarouseNum = () => {
  axios
    .get(`/api/article/carouselNum?uuid=${authStore.user?.uuid}`, {
      headers: {
        Authorization: authStore.token,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        carouselNum.value = res.data.data
        if (carouselNum.value >= carouselMaxNum.value) {
          article.value.isCarousel = 0
        }
      }
    })
    .catch((err) => {
      ElNotification({
        title: '获取轮播图数量失败',
        message: err.message,
        type: 'error',
        position: 'top-left',
      })
    })
}
onMounted(() => {
  getCarouseNum()
  axios
    .get(`/api/article/carouselMaxNum?uuid=${authStore.user?.uuid}`, {
      headers: {
        Authorization: authStore.token,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        carouselMaxNum.value = res.data.data
      }
    })
    .catch((err) => {
      ElNotification({
        title: '获取最大轮播图数量失败',
        message: err.message,
        type: 'error',
        position: 'top-left',
      })
    })
})
</script>

<template>
  <div class="writeArticle">
    <div class="pageTitle">· 写文章 - 发布文章</div>
    <div class="articleInfo">
      <div class="articleInfoTitle">
        <div class="articleTitle">
          <span style="color: red">*</span>文章标题<el-input
            v-model="article.title"
            style="width: 320px; margin-left: 10px"
            placeholder="请输入文章标题"
            clearable
            size="default"
          />
          <el-button class="releaseBtn" type="primary" size="default" @click="submitArticle"
            >发布</el-button
          >
          <el-button class="releaseBtn" type="danger" size="default" @click="clearArticle"
            >清空全部</el-button
          >
        </div>
        <div class="articleOtherInfo">
          <div class="item">
            <span style="color: red">*</span>文章摘要<el-input
              placeholder="请输入文章摘要"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="article.description"
            />
          </div>
          <div class="item" v-if="article.type === 1 || article.type === 0">
            <span>文章分类：</span>
            <el-select
              v-model="article.categoryId"
              placeholder="请选择文章分类"
              style="width: 160px"
            >
              <el-option
                v-for="item in category_options"
                :key="item.id"
                :label="item.name"
                :value="item.id"
              />
              <template #footer>
                <el-input v-model="newCategory" trigger="Space" placeholder="添加新分类" />
                <el-button
                  type="primary"
                  size="small"
                  @click="addNewCategory"
                  style="margin-top: 10px"
                >
                  添加
                </el-button>
              </template>
            </el-select>
          </div>
          <div class="item" v-if="article.type === 1 || article.type === 0">
            <span>文章标签：</span>
            <el-select
              v-model="article.tags"
              multiple
              collapse-tags
              collapse-tags-tooltip
              placeholder="请选择文章标签"
              style="width: 160px"
            >
              <el-option
                v-for="item in tag_options"
                :key="item.id"
                :label="`${item.name}(${item.count})`"
                :value="item.id"
              />
              <template #footer>
                <el-input-tag v-model="newTags" trigger="Space" placeholder="添加新标签-空格分隔" />
                <el-button type="primary" size="small" @click="addNewTag" style="margin-top: 10px">
                  添加
                </el-button>
              </template>
            </el-select>
          </div>
        </div>
        <div class="articleCover">
          <div>
            <span><span style="color: red">*</span>文章封面：</span>
            <input
              type="file"
              accept="image/*"
              ref="uploadCoverInput"
              @change="handleCoverChange"
            />
          </div>
          <img class="articleCoverImg" :src="stringImg" alt="" />
        </div>
        <div class="articleType">
          <div class="item">
            <span><span style="color: red">*</span>开放评论：</span>
            <el-radio-group v-model="article.isComment">
              <el-radio :value="0">是</el-radio>
              <el-radio :value="1">否</el-radio>
            </el-radio-group>
          </div>
          <div class="item" v-if="article.type === 1 || article.type === 0">
            <span><span style="color: red">*</span>是否置顶：</span>
            <el-radio-group v-model="article.isTop">
              <el-radio :value="0">否</el-radio>
              <el-radio :value="1">是</el-radio>
            </el-radio-group>
          </div>
          <div class="item">
            <span><span style="color: red">*</span>可见性：</span>
            <el-radio-group v-model="article.status">
              <el-radio :value="0">公开</el-radio>
              <el-radio :value="1">私密</el-radio>
              <el-radio :value="2">草稿箱</el-radio>
            </el-radio-group>
          </div>
          <div class="item" v-if="article.type === 1 || article.type === 0">
            <span
              ><span style="color: red">*</span>是否进行轮播({{ carouselNum }}/{{
                carouselMaxNum
              }})：</span
            >
            <el-radio-group v-model="article.isCarousel" :disabled="carouselNum >= carouselMaxNum">
              <el-radio :value="0">否</el-radio>
              <el-radio :value="1">是</el-radio>
            </el-radio-group>
          </div>
          <div class="item">
            <span><span style="color: red">*</span>文章类型：</span>
            <el-radio-group v-model="article.type">
              <el-radio :value="0">普通文章</el-radio>
              <el-radio :value="1">转载文章</el-radio>
              <el-radio :value="2">关于本站</el-radio>
              <el-radio :value="3">版权协议</el-radio>
              <el-radio :value="4">隐私政策</el-radio>
            </el-radio-group>
          </div>
          <div class="item" v-if="article.type === 1">
            文章作者（转载必填）<el-input
              placeholder="请输入文章作者"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="article.author"
            />
          </div>
          <div class="item" v-if="article.type === 1">
            文章链接（转载必填）<el-input
              placeholder="请输入文章链接"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="article.url"
            />
          </div>
        </div>
      </div>
      <div class="articleContent">· 文章内容</div>
      <textEditor v-model="article.content" />
    </div>
  </div>
</template>

<style scoped>
.writeArticle {
  width: 100%;
  padding: 16px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  .pageTitle {
    font-size: 18px;
    color: var(--Main_font_SecondColor);
  }
  .articleInfo {
    width: 100%;
    margin-top: 10px;
    .articleInfoTitle {
      --marginTopBottom: 16px;
      width: 100%;
      .articleTitle {
        display: flex;
        align-items: center;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .releaseBtn {
          margin-left: 10px;
        }
      }
      .articleOtherInfo {
        display: flex;
        align-items: center;
        width: 100%;
        margin: var(--marginTopBottom) 0;
        .item {
          display: flex;
          align-items: center;
          margin-right: 10px;
          color: var(--Main_font_SecondColor);
          &:last-child {
            margin-right: 0;
          }
        }
      }
      .articleCover {
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
        .articleCoverImg {
          max-height: 80px;
          border-radius: var(--BorderRadius);
          border: 1.5px solid var(--Main_BoxBorderColor);
          margin-top: 10px;
        }
      }
      .articleType {
        margin: var(--marginTopBottom) 0;
        .item {
          margin: 10px 0;
          display: flex;
          align-items: center;
          color: var(--Main_font_SecondColor);
        }
      }
    }
  }
}
.articleContent {
  margin-top: 10px;
  color: var(--Main_font_SecondColor);
}
</style>
