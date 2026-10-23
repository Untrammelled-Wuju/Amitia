import axios from "axios";
import { ElNotification } from "element-plus";
import { onMounted, ref } from "vue";
type BlogInfo = {
  // 博客信息
  version: string
  recordNumber: string
  proverb: string
  name: string
  title: string
  desc: string
  darkBg: string
  lightBg: string
  author: string
  wechatPay: string
  alipay: string
  blogCreateTime: string
  blogUpdateTime: string
  avatar: string
  email: string
  github: string
  gitee: string
  wechat: string
  qq: string
  telegram: string
  bilibili: string
  twitter: string
  totalVisits: number
  // 统计信息
  totalWordCount: number
  articleCount: number
  tagsCount: number
  textCount: number
  // 默认值
  defaultAvatar: string
}
export const blogInfo = ref<BlogInfo>({
  // 博客信息
  version: '',
  recordNumber: "--",
  proverb: '',
  name: '',
  title: '',
  desc: '',
  darkBg: '',
  lightBg: '',
  author: '',
  totalVisits: 0,
  blogCreateTime: '',
  blogUpdateTime: '',
  avatar: '/src/assets/images/avatar.png',
  email: '',
  github: '',
  gitee: '',
  wechat: '',
  wechatPay: '',
  alipay: '',
  telegram: '',
  bilibili: '',
  twitter: '',
  qq: '',
  // 统计信息
  totalWordCount: 0,
  articleCount: 0,
  tagsCount: 0,
  textCount: 0,
  // 默认值
  defaultAvatar: '/src/assets/images/avatar.png'
})
// 恢复网站信息
const saveBlogInfo = () => {
  localStorage.setItem('blogInfo', JSON.stringify(blogInfo.value))
}
if(localStorage.getItem('blogInfo')) {
  blogInfo.value = JSON.parse(localStorage.getItem('blogInfo') || '{}')
}
// 获取网站信息
axios.get('/webInfo/info')
.then(res => {
  if (res.data.code === 200) {
    const resData = res.data.data;
    blogInfo.value.version = resData.version;
    blogInfo.value.recordNumber = resData.recordNumber;
    blogInfo.value.name = resData.name;
    blogInfo.value.title = resData.title;
    blogInfo.value.desc = resData.desc;
    blogInfo.value.darkBg = resData.darkBg;
    blogInfo.value.lightBg = resData.lightBg;
    blogInfo.value.author = resData.author;
    blogInfo.value.totalVisits = resData.visitedCount;
    blogInfo.value.blogCreateTime = resData.createdTime;
    blogInfo.value.blogUpdateTime = resData.updateTime;
    blogInfo.value.avatar = resData.avatar;
    blogInfo.value.email = resData.email;
    blogInfo.value.github = resData.github;
    blogInfo.value.gitee = resData.gitee;
    blogInfo.value.wechat = resData.wechat;
    blogInfo.value.qq = resData.qq;
    blogInfo.value.telegram = resData.telegram;
    blogInfo.value.bilibili = resData.bilibili;
    blogInfo.value.twitter = resData.twitter;
    blogInfo.value.wechatPay = resData.wechatPay;
    blogInfo.value.alipay = resData.alipay;
    blogInfo.value.proverb = resData.proverb;
    // 默认值
    blogInfo.value.defaultAvatar = resData.defaultAvatar;
    saveBlogInfo()
  } else {
    onMounted(() => {
      ElNotification({
        title: '获取网站信息失败',
        message: '请检查网络连接或联系管理员',
        type:'error',
        duration: 0,
        position: 'top-left'
      })
    })
  }
})
.catch(err => {
  console.log(err);
  onMounted(() => {
    ElNotification({
      title: '获取网站信息失败',
      message: '请检查网络连接或联系管理员',
      type:'error',
      duration: 0,
      position: 'top-left'
    })
  })
});

