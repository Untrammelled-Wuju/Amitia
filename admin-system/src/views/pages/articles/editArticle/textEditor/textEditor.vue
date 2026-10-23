<script setup lang="ts">
import textEditorHeader from './textEditorHeader/textEditorHeader.vue'
import { ref, watch, nextTick,onMounted, onUnmounted } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import markdownit from 'markdown-it'
import hljs from 'highlight.js'
import mermaid from 'mermaid'
import 'highlight.js/styles/atom-one-dark.css'
import mathjax3 from 'markdown-it-mathjax3'
import container from 'markdown-it-container'
import mark from 'markdown-it-mark'
import { ElMessage } from 'element-plus'
const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  }
})
const emit = defineEmits(['update:modelValue'])
mermaid.initialize({
  startOnLoad: false,
  theme: 'dark',
  securityLevel: 'loose',
  themeCSS: `
    :root {
      --background: transparent;
      --text-color: black;
    }
    ${/* 流程图连线颜色 */ ''}
    .edgePath path {
      stroke: black !important;
    }
    ${/* 增强暗黑模式适配 */ ''}
    .label foreignObject {
      background-color: transparent !important;
    }
  `,
})
const md = markdownit({
  html: true, // html解析支持
  breaks: true, // 换行符支持
  typographer: true, // 智能排版
  linkify: true, // 链接自动识别
  highlight: function (str, lang) {
    // 公共处理逻辑
    const processCode = (code: string, language: string) => {
      const highlighted = language
        ? hljs.highlight(code, { language }).value
        : hljs.highlightAuto(code).value
      const lines = highlighted.split('\n')

      return `
        <figure class="highlight html">
          <div class="highlight-tools">
            <div class="codeLangBox">
              <div class="circleBox">
                <div class="circle"></div>
                <div class="circle"></div>
                <div class="circle"></div>
              </div>
              <div class="code-lang">${language || 'TEXT'}</div>
            </div>
            <i class="fas fa-paste copy-button"></i>
          </div>
          <div class="code-container">
            <div class="gutter-column">
              <table>
                <tbody>
                  ${lines
                    .map(
                      (line, index) => `<tr><td class="gutter"><pre>${index + 1}</pre></td></tr>`,
                    )
                    .slice(0, -1)
                    .join('')}
                </tbody>
              </table>
            </div>
            <div class="code-column">
              <table>
                <tbody>
                  ${lines
                    .map((line) => `<tr><td class="code"><pre>${line || ' '}</pre></td></tr>`)
                    .slice(0, -1)
                    .join('')}
                </tbody>
              </table>
            </div>
          </div>
        </figure>`
    }
    try {
      if (lang && hljs.getLanguage(lang)) {
        return processCode(str, lang)
      }
      // 默认处理也显示行号
      return processCode(str, '')
    } catch (err) {
      console.log(err)
      return ''
    }
  },
})
  .use(mathjax3)
  .use(container, 'warning')
  .use(container, 'info')
  .use(container, 'danger')
  .use(container, 'center')
  .use(container, 'left')
  .use(container, 'right')
  .use(mark, { del: true })
md.renderer.rules.table_open = () => '<table class="markdown-table">'
md.renderer.rules.table_close = () => '</table>'
const originalFence = md.renderer.rules.fence
md.renderer.rules.fence__default = originalFence

// mermaid渲染处理
md.renderer.rules.fence = (tokens, idx, options, env, self) => {
  const token = tokens[idx]
  if (token.info.trim() === 'mermaid') {
    // 增强注释过滤（处理Windows换行符）
    const filteredContent = token.content.replace(/^\s*%%.*(\r?\n)?/gm, '')
    return `<div class="mermaid">${filteredContent}</div>`
  }
  if (md.renderer.rules.fence__default) {
    return md.renderer.rules.fence__default(tokens, idx, options, env, self)
  }
  return self.renderToken(tokens, idx, options)
}


