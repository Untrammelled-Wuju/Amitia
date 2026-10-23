<script setup lang="ts">
// import axios from 'axios'
import { ElTable, ElPagination, /* ElMessageElMessage */ } from 'element-plus'
import { ref } from 'vue'
const total = ref(1)
const currentPage = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const OperationLogData = ref([
  {
    user: 'admin',
    networkOperator: '中国电信',
    ip: '127.0.0.1',
    location: '北京市朝阳区',
    time: '2022-01-01 12:00:00',
  },
  {
    user: 'admin',
    networkOperator: '中国移动',
    ip: '127.0.0.1',
    location: '北京市朝阳区',
    time: '2022-01-01 12:00:00',
  },
  {
    user: 'admin',
    networkOperator: '中国联通',
    ip: '127.0.0.1',
    location: '北京市朝阳区',
    time: '2022-01-01 12:00:00',
  },
])

const getLoginLogData = () => {
  // axios
  //   .get('/api/auth/logs/login', {
  //     headers: {
  //       Authorization: localStorage.getItem('token'),
  //     },
  //   })
  //   .then((res) => {
  //     // 新增日志解析逻辑
  //     const logPattern = /"msg":"{(.*?)}".*"time":"(.*?)"/
  //     const data = res.data.split('\n').filter(Boolean)

  //     OperationLogData.value = data
  //       .map((log) => {
  //         const match = log.match(logPattern)
  //         if (match) {
  //           const fields = match[1].split(',')
  //           const logObj = {}

  //           fields.forEach((field) => {
  //             const [key, value] = field.split(':')
  //             const cleanKey = key.trim().replace(/["\\]/g, '') // 同时去除双引号和反斜杠
  //             const cleanValue = value.trim().replace(/["\\]/g, '') // 新增反斜杠处理
  //             logObj[cleanKey] = cleanValue
  //           })

  //           return {
  //             user: logObj.用户 || '未知',
  //             networkOperator: logObj.operator || '未知',
  //             ip: logObj.ip || '0.0.0.0',
  //             location: logObj.location || '未知地区',
  //             time: match[2],
  //           }
  //         }
  //         return null
  //       })
  //       .filter(Boolean)
  //   })
  //   .catch((err) => {
  //     ElMessage.error(err)
  //     console.log(err)
  //   })
}
getLoginLogData()
</script>
<template>
  <div id="loginLogPage">
    <el-table :data="OperationLogData" border style="width: 100%" height="710" v-loading="loading">
      <el-table-column prop="user" label="用户" :min-width="160" />
      <el-table-column prop="networkOperator" label="网络运营商" :min-width="120" />
      <el-table-column prop="ip" label="IP地址" :min-width="180" />
      <el-table-column prop="location" label="详细信息" :min-width="580" />
      <el-table-column prop="time" label="登录时间" :min-width="260" />
    </el-table>
    <el-pagination
      background
      layout="total, sizes, prev, pager, next, jumper"
      :total="total"
      v-model:current-page="currentPage"
      v-model:page-size="pageSize"
      style="margin-top: 20px; justify-content: center"
      :page-sizes="[10, 20, 50, 100, 200]"
    />
  </div>
</template>
<style scoped>
.el-table {
  flex: 1;
}
</style>
