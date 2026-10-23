<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()
import axios from 'axios'
type Sentence = {
  content: string
  status: number
  author: string
}
const sentence = ref<Sentence>({
  content: '',
  status: 1,
  author: '',
})
const clearSentence = () => {
  sentence.value.content = ''
  sentence.value.status = 1
  sentence.value.author = ''
}
// 新增句子
const submitSentence = () => {
  if (!sentence.value.content) {
    ElMessage.error('请输入句子内容')
    return
  }
  if (!sentence.value.author) {
    ElMessage.error('请输入作者')
    return
  }
  ElMessageBox.alert('确定新增句子吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning',
  })
    .then((action) => {
      if (action === 'confirm') {
        axios
          .post(
            `/api/sentence/add?uuid=${authStore.user?.uuid}`,
            {
              ...sentence.value,
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
              ElMessageBox.confirm('是否清空编辑区句子？', '提示', {
                confirmButtonText: '确定',
                cancelButtonText: '取消',
                type: 'warning',
              }).then((action) => {
                if (action === 'confirm') {
                  clearSentence()
                }
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
  <div class="writeSentence">
    <div class="pageTitle">· 句子发布 - 新增句子</div>
    <div class="sentencesInfo">
      <div class="sentencesInfoTitle">
        <div class="sentencesTitle">
          <span style="color: red">*</span>句子内容
          <el-button class="releaseBtn" type="primary" size="default" @click="submitSentence"
            >发布</el-button
          >
          <el-button class="releaseBtn" type="danger" size="default" @click="clearSentence"
            >清空全部</el-button
          >
        </div>
        <el-input
          v-model="sentence.content"
          placeholder="请输入句子内容"
          type="textarea"
          :rows="4"
          maxlength="100"
          show-word-limit
        />
        <div class="sentencesOtherInfo">
          <div class="sentencesDescription">
            <span style="color: red">*</span>
            作者
            <el-input
              placeholder="请输入作者"
              clearable
              size="default"
              style="width: 380px; margin-left: 10px"
              v-model="sentence.author"
            />
          </div>
        </div>
        <div class="sentencesType">
          <div class="visibleTypes">
            <span><span style="color: red">*</span>可见性：</span>
            <el-radio-group v-model="sentence.status">
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
.writeSentence {
  width: 100%;
  padding: 16px;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  .pageTitle {
    font-size: 18px;
    color: var(--Main_font_SecondColor);
  }
  .sentencesInfo {
    width: 100%;
    margin-top: 10px;
    .sentencesInfoTitle {
      --marginTopBottom: 16px;
      width: 100%;
      .sentencesTitle {
        display: flex;
        align-items: center;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .releaseBtn {
          margin-left: 10px;
        }
      }
      .sentencesOtherInfo {
        display: flex;
        align-items: center;
        width: 100%;
        margin: var(--marginTopBottom) 0;
        color: var(--Main_font_SecondColor);
        .sentencesDescription {
          display: flex;
          align-items: center;
          margin-right: 10px;
          color: var(--Main_font_SecondColor);
          &:last-child {
            margin-right: 0;
          }
        }
      }
      .sentencesCover {
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
        .sentencesCoverImg {
          max-height: 80px;
          border-radius: var(--BorderRadius);
          border: 1.5px solid var(--Main_BoxBorderColor);
          margin-top: 10px;
        }
      }
      .sentencesUrl {
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
      .sentencesType {
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