// 编辑器功能
const debounceRenderMermaid = useDebounceFn(() => {
  try {
    const previewContainer = document.querySelector('.writeArticle__body__preview')
    if (!previewContainer) return

    // 封装渲染函数
    const renderMermaidCharts = (el: HTMLElement) => {
      if (el.dataset.processed) return
      // 彻底清除所有注释（包括空注释行）
      const cleanedContent = (el.textContent || '')
        .replace(/^\s*%%.*$/gm, '') // 匹配任意空白开头的注释
        .replace(/\n\s*\n/g, '\n') // 清除空行
      el.innerHTML = cleanedContent
      mermaid
        .run({
          nodes: [el],
          suppressErrors: false, // 显示错误信息
        })
        .catch((err) => {
          console.error('Mermaid render error:', err)
          el.innerHTML = `<div class="mermaid-error">图表渲染失败: ${err.message}</div>`
        })
    }

    previewContainer.querySelectorAll<HTMLElement>('.mermaid').forEach(renderMermaidCharts)
  } catch (err) {
    console.error('Mermaid render error:', err)
  }
}, 300)

// 文本数据绑定
const textData = ref(props.modelValue || localStorage.getItem('temporaryArticle') || '')
watch(
  () => props.modelValue,
  (newValue) => {
    // 仅在值不同时更新，避免循环更新
    if (newValue !== textData.value) {
      textData.value = newValue
    }
  },
  { immediate: true } // 立即执行一次，确保初始值同步
)
const history = ref<string[]>([]) // 历史记录栈
const historyIndex = ref(-1) // 当前历史记录索引

// 合并声明和初始化
const saveHistory = useDebounceFn(() => {
  history.value = history.value.slice(0, historyIndex.value + 1)
  history.value.push(textData.value)
  historyIndex.value = history.value.length - 1
}, 300)

// 初始化时保存初始状态（移动到函数定义之后）
if(localStorage.getItem('temporaryArticle')) {
  textData.value = localStorage.getItem('temporaryArticle')!
  saveHistory() // 现在可以安全调用
}

watch(textData, () => {
  saveHistory()
  autoSave()
  emit('update:modelValue', textData.value)
  nextTick(() => {
    debounceRenderMermaid()
  })
})
const autoSave = () => {
  localStorage.setItem('temporaryArticle', textData.value)
}
// 添加键盘事件监听
const handleKeyDown = (e: KeyboardEvent) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
    e.preventDefault()
    undo()
  } else if ((e.ctrlKey || e.metaKey) && e.key === 'Z') {
    e.preventDefault()
    redo()
  }
}

// 撤销操作
const undo = () => {
  if (historyIndex.value > 0) {
    historyIndex.value--
    textData.value = history.value[historyIndex.value]
  }
}

// 重做操作
const redo = () => {
  if (historyIndex.value < history.value.length - 1) {
    historyIndex.value++
    textData.value = history.value[historyIndex.value]
  }
}

