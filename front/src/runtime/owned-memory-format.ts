export interface OwnedMemoryField {
  label: string;
  value: string;
}

const FIELD_LABELS: Record<string, string> = {
  name: "名字",
  likes: "喜欢",
  dislikes: "不喜欢",
  preference: "偏好",
  habit: "习惯",
  plan: "计划",
  relationship: "关系",
  event: "事件",
  details: "详情",
  summary: "摘要",
  topic: "话题",
  user_input: "用户输入",
  ai_reply_summary: "回复要点",
  status: "状态",
  subject: "主语",
  predicate: "谓语",
  object: "宾语",
  relation: "关系",
  relations: "关系",
  facts: "关联事实",
  nodes: "节点",
  edges: "连线",
  links: "连线",
  source: "来源",
  target: "目标",
  from: "起点",
  to: "终点",
  content: "内容",
  text: "文本",
  value: "内容",
  attributeName: "属性",
  attributeValue: "取值",
  fieldName: "字段",
  fieldValue: "取值",
  memoryType: "类型",
  importance: "重要度",
  createdAt: "创建时间",
  updatedAt: "更新时间",
  file: "文件",
  file_name: "文件",
  filename: "文件",
  path: "路径",
  target_file: "目标文件",
  target_workspace: "目标工作区",
  expected_content: "期望内容",
  actual_content: "实际内容",
  created: "已创建",
  resolved_path: "解析路径",
  workspace: "工作区",
  workspace_path_available: "工作区路径可用",
  file_content_available: "文件内容可用",
  note: "说明",
  outcome: "结果",
  result: "结果",
  reason: "原因",
  error: "错误",
  errors: "错误",
  refused: "拒绝行为",
  tools_tried: "尝试的工具",
  extra_probe: "额外探测",
  color: "颜色",
  color_name: "颜色",
  hex: "色值",
  scope: "范围",
  property: "属性",
  statement: "结论",
  values: "向量",
  modelFingerprint: "模型",
};

const TITLE_LABELS: Record<string, string> = {
  profile_name: "名字",
  profile_likes: "喜欢的",
  profile_dislikes: "不喜欢的",
  profile_communication_preference: "沟通偏好",
  profile_preference: "偏好",
  user_name_is_axu: "用户名字",
  user_self_introduction: "用户自我介绍",
  conversation_summary: "对话摘要",
  current_turn_context: "当前对话上下文",
  e2e_proof_file_creation_status: "文件创建状态",
  e2e_proof_file_status: "文件状态",
};

const PRIMARY_TEXT_KEYS = ["value", "text", "summary"];

const WORD_LABELS: Record<string, string> = {
  page: "页面",
  background: "背景",
  image: "图片",
  mode: "模式",
  enabled: "启用",
  name: "名称",
  count: "数量",
  previous: "原",
  opacity: "不透明度",
  revision: "版本",
  version: "版本",
  selector: "选择器",
  computed: "实际",
  value: "值",
  description: "说明",
  action: "操作",
  result: "结果",
  status: "状态",
  custom: "自定义",
  rules: "规则",
  theme: "主题",
  preset: "预设",
  primary: "主",
  color: "颜色",
  border: "边框",
  radius: "圆角",
  root: "根路径",
  includes: "包含",
  include: "包含",
  not: "不",
  source: "来源",
  uri: "地址",
  communication: "沟通",
  preference: "偏好",
  reset: "重置",
  all: "全部",
  default: "默认",
  transparent: "透明",
  sidebar: "侧边栏",
  chat: "聊天",
  mount: "挂载",
  workspace: "工作区",
  file: "文件",
  content: "内容",
  expected: "期望",
  actual: "实际",
  target: "目标",
  event: "事件",
  details: "详情",
  summary: "摘要",
  topic: "话题",
  user: "用户",
  input: "输入",
  reply: "回复",
  self: "自我",
  introduction: "介绍",
  likes: "喜欢",
  dislikes: "不喜欢",
  setting: "设置",
  settings: "设置",
  restore: "恢复",
  change: "变更",
  state: "状态",
  current: "当前",
  style: "样式",
  host: "宿主",
  session: "会话",
  required: "必需",
  caveat: "备注",
  note: "说明",
  proof: "凭据",
  creation: "创建",
  created: "创建",
  attempt: "尝试",
  failed: "失败",
  retry: "重试",
  availability: "可用性",
  capability: "能力",
  window: "窗口",
  ui: "界面",
  applied: "已应用",
  drive: "磁盘",
  read: "读取",
  request: "请求",
};

