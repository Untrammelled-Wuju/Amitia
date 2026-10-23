<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElConfigProvider, ElMessage } from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'dayjs/locale/zh-cn'
import axios from 'axios'
import { useAuthStore } from '@/stores/authStore'
const authStore = useAuthStore()

const dialogFormVisible = ref(false)
const shortcuts = [
  {
    text: '上周',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setDate(start.getDate() - 7)
      return [start, end]
    },
  },
  {
    text: '上月',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setMonth(start.getMonth() - 1)
      return [start, end]
    },
  },
  {
    text: '前3个月',
    value: () => {
      const end = new Date()
      const start = new Date()
      start.setMonth(start.getMonth() - 3)
      return [start, end]
    },
  },
]

const timeData = ref<Array<string>>([])
interface MenuDataModel {
  value: string
  orderWay: string
  isDeleted: string
  startTime: string
  endTime: string
  status: number
}
const menuData = ref<MenuDataModel>({
  value: '',
  orderWay: '',
  isDeleted: '',
  startTime: '',
  endTime: '',
  status: 1,
})

let timeoutId: ReturnType<typeof setTimeout>
const emit = defineEmits(['updateMenuData'])
watch(menuData.value, (newVal) => {
  clearTimeout(timeoutId)
  timeoutId = setTimeout(() => {
    emit('updateMenuData', newVal)
  }, 300)
})

watch(timeData, (newVal) => {
  if (newVal && newVal.length >= 2) {
    menuData.value.startTime = newVal[0] || ''
    menuData.value.endTime = newVal[1] || ''
  } else {
    menuData.value.startTime = ''
    menuData.value.endTime = ''
  }
})
const addForm = reactive({
  name: '',
  description: '',
  url: '',
  avatar: '',
  sort: 0,
  status: 1,
})
const subMitForm = () => {
  axios
    .post('/api/friendLink/add', addForm, {
      params: {
        uuid: authStore.user?.uuid,
      },
      headers: {
        Authorization: authStore.token,
      },
    })
    .then((res) => {
      if (res.data.code === 200) {
        ElMessage.success('新增成功')
        dialogFormVisible.value = false
        emit('updateMenuData', menuData.value)
      } else {
        ElMessage.error(`新增失败，请稍后再试：{${res.data.message}}`)
      }
    })
    .catch((err) => {
      ElMessage.error(`新增失败，请稍后再试：{${err.message}}`)
    })
}
</script>

<template>
  <div class="articleListMenu">
    <div class="sorting">
      <div class="item">
        <span>发布时间：</span>
        <el-config-provider :locale="zhCn">
          <el-date-picker
            v-model="timeData"
            type="daterange"
            :shortcuts="shortcuts"
            range-separator="—"
            format="YYYY-MM-DD"
            value-format="YYYY-MM-DD"
            start-placeholder="起始时间"
            end-placeholder="结束时间"
            clearable
          />
        </el-config-provider>
      </div>
      <div class="item">
        <span>可见状态：</span>
        <el-select v-model="menuData.status" placeholder="可见状态" style="width: 100px" clearable>
          <el-option label="公开" value="1" />
          <el-option label="私密" value="0" />
        </el-select>
      </div>
      <div class="addStudent" @click="dialogFormVisible = true">新增友链</div>
    </div>
    <div class="sorting">
      <div class="item">
        <span>搜索：</span>
        <el-input
          v-model="menuData.value"
          style="width: 320px"
          placeholder="请输入搜索内容"
          clearable
        >
          <template #prefix>
            <el-icon class="el-input__icon"><search /></el-icon>
          </template>
        </el-input>
        <el-button type="primary" style="margin-left: 10px">搜索</el-button>
        <el-button type="danger" style="margin-left: 10px">重置</el-button>
      </div>
    </div>
  </div>
  <el-dialog v-model="dialogFormVisible" title="请填写站点信息" width="500">
    <el-form :model="addForm">
      <el-form-item label="* 站点名称">
        <el-input v-model="addForm.name" autocomplete="off" />
      </el-form-item>
      <el-form-item label="* 站点描述">
        <el-input v-model="addForm.description" autocomplete="off" />
      </el-form-item>
      <el-form-item label="* 站点地址">
        <el-input v-model="addForm.url" autocomplete="off" />
      </el-form-item>
      <el-form-item label="* 站点头像">
        <el-input v-model="addForm.avatar" autocomplete="off" />
      </el-form-item>
      <el-form-item label="* 站点权重">
        <el-input v-model="addForm.sort" autocomplete="off" />
      </el-form-item>
      <el-form-item label="* 站点可见状态">
        <el-select v-model="addForm.status" placeholder="可见状态" style="width: 100px">
          <el-option label="公开" :value="1" />
          <el-option label="私密" :value="0" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <div class="dialog-footer">
        <el-button @click="dialogFormVisible = false">取消</el-button>
        <el-button type="primary" @click="subMitForm"> 提交 </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.articleListMenu {
  width: 100%;
  background: var(--Main_Bg_BoxColor);
  border-radius: var(--BorderRadius);
  box-shadow: var(--Main_boxShadow);
  padding: 0 16px;
  .sorting {
    width: 100%;
    height: 48px;
    display: flex;
    align-items: center;
    .item {
      display: flex;
      align-items: center;
      margin-right: 20px;
      color: var(--Main_font_SecondColor);
    }
    > div {
      &:last-child {
        margin-right: 0;
      }
    }
    .addStudent {
      user-select: none;
      width: 80px;
      padding: 5px 0;
      border-radius: var(--BorderRadius);
      background-color: rgb(97, 141, 247);
      color: white;
      box-shadow: 0px 0px 6px var(--BoxShadowColor);
      display: flex;
      align-items: center;
      justify-content: center;
      cursor: pointer;
      transition: var(--transitionTime);
      margin-left: auto;
      &:hover {
        background-color: rgb(59, 107, 220);
      }
    }
  }
}
</style>