// 在textarea元素上添加键盘事件监听
onMounted(() => {
  const textarea = writeContentRef.value
  if (textarea) {
    textarea.addEventListener('keydown', handleKeyDown)
  }
})
const clearInput = () => {
  textData.value = ''
  ElMessage.success('文章已清空')
  localStorage.removeItem('temporaryArticle')
}
// 处理头部菜单栏点击事件
const textEditorHeaderRef = ref<InstanceType<typeof textEditorHeader>>()
const isFullScreen = ref<boolean | false>(false)
// 计算光标起始位置和结束位置
const writeContentRef = ref<HTMLTextAreaElement>()
const handleCursorPosition = () => {
  return writeContentRef.value ? {
    start: writeContentRef.value.selectionStart,
    end: writeContentRef.value.selectionEnd
  } : { start: 0, end: 0 }
}
const handleBold = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  // 获取存储的光标位置
  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 构造新文本
  const newText = selectedText
    ? `**${selectedText}**`
    : '****'

  // 更新文本内容
  textData.value =
    textData.value.slice(0, start) +
    newText +
    textData.value.slice(end)

  // 调整光标位置
  nextTick(() => {
    const newPos = selectedText ? end + 2 : start + 2
    textarea.setSelectionRange(newPos, newPos)
    textarea.focus()
  })
}
const handleIncline = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  // 获取存储的光标位置
  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 构造新文本（使用++作为下划线标记）
  const newText = selectedText
    ? `*${selectedText}*`
    : '**'

  // 更新文本内容
  textData.value =
    textData.value.slice(0, start) +
    newText +
    textData.value.slice(end)

  // 调整光标位置
  nextTick(() => {
    const offset = selectedText ? 3 : 4 // 根据是否选中文本计算偏移量
    const newPos = selectedText ? end + offset : start + offset
    textarea.setSelectionRange(newPos - (selectedText ? 0 : 1), newPos)
    textarea.focus()
  })
}
const handleCrossline = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  // 获取存储的光标位置
  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 构造新文本（使用Markdown标准删除线语法）
  const newText = selectedText
    ? `~~${selectedText}~~`
    : '~~~~'

  // 更新文本内容
  textData.value =
    textData.value.slice(0, start) +
    newText +
    textData.value.slice(end)

  // 调整光标位置
  nextTick(() => {
    const offset = selectedText ? 2 : 2 // 前后各添加两个波浪号
    const newPos = selectedText ? end + offset : start + offset
    textarea.setSelectionRange(newPos, newPos)
    textarea.focus()
  })
}
// 处理标题
const handleTitle = (index: number) => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 确保标题级别有效性
  const validIndex = Math.max(1, Math.min(6, index))
  const headerMark = '#'.repeat(validIndex) + ' '  // 确保包含空格
  const placeholder = '请输入文本'

  const newText = selectedText
    ? `${headerMark}${selectedText}\n`
    : `${headerMark}${placeholder}\n`

  textData.value =
    textData.value.slice(0, start) +
    newText +
    textData.value.slice(end)

  nextTick(() => {
    if (!selectedText) {
      const selectionStart = start + headerMark.length
      const selectionEnd = selectionStart + placeholder.length
      textarea.setSelectionRange(selectionStart, selectionEnd)
    } else {
      // 计算新的选区范围（包含新增的标题标记）
      const newStart = start + headerMark.length
      const newEnd = newStart + selectedText.length
      textarea.setSelectionRange(newStart, newEnd)
    }
    textarea.focus()
  })
}
const handleUnderline = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 使用HTML标签实现下划线（已配置html解析支持）
  const newText = selectedText
    ? `<u>${selectedText}</u>`
    : '<u></u>'

  textData.value =
    textData.value.slice(0, start) +
    newText +
    textData.value.slice(end)

  nextTick(() => {
    const offset = selectedText ? 3 : 4
    const newPos = selectedText ? end + offset : start + offset
    textarea.setSelectionRange(newPos - (selectedText ? 0 : 1), newPos)
    textarea.focus()
  })
}

// 通用段落对齐处理函数
const createAlignmentHandler = (direction: string) => () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  let selectedText = textData.value.slice(start, end)

  // 匹配现有的对齐语法
  const alignmentRegex = /^::: (left|center|right)\n([\s\S]*?)\n:::$/gm
  const match = alignmentRegex.exec(selectedText)

  // 如果已存在对齐语法，提取内容部分
  if (match) {
    selectedText = match[2]
    // 调整选区范围（去掉外层语法）
    const fullMatchLength = match[0].length
    const contentLength = selectedText.length
    const offset = (fullMatchLength - contentLength) / 2
    textarea.setSelectionRange(start + offset, end - offset)
  }

  // 构造新的对齐语法
  const wrapper = `::: ${direction}\n${selectedText || '请输入内容'}\n:::\n`

  textData.value =
    textData.value.slice(0, start) +
    wrapper +
    textData.value.slice(end)

  nextTick(() => {
    if (selectedText) {
      // 有选中文本时全选语法块
      const newStart = start
      const newEnd = newStart + wrapper.length
      textarea.setSelectionRange(newStart, newEnd)
    } else {
      // 无选中文本时只选中占位符
      const placeholderStart = start + direction.length + 5  // ::: direction\n 的长度
      const placeholderEnd = placeholderStart + 5           // "请输入内容" 的长度
      textarea.setSelectionRange(placeholderStart, placeholderEnd)
    }
    textarea.focus()
  })
}

const handleLeft = createAlignmentHandler('left')
const handleCenter = createAlignmentHandler('center')
const handleRight = createAlignmentHandler('right')

