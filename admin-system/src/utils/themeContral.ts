import { ElNotification } from "element-plus";
//检查系统主题
function checkSystemTheme() {
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
}
function SetTheme(){//设置主题
  //判断主题有没有值
  if (localStorage.getItem('theme') == null) {
    //如果没有，就将主题设置为 系统主题
    if(checkSystemTheme() === 'light'){
      document.documentElement.dataset.theme = 'light'
      localStorage.setItem('theme','light')
    }else if(checkSystemTheme() === 'dark'){
      document.documentElement.dataset.theme = 'dark'
      localStorage.setItem('theme','dark')
    }
    //防止出现没有主题的情况
    else{
      document.documentElement.dataset.theme = 'light'
      localStorage.setItem('theme','light')
    }
  } else {
    if (localStorage.getItem('theme') == 'light') {
      document.documentElement.dataset.theme = 'light'
    } else if (localStorage.getItem('theme') == 'dark') {
      document.documentElement.dataset.theme = 'dark'
    }
  }
}
SetTheme()

//主题切换
function ChangeTheme() {
  if (document.documentElement.dataset.theme == 'light') {
    localStorage.setItem('theme', 'dark')
    document.documentElement.dataset.theme = localStorage.getItem('theme') || checkSystemTheme()
    ElNotification({
      type:'success',
      title: '切换成功',
      message: '已切换为暗黑模式',
      position: 'top-left',
    })
  } else if (document.documentElement.dataset.theme == 'dark') {
    localStorage.setItem('theme', 'light')
    document.documentElement.dataset.theme = localStorage.getItem('theme') || checkSystemTheme()
    ElNotification({
      type:'success',
      title: '切换成功',
      message: '已切换为明亮模式',
      position: 'top-left',
    })
  }
}
export default ChangeTheme