const META_KEYS = new Set([
  "id",
  "memoryId",
  "memory_id",
  "source",
  "memoryType",
  "importance",
  "createdAt",
  "updatedAt",
  "updated_at",
  "verifiedAt",
  "projectionStatus",
  "conversationId",
  "deleted",
]);

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function translateKey(key: string): string | null {
  const exact = FIELD_LABELS[key] || TITLE_LABELS[key];
  if (exact) return exact;
  const snake = key.replace(/([a-z0-9])([A-Z])/g, "$1_$2");
  const words = snake.split(/[_\-\s]+/).filter(Boolean);
  if (words.length === 0) return null;
  const parts = words.map((word) => WORD_LABELS[word.toLowerCase()]);
  return parts.every(Boolean) ? parts.join("") : null;
}

function stripPrefix(raw: string): string {
  return raw
    .replace(/^(profile_|user_|episodic_|system_|fact_|memory_|graph_)/, "")
    .replace(/^[a-z_]+:/, "");
}

function fieldLabel(key: string): string {
  return translateKey(key) || key.replace(/[_-]+/g, " ");
}

function textValue(value: any): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" || typeof value === "bigint") return String(value);
  if (typeof value === "boolean") return value ? "是" : "否";
  if (Array.isArray(value)) return value.map(textValue).filter(Boolean).join("、");
  if (isPlainObject(value)) {
    return Object.entries(value)
      .map(([key, item]) => ({ label: fieldLabel(key), value: textValue(item) }))
      .filter((item) => item.value !== "")
      .map((item) => `${item.label}：${item.value}`)
      .join("；");
  }
  return String(value);
}

export function ownedMemoryContent(row: any): any {
  const body = row?.body ?? {};
  return body.content ?? body.value;
}

export function ownedMemoryTitle(row: any): string {
  const body = row?.body ?? {};
  const raw = String(body.title || body.key || "").trim();
  if (!raw) return String(body.summary || row?.id || "记忆记录");
  return translateKey(raw) || translateKey(stripPrefix(raw)) || raw;
}

export function ownedMemoryFields(row: any): OwnedMemoryField[] {
  const kind = String(row?.kind || "");
  const content = ownedMemoryContent(row);
  if (row?.kind === "candidate" || row?.body?.type === "owned-memory-candidate") {
    const value = textValue(row?.body?.value);
    return value ? [{ label: "候选内容", value }] : [];
  }
  if (kind === "vector" && isPlainObject(content)) {
    const dim = Array.isArray(content.values) ? content.values.length : 0;
    const model = String(content.modelFingerprint || "待重建").slice(0, 16);
    return [{ label: "向量", value: `${dim} 维 · 模型 ${model}` }];
  }
  if (typeof content === "string") return content.trim() ? [{ label: "内容", value: content }] : [];
  if (!isPlainObject(content)) {
    const value = textValue(content);
    return value ? [{ label: "内容", value }] : [];
  }
  if (kind === "graph" && Array.isArray(content.nodes)) {
    const nodes = content.nodes.length;
    const edges = Array.isArray(content.edges) ? content.edges.length : 0;
    const fields: OwnedMemoryField[] = [{ label: "图谱", value: `${nodes} 个节点 · ${edges} 条关系` }];
    for (const [key, value] of Object.entries(content)) {
      if (["nodes", "edges", "links", "relations"].includes(key)) continue;
      const text = textValue(value);
      if (text) fields.push({ label: fieldLabel(key), value: text });
    }
    return fields;
  }
  if (kind === "fact" && content.subject !== undefined && content.object !== undefined) {
    const sentence = [content.subject, content.predicate, content.object].map(textValue).filter(Boolean).join(" · ");
    return sentence ? [{ label: "事实", value: sentence }] : [];
  }
  const primary = PRIMARY_TEXT_KEYS.find((key) => typeof content[key] === "string" && String(content[key]).trim() !== "");
  const fields: OwnedMemoryField[] = [];
  if (primary) fields.push({ label: "内容", value: String(content[primary]) });
  for (const [key, value] of Object.entries(content)) {
    if (primary && key === primary) continue;
    if (META_KEYS.has(key) && !(primary && ["source", "importance"].includes(key))) continue;
    const text = textValue(value);
    if (text) fields.push({ label: fieldLabel(key), value: text });
  }
  return fields;
}

export function ownedMemoryText(row: any): string {
  const fields = ownedMemoryFields(row);
  if (fields.length === 0) return "";
  if (fields.length === 1) return fields[0].value;
  return fields.map((field) => `${field.label}：${field.value}`).join(" · ");
}

export function ownedMemoryEditableText(row: any): string {
  const content = ownedMemoryContent(row);
  if (content === null || content === undefined) return "";
  if (typeof content === "string") return content;
  const primary = PRIMARY_TEXT_KEYS.find((key) => isPlainObject(content) && typeof content[key] === "string" && String(content[key]).trim() !== "");
  if (primary) return String((content as Record<string, any>)[primary]);
  return JSON.stringify(content, null, 2);
}