// 处理列表
const orderedList = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const currentValue = textData.value
  const hasSelection = start !== end

  // 获取当前行信息
  const lineStart = currentValue.lastIndexOf('\n', start - 1) + 1
  const lineEnd = currentValue.indexOf('\n', end)
  const currentLine = currentValue.slice(lineStart, lineEnd === -1 ? undefined : lineEnd)

  // 判断是否已经是列表项
  const isListLine = /^\s*\d+\.\s/.test(currentLine)

  if (hasSelection) {
    // 处理多行选区
    const selectedLines = currentValue.slice(start, end).split('\n')
    const newLines = selectedLines.map((line, index) => {
      if (/^\s*\d+\.\s/.test(line)) {
        return line.replace(/^\s*\d+\.\s*/, '') // 移除已有列表标记
      }
      return `${index + 1}. ${line.trimStart()}` // 添加递增序号
    })

    textData.value =
      currentValue.slice(0, start) +
      newLines.join('\n') +
      currentValue.slice(end)

    nextTick(() => {
      textarea.setSelectionRange(start, start + newLines.join('\n').length)
      textarea.focus()
    })
  } else {
    // 处理单行
    const newLine = isListLine
      ? currentLine.replace(/^\s*\d+\.\s*/, '')
      : `1. ${currentLine.trimStart()}`

    textData.value =
      currentValue.slice(0, lineStart) +
      newLine +
      currentValue.slice(lineStart + currentLine.length)

    nextTick(() => {
      const newPos = lineStart + newLine.length
      textarea.setSelectionRange(newPos, newPos)
      textarea.focus()
    })
  }
}
const unorderedList = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const currentValue = textData.value
  const hasSelection = start !== end

  // 获取当前行信息（与有序列表相同逻辑）
  const lineStart = currentValue.lastIndexOf('\n', start - 1) + 1
  const lineEnd = currentValue.indexOf('\n', end)
  const currentLine = currentValue.slice(lineStart, lineEnd === -1 ? undefined : lineEnd)

  // 匹配无序列表语法（修改正则表达式）
  const isListLine = /^\s*[-+*]\s/.test(currentLine)

  if (hasSelection) {
    // 处理多行选区（修改标记符号）
    const selectedLines = currentValue.slice(start, end).split('\n')
    const newLines = selectedLines.map(line => {
      if (/^\s*[-+*]\s/.test(line)) {
        return line.replace(/^\s*[-+*]\s*/, '') // 移除已有标记
      }
      return `- ${line.trimStart()}` // 统一使用短横线
    })

    textData.value =
      currentValue.slice(0, start) +
      newLines.join('\n') +
      currentValue.slice(end)

    nextTick(() => {
      textarea.setSelectionRange(start, start + newLines.join('\n').length)
      textarea.focus()
    })
  } else {
    // 处理单行（修改标记符号）
    const newLine = isListLine
      ? currentLine.replace(/^\s*[-+*]\s*/, '')
      : `- ${currentLine.trimStart()}`

    textData.value =
      currentValue.slice(0, lineStart) +
      newLine +
      currentValue.slice(lineStart + currentLine.length)

    nextTick(() => {
      const newPos = lineStart + newLine.length
      textarea.setSelectionRange(newPos, newPos)
      textarea.focus()
    })
  }
}
// 处理链接
const handleLink = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 构造链接模板
  const linkTemplate = selectedText
    ? `[${selectedText}](url "title")`
    : '[链接文本](url "标题")'

  // 更新文本内容
  textData.value =
    textData.value.slice(0, start) +
    linkTemplate +
    textData.value.slice(end)

  nextTick(() => {
    // 计算光标定位
    let newPos = start + linkTemplate.indexOf('url') + 4 // 定位到url位置
    if (!selectedText) {
      newPos = start + 1 // 无选中文本时定位到链接文本处
    }

    // 设置选区范围
    const selectionStart = selectedText ? newPos - 3 : start + 1
    const selectionEnd = selectedText ? newPos - 3 + 3 : start + 5
    textarea.setSelectionRange(selectionStart, selectionEnd)
    textarea.focus()
  })
}
// 处理图片
const handleImg = (desc: string,url: string) => {
  const textarea = writeContentRef.value
  console.log(desc,url)
  if (!textarea) return

  const { start, end } = handleCursorPosition()

  // 构造图片Markdown格式
  const altText = desc || '图片描述'
  const imgTemplate = `![${altText}](${url || '图片链接'} "${altText}")`

  // 更新文本内容
  textData.value =
    textData.value.slice(0, start) +
    imgTemplate +
    textData.value.slice(end)

  nextTick(() => {
    // 根据是否有URL设置光标位置
    if (!url) {
      // 无URL时选中"图片链接"文本
      const linkStart = start + altText.length + 4 // ![alt]() 结构的长度
      const linkEnd = linkStart + 4 // "图片链接"的长度
      textarea.setSelectionRange(linkStart, linkEnd)
    } else {
      // 有URL时将光标移至末尾
      textarea.setSelectionRange(start + imgTemplate.length, start + imgTemplate.length)
    }
    textarea.focus()
  })
}
// 处理代码块
const handleCode = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 构造代码块模板（带语言占位符）
  const codeTemplate = selectedText
    ? `\`\`\`language\n${selectedText}\n\`\`\``
    : `\`\`\`language\n \n\`\`\``

  textData.value =
    textData.value.slice(0, start) +
    codeTemplate +
    textData.value.slice(end)

  nextTick(() => {
    // 定位到语言占位符位置
    const langPos = start + 3 // 跳过三个反引号
    const selectionStart = langPos
    const selectionEnd = langPos + 8 // "language"长度

    // 选中占位符方便替换
    textarea.setSelectionRange(selectionStart, selectionEnd)
    textarea.focus()
  })
}
// 处理表格
const handleTable = () => {
  const textarea = writeContentRef.value
  if (!textarea) return

  const { start, end } = handleCursorPosition()
  const selectedText = textData.value.slice(start, end)

  // 构造带占位符的表格模板
  const tableTemplate =
    `| 列1     | 列2     |
|---------|---------|
| 内容1   | 内容2   |`

  textData.value =
    textData.value.slice(0, start) +
    (selectedText ? `${selectedText}\n${tableTemplate}` : tableTemplate) +
    textData.value.slice(end)

  nextTick(() => {
    // 定位到第一个表头占位符
    const columnPos = start + 2 // 跳过首行的 | 符号
    textarea.setSelectionRange(columnPos, columnPos + 2) // 选中"列1"
    textarea.focus()
  })
}

