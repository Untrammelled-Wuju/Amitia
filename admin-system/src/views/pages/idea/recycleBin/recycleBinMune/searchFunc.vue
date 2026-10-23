<script setup lang="ts">
import { ref, watch } from 'vue'
const emit = defineEmits(['serachValue']);


const searchData = ref({
  searchValue: '',
  sortValue: 'createTime',
  orderWay: '',
})

let timeoutId: ReturnType<typeof setTimeout>
watch(searchData.value, (newVal) => {
  clearTimeout(timeoutId)
  timeoutId = setTimeout(() => {
    emit('serachValue', newVal)
  }, 300)
})
const searchFunc = () => {
  emit('serachValue', searchData.value)
}
</script>
<template>
  <div class="outsideBox">
    <div id="SearchBox">
      <input
        type="text"
        placeholder="输入文章标题"
        v-model="searchData.searchValue"
      />
      <button @click="searchFunc">搜索</button>
    </div>
    <div class="sort">
      <el-select v-model="searchData.sortValue" placeholder="排序">
        <el-option label="按创建时间排序" value="createTime"></el-option>
      </el-select>
      <el-select v-model="searchData.orderWay" placeholder="排序方式" style="margin-left: 10px;">
        <el-option label="升序" value="asc"></el-option>
        <el-option label="降序" value="desc"></el-option>
      </el-select>
    </div>
  </div>
</template>
<style scoped>
.outsideBox{
  height: 100%;
  display: flex;
}
#SearchBox {
  height: 100%;
  width: 300px;
  display: flex;
  border-radius: var(--BorderRadius);
  overflow: hidden;
  border: 0.5px solid #ccc;
  padding: 0 6px;
  box-shadow: 0 0 2px var(--BoxShadowColor);
  button {
    outline: none;
    border: none;
    background: none;
    height: 100%;
    width: 38px;
    cursor: pointer;
    color: var(--Main_font_SecondColor);
  }
  input {
    height: 100%;
    width: calc(100% - 38px);
    outline: none;
    border: none;
    background: none;
    padding: 0 6px;
    padding-top: 3px;
    font-size: 14px;
    color: #333;
    &:focus {
      outline: none;
    }
  }
}
.sort{
  height: 100%;
  display: flex;
  align-items: center;
  width: 320px;
  margin-left: 20px;
}
</style>
