<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import axios from 'axios'
const category_options = ref<string[]>([])
// 获取数据
const getData = () => {
  axios
    .get('/api/idea/category', {
      params: {
        uuid: authStore.user?.uuid,
        page: 1,
        pagesize: 999,
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
const nowTime = ref<string>('')
onMounted(() => {
  getData()
  const date = new Date();
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  nowTime.value = `${year}-${month}-${day}`;
})
type Idea = {
  category: string
  tags: string
  content: string
  status: number
  isTop: number
}
const idea = ref<Idea>({
  category: '突发奇想',
  tags: '',
  content: '',
  status: 1,
  isTop: 0,
})
let throttleTimer: ReturnType<typeof setTimeout> | null = null;
watch(idea.value, (newVal) => {
  if (newVal.tags[0] === ';') {
    if (!throttleTimer) {
      ElMessage.error('闪念标签不能以分号开头');
      throttleTimer = setTimeout(() => {
        throttleTimer = null;
      }, 1000);
    }
    idea.value.tags = idea.value.tags.slice(1)
    return
  }
})

const handleReleaseIdea = () => {
  if (!idea.value.category) {
    ElMessage.error('请选择闪念分类')
    return
  }
  if (!idea.value.content) {
    ElMessage.error('请输入闪念内容')
    return
  }
  axios
    .post(`/api/idea/add?uuid=${authStore.user?.uuid}`, {
      ...idea.value,
    },
    {
      headers: {
        Authorization: authStore.token,
      }
    })
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('发布成功')
      } else {
        ElMessage.error(`发布失败，请稍后再试：{${res.data.message}}`)
      }
    })
    .catch((err) => {
      ElMessage.error(`发布失败，请稍后再试：{${err.message}}`)
    })
}
</script>

<template>
  <div class="writeIdea">
    <div class="pageTitle">· 写闪念 - 发布闪念</div>
    <div class="ideaInfo">
      <div class="ideaInfoTitle">
        <div class="ideaOtherInfo">
          <div class="category">
            <span>闪念分类：</span>
            <el-select v-model="idea.category" placeholder="请选择闪念分类" style="width: 160px">
              <el-option v-for="item in category_options" :key="item" :label="item" :value="item" />
              <template #footer>
                <el-input
                  v-model="idea.category"
                  class="option-input"
                  placeholder="input option name"
                  size="small"
                  maxLength="10"
                />
              </template>
            </el-select>
          </div>
          <div class="tag">
            <span>闪念标签：</span>
            <el-input
              placeholder="请输入闪念标签-多个标签请用英文分号隔开"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="idea.tags"
              maxlength="20"
            />
          </div>
        </div>
        <div class="ideaType">
          <div class="isTop">
            <span><span style="color: red">*</span>是否置顶：</span>
            <el-radio-group v-model="idea.isTop">
              <el-radio :value="1">是</el-radio>
              <el-radio :value="0">否</el-radio>
            </el-radio-group>
          </div>
          <div class="visibleTypes">
            <span><span style="color: red">*</span>可见性：</span>
            <el-radio-group v-model="idea.status">
              <el-radio :value="0">公开</el-radio>
              <el-radio :value="1">私密</el-radio>
            </el-radio-group>
          </div>
          <div class="release">
            <el-button type="primary" @click="handleReleaseIdea">发布闪念</el-button>
          </div>
        </div>
      </div>
      <div class="ideaContent">· 闪念内容</div>
      <div class="ideaWriteContent">
        <div class="writeArea">
          <textarea
            name="content"
            id="ideaWriteInput"
            v-model="idea.content"
            placeholder="开始编辑..."
            maxlength="100"
          ></textarea>
        </div>
        <div class="viewArea">
          <div class="noteArea">
            <div class="noteAreaTop">
              <div class="circleBox">
                <div class="circle"></div>
                <div class="circle"></div>
                <div class="circle"></div>
              </div>
              <div class="idea_time">{{ nowTime }}</div>
            </div>
            <div class="idea_preview_content">
              <div class="idea_text">
                <!-- <p class="idea_package">“</p> -->
                <p v-html="idea.content"></p>
                <!-- <p class="idea_package">”</p> -->
              </div>
              <div class="idea_categoryAndTag">
                <div class="idea_item">📁 {{ idea.category }}</div>
                <div
                  class="idea_item tag"
                  v-for="(item, index) in idea.tags.split(';')"
                  :key="index"
                  v-show="idea.tags.length > 0"
                  :class="{ tagFirst: index === 0 }"
                >
                  #{{ item }}
                </div>
              </div>
            </div>
            <div class="idea_preview_bottom">
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.writeIdea {
  width: 100%;
  padding: 16px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  .pageTitle {
    font-size: 18px;
    color: var(--Main_font_SecondColor);
  }
  .ideaInfo {
    width: 100%;
    margin-top: 10px;
    .ideaInfoTitle {
      --marginTopBottom: 16px;
      width: 100%;
      .ideaOtherInfo {
        display: flex;
        align-items: center;
        width: 100%;
        .category,
        .tag {
          display: flex;
          align-items: center;
          margin-right: 10px;
          color: var(--Main_font_SecondColor);
          &:last-child {
            margin-right: 0;
          }
        }
      }
      .ideaType {
        display: flex;
        .isTop,
        .visibleTypes {
          margin: 10px 0;
          display: flex;
          align-items: center;
          margin-left: 30px;
          color: var(--Main_font_SecondColor);
        }
        .isTop {
          margin-left: 0;
        }
        .release{
          margin: 10px 0;
          margin-left: auto;
          margin-right: 50px;
        }
      }
    }
  }
}
.ideaContent {
  margin-top: 0px;
  color: var(--Main_font_SecondColor);
}
.ideaWriteContent {
  width: 100%;
  height: 640px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  overflow: hidden;
  margin-top: 18px;
  .writeArea {
    width: 50%;
    height: 100%;
    background-color: var(--Main_Bg_BoxColor);
    #ideaWriteInput {
      flex: 1;
      height: 100%;
      width: 100%;
      resize: none;
      box-sizing: border-box;
      padding: 10px;
      border: none;
      outline: none;
      font-size: 18px;
      padding-bottom: 50vh;
      color: var(--Main_font_SecondColor);
      font-family:
        Inter,
        -apple-system,
        BlinkMacSystemFont,
        'Segoe UI',
        Roboto,
        Oxygen,
        Ubuntu,
        Cantarell,
        'Fira Sans',
        'Droid Sans',
        'Helvetica Neue',
        sans-serif;
    }
  }
  .viewArea {
    width: 50%;
    height: 100%;
    .noteArea {
      width: 60%;
      background-color: #f29c17aa;
      border-radius: 8px;
      box-shadow: 0 0 10px rgba(0, 0, 0, 0.2);
      overflow: hidden;
      padding: 10px 20px 20px 20px;
      display: flex;
      flex-direction: column;
      margin: 0 auto;
      .noteAreaTop {
        height: 38px;
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        .circleBox{
          height: 100%;
          display: flex;
          align-items: center;
          .circle {
            --wh: 20px;
            width: var(--wh);
            height: var(--wh);
            border-radius: 50%;
            background-color: #fff;
            position: relative;
            margin-left: 10px;
            &:first-child {
              margin-left: 0;
            }
          }
        }
        .idea_time{
          font-size: 20px;
          color: var(--Main_font_SecondColor);
        }
      }
      .idea_preview_content {
        flex: 1;
        padding: 10px 0;
        .idea_text {
          color: #333;
          line-height: 31px;
          text-indent: 2rem;
          font-size: 18px;
          word-break: break-all;
          color: var(--Main_font_SecondColor);
          .idea_package{
            font-size: 28px;
            line-height: 28px;
            padding-top: 8px;
            font-weight: bold;
          }
        }
        .idea_categoryAndTag {
          width: 100%;
          margin-top: 30px;
          display: flex;
          align-items: center;
          flex-wrap: wrap;
          .idea_item{
            margin-left: 6px;
            font-size: 18px;
            color: var(--Main_font_SecondColor);
            text-shadow: 0 0 10px rgba(0, 0, 0, 0.2);
            &:first-child {
              margin-left: 0;
            }
            &.tagFirst {
              margin-left: 10px;
            }
          }
        }
      }
      .idea_preview_bottom {
        padding-top: 10px;
        width: 100%;
        display: flex;
      }
    }
  }
}
</style>