// 处理编辑区滚动
// 添加同步滚动相关变量
const syncScroll = ref(true); // 是否同步滚动
const lastScrollTime = ref(0); // 上次滚动时间戳，用于区分用户滚动和程序滚动
// 处理输入框滚动事件
const handleTextareaScroll = () => {
  if (!syncScroll.value) return;

  const textarea = writeContentRef.value;
  const preview = document.querySelector('.writeArticle__body__preview');

  if (!textarea || !preview) return;

  // 计算滚动比例
  const textareaHeight = textarea.scrollHeight - textarea.clientHeight;
  const previewHeight = preview.scrollHeight - preview.clientHeight;

  if (textareaHeight > 0 && previewHeight > 0) {
    const scrollRatio = textarea.scrollTop / textareaHeight;
    const previewScrollTop = scrollRatio * previewHeight;

    // 记录当前时间戳，表示这是程序触发的滚动
    lastScrollTime.value = Date.now();
    preview.scrollTop = previewScrollTop;
  }
};
// 处理预览区域滚动事件
const handlePreviewScroll = () => {
  // 忽略程序触发的滚动（100ms内）
  if (Date.now() - lastScrollTime.value < 100) return;
  // 用户手动滚动预览区域，暂时禁用同步
  syncScroll.value = false;
  // 3秒后重新启用同步
  setTimeout(() => {
    syncScroll.value = true;
  }, 3000);
};
onMounted(() => {
  const textarea = writeContentRef.value
  if (textarea) {
    textarea.addEventListener('keydown', handleKeyDown)
    // 添加输入框滚动事件监听
    textarea.addEventListener('scroll', handleTextareaScroll)
  }
  // 添加预览区域滚动事件监听
  const preview = document.querySelector('.writeArticle__body__preview')
  if (preview) {
    preview.addEventListener('scroll', handlePreviewScroll)
  }
})
// 添加组件卸载时的事件清理
onUnmounted(() => {
  const textarea = writeContentRef.value
  if (textarea) {
    textarea.removeEventListener('keydown', handleKeyDown)
    textarea.removeEventListener('scroll', handleTextareaScroll)
  }

  const preview = document.querySelector('.writeArticle__body__preview')
  if (preview) {
    preview.removeEventListener('scroll', handlePreviewScroll)
  }
})
</script>

