// 解决导入单vue组件时报错提示未找到xxx模块
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module '@/components/PageTemplate/PageTemplate.vue' {
  import { DefineComponent } from 'vue'
  const PageTemplate: DefineComponent<object, object, unknown>
  export default PageTemplate
}

declare module 'nprogress' {
  interface NProgress {
    configure(options: unknown): void
    start(): void
    done(): void
    // 可根据实际使用情况添加更多方法声明
  }
  const nprogress: NProgress
  export default nprogress
}

declare module 'markdown-it-mark' {
  import MarkdownIt from 'markdown-it';
  const markdownItMark: MarkdownIt.PluginSimple;
  export = markdownItMark;
}
declare module 'markdown-it-underline' {
  interface UnderlineOptions {
    className?: string
  }
  function underline(md: MarkdownIt, options?: UnderlineOptions): void
  export = underline
}
