declare module "markdown-it-task-lists" {
  import type MarkdownIt from "markdown-it";
  const taskLists: MarkdownIt.PluginWithOptions<{
    enabled?: boolean;
    label?: boolean;
    labelAfter?: boolean;
  }>;
  export default taskLists;
}

declare module "markdown-it-footnote" {
  import type MarkdownIt from "markdown-it";
  const footnote: MarkdownIt.PluginSimple;
  export default footnote;
}