<template>
  <div class="textEditor" :class="{ fullscreen: isFullScreen }">
    <div class="writeArticle__header__container">
      <textEditorHeader
        ref="textEditorHeaderRef"
        @save="autoSave();ElMessage.success('文章已保存到浏览器')"
        @clearInput="clearInput"
        @prevStep="undo"
        @nextStep="redo"
        @bold="handleBold"
        @incline="handleIncline"
        @crossline="handleCrossline"
        @title="handleTitle"
        @underline="handleUnderline"
        @left="handleLeft"
        @center="handleCenter"
        @right="handleRight"
        @orderedList="orderedList"
        @unorderedList="unorderedList"
        @link="handleLink"
        @img="handleImg"
        @code="handleCode"
        @table="handleTable"
        @fullScreen="isFullScreen = $event"
        @downloadArticle="textEditorHeaderRef!.downloadArticle(textData)"
      />
    </div>
    <div class="writeArticle__body">
      <div class="writeArticle__body__writeContent">
        <textarea
          name="content"
          id="writeContent"
          v-model="textData"
          ref="writeContentRef"
          @click="handleCursorPosition"
          @keyup="handleCursorPosition"
          @input="handleCursorPosition"
          placeholder="开始编辑..."
        ></textarea>
      </div>
      <div class="writeArticle__body__preview" v-html="md.render(textData)"></div>
    </div>
  </div>
</template>

<style scoped>
.textEditor {
  --textEditor_borderColor: var(--Main_BoxBorderColor);
  --textEditor_background: var(--Main_Bg_BoxColor);
  --textEditor_HeaderIconColor: var(--Main_font_SecondColor);
  --textEditor_writeArticle__body__preview_background: var(--Main_Bg_BoxColor);
  --textEditor_borderRadius: 8px;
  --textEditor_scrollbarThumbColor: #ccc;
  --textEditor_markdown_mark_background: #ffcc00;
  --textEditor_markdown_divider_background: #ddd;
  --textEditor_markdown_text_firstColor: black;
  --textEditor_markdown_borderColor: #ccc;
  --textEditor_markdown_linkText_color: #A591E0;
  --writeArticle__body__writeContentFontColor: var(--Main_font_SecondColor);
  --writeArticle__body__previewFontColor: var(--Main_font_SecondColor);
  --textEditor_markdown_text_black: rgba(38, 38,38, 1);
  width: 100%;
  border-radius: var(--textEditor_borderRadius);
  border: 2px solid var(--textEditor_borderColor);
  margin-top: 6px;
  &.fullscreen {
    position: fixed;
    top: 0;
    left: 0;
    height: 100vh;
    width: 100vw;
    z-index: 2000;
    background: var(--textEditor_background);
    margin: 0;
    border: none;
    border-radius: 0;
    .writeArticle__body {
      height: calc(100vh - 50px) !important;
    }
  }
  .writeArticle__header__container {
    height: 42px;
    width: 100%;
    border-bottom: 1px solid var(--textEditor_borderColor);
  }
  .writeArticle__body {
    display: grid;
    grid-template-columns: 1fr 1fr;
    height: calc(850px - 50px);
    width: 100%;
    .writeArticle__body__writeContent {
      height: 100%;
      overflow: hidden;
      display: flex;
      border-right: 1px solid var(--textEditor_borderColor);
      #writeContent {
        flex: 1;
        min-height: 100%;
        height: 100%;
        width: 100%;
        resize: none;
        box-sizing: border-box;
        padding: 10px;
        border: none;
        outline: none;
        font-size: 14px;
        padding-bottom: 50vh;
        color: var(--writeArticle__body__writeContentFontColor);
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
    .writeArticle__body__preview {
      height: 100%;
      overflow-y: auto;
      padding: 10px;
      padding-bottom: 50vh;
      color: var(--writeArticle__body__previewFontColor);
      &::-webkit-scrollbar {
        background-color: transparent;
        width: 6px;
      }
      &::-webkit-scrollbar-thumb {
        background-color: var(--textEditor_scrollbarThumbColor);
        border-radius: 10px;
      }
    }
    #writeContent,
    .writeArticle__body__preview {
      background-color: var(--textEditor_writeArticle__body__preview_background);
      &::-webkit-scrollbar {
        background-color: transparent;
        width: 6px;
      }
      &::-webkit-scrollbar-thumb {
        background-color: var(--textEditor_scrollbarThumbColor);
        border-radius: 10px;
      }
    }
  }
}

:deep(p) {
  word-break: break-word;
  margin-bottom: 16px;
  font-size: 16px;
  &:first-child {
    margin-top: 0;
  }
  strong {
    font-weight: 600;
    letter-spacing: 0.02em;
  }
  mark {
    background-color: var(--textEditor_markdown_mark_background);
  }
  u{
    text-decoration: underline;
  }
}
:deep(a) {
  color: var(--textEditor_markdown_linkText_color);
  text-decoration: underline;
}
:deep(ul),
:deep(ol) {
  padding-left: 2em;
  margin: 8px 0;
  color: black;
  color: var(--writeArticle__body__previewFontColor);
  li {
    line-height: 1.8;
    margin: 4px 0;
    &::marker {
      color: var(--writeArticle__body__previewFontColor);
    }
  }
}
:deep(hr) {
  border: 0;
  border-top: 1px solid var(--textEditor_markdown_divider_background);
  margin: 1.5em 0;
}
:deep(ul) {
  list-style-type: disc;
  margin: 16px 0;
  li {
    margin: 4px 0;
    &::marker {
      font-weight: 600;
      font-size: 1.05em;
    }
  }
   ul {
    list-style-type: circle;
    ul {
      list-style-type: square;
      ul {
        list-style-type: "▸    ";
      }
    }
  }
}
:deep(ol) {
  list-style-type: decimal;
  margin: 16px 0;
  li {
    margin: 4px 0;
    &::marker {
      font-weight: 600;
      font-size: 1.05em;
    }
  }
  ol {
    list-style-type: lower-alpha;
    ol {
      list-style-type: lower-roman;
      ol {
        list-style-type: lower-greek;
        ol {
          list-style-type: upper-roman;
        }
      }
    }
  }
}
:deep(.markdown-table) {
  /* 表格支持 */
  width: 100%;
  border-collapse: collapse;
  margin: 15px 0;
  th,
  td {
    border: 1px solid var(--textEditor_borderColor);
    padding: 8px;
    text-align: center;
  }
  th {
    background-color: #f5f5f5;
    color: rgb(88, 88, 88);
  }
}
:deep(del),
:deep(s) {
  text-decoration: line-through;
  color: #666;
  @media (prefers-color-scheme: dark) {
    color: currentColor;
    opacity: 0.7;
  }
}
:deep(img) {
  max-width: 100%;
  height: auto;
  display: block;
  margin: 10px 0;
  border-radius: 4px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}
/* 视频容器样式 */
:deep(video) {
  width: 100%;
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}
:deep(pre) {
  all: unset;
  background: transparent !important;
  padding: 0 !important;
  margin: 0 !important;
}
:deep(code) {
  background: transparent !important;
  padding: 0 !important;
}
:deep(figure.highlight) {
  --padding: 10px;
  background: rgb(40, 44, 52);
  border-radius: 8px;
  overflow: hidden;
  margin: 10px 0;
  .highlight-tools {
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: white;
    background-color: #1c1c1c;
    padding: 0 var(--padding);
    height: 36px;
    .codeLangBox {
      display: flex;
      align-items: center;
      .circleBox {
        --wh: 14px;
        height: var(--wh);
        display: flex;
        align-items: center;
        margin-right: 16px;
        .circle {
          width: var(--wh);
          height: var(--wh);
          border-radius: 50%;
          &:first-child {
            background-color: #fc625d;
          }
          &:nth-child(2) {
            background-color: rgb(253, 188, 64);
            margin: 0 6px;
          }
          &:last-child {
            background-color: rgb(53, 205, 75);
          }
        }
      }
      .code-lang {
        font-size: 16px;
      }
    }
    .copy-button {
      cursor: pointer;
    }
  }
  .code-container {
    display: flex;
    .gutter-column {
      color: #ccc;
      padding: 0 var(--padding);
      border-right: 1px solid var(--textEditor_markdown_borderColor);
      user-select: none;
      font-size: 16px;
    }
    .code-column {
      color: #ddd;
      padding: 0 var(--padding);
      overflow-x: auto;
      font-size: 16px;
      table {
        tbody {
          tr {
            td {
              pre {
                white-space: pre;
              }
            }
          }
        }
      }
      &::-webkit-scrollbar {
        background: transparent;
        height: 8px;
        cursor: pointer;
      }
      &::-webkit-scrollbar-thumb {
        background-color: rgb(88, 88, 88);
        border-radius: 10px;
        cursor: pointer;
      }
      &::-webkit-scrollbar-thumb:hover {
        background-color: rgb(128, 128, 128);
      }
    }
  }
}

/* mermaid */
:deep(.mermaid) {
  background: transparent;
  border-radius: 8px;
  margin: 15px 0;
  border: 1px solid var(--textEditor_markdown_borderColor);
  display: flex;
  align-items: center;
  justify-content: center;
  .label text {
    fill: white !important;
  }
  .actor-line {
    stroke: var(--textEditor_markdown_text_firstColor) !important;
  }
  .task-0,
  .task-1 {
    stroke: red !important;
  }

  /* 增强深色模式适配 */
  @media (prefers-color-scheme: dark) {
    rect[fill='#ffffff'],
    circle[fill='#ffffff'] {
      fill: #2d2d2d !important;
    }
    .pieTitle,
    .gantt-title {
      fill: #fff !important;
    }
  }
}

/* 数学公式样式 */
.MathJax {
  font-size: 1.1em !important;
  color: #333;
}
mjx-container {
  overflow-x: auto;
  overflow-y: hidden;
  margin: 5px 0;
}
@media (prefers-color-scheme: dark) {
  .MathJax {
    color: #eee;
  }
}

/* 提示框样式 */
:deep(*) {
  &.warning,
  &.info,
  &.danger {
    min-height: 68px;
    display: flex;
    align-items: center;
    border-radius: 3px;
    font-size: 16px;
    padding: 10px 0;
    color: rgb(88, 88, 88);
    p {
      padding-left: 10px;
      margin: 0 !important;
    }
  }
  &.warning {
    background: #fffbe6;
    border-left: 7px solid #faad14;
    margin: 10px 0;
    .alert-icon {
      color: #faad14;
    }
    li{
      color: var(--textEditor_markdown_text_black);
    }
    p{
      color: var(--textEditor_markdown_text_black);
    }
  }
  &.info {
    background: #e6f7ff;
    border-left: 7px solid #1890ff;
    .alert-icon {
      color: #1890ff;
    }
    li{
      color: var(--textEditor_markdown_text_black);
    }
    p{
      color: var(--textEditor_markdown_text_black);
    }
  }
  &.danger {
    background: #fff1f0;
    border-left: 7px solid #ff4d4f;
    .alert-icon {
      color: #ff4d4f;
    }
    li{
      color: var(--textEditor_markdown_text_black);
    }
    p{
      color: var(--textEditor_markdown_text_black);
    }
  }
  :deep(.center) {
    display: flex;
    justify-content: center;
    margin: 10px 0;
    p {
      margin: 0;
    }
  }

  :deep(.left) {
    margin: 10px 0;
    p {
      margin-left: 0;
    }
  }

  :deep(.right) {
    display: flex;
    justify-content: end;
    margin: 10px 0;
    p {
      margin: 0;
    }
  }
}
:deep(:where(h1, h2, h3, h4, h5, h6)) {
  position: relative;
  color: var(--writeArticle__body__previewFontColor);
  font-weight: 700;
  margin: 20px 0px 14px;
}
:deep(h1){
  font-size: 2em;
}
:deep(h2) {
  font-size: 1.8em;
}
:deep(h3) {
  font-size: 1.6em;
}
:deep(h4) {
  font-size: 1.4em;
}
:deep(h5) {
  font-size: 1.2em;
}
:deep(h6) {
  font-size: 1em;
}
:deep(blockquote){
  background-color: rgba(73,177,245,0.1);
  padding: 10px;
  border-left: 3.5px solid #49b1f5;
}
</style>
