import type { WorldBookEntry } from "@/composables/useWorldBook";

export type WorldBookImportRow = {
  valid: boolean;
  item?: Partial<WorldBookEntry>;
  matchType: string;
  matchPattern: string;
  message: string;
};

export const importExampleJson = JSON.stringify(
  [
    {
      "__说明": {
        matchType: "必填。匹配方式：regex=正则匹配，exact=精确文本，keyword=逗号分隔关键词。",
        matchPattern: "必填。匹配内容；正则模式填写正则表达式，精确模式填写完整文本，关键词模式用英文逗号分隔。",
        matchScope: "可选。匹配范围：full_context=全部上下文，user_message=仅用户消息，assistant_reply=仅 AI 回复；默认 full_context。",
        injectContent: "必填。匹配命中后注入提示词的设定内容。",
        priority: "可选。0-10 的整数，数值越高越优先，默认 0。",
        characterId: "可选。关联角色 ID；留空表示当前空间全局规则，填写时必须使用已存在的角色 ID。",
      },
      matchType: "keyword",
      matchPattern: "世界树, 精灵, 上古战争",
      matchScope: "full_context",
      injectContent: "世界树位于大陆中央，是精灵文明和上古战争的起源地。",
      priority: 8,
      characterId: "",
    },
  ],
  null,
  2,
);

export function parseWorldBookImport(text: string): WorldBookImportRow[] {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error("JSON 格式错误，请检查括号、逗号和字符串引号");
  }
  if (!Array.isArray(data)) {
    throw new Error("JSON 顶层必须是数组");
  }
  if (data.length === 0) {
    throw new Error("JSON 数组不能为空");
  }
  return data.map((item, index) => validateWorldBookImportItem(item, index));
}

function validateWorldBookImportItem(raw: unknown, index: number): WorldBookImportRow {
  const rowNumber = index + 1;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return {
      valid: false,
      matchType: "",
      matchPattern: "",
      message: `第 ${rowNumber} 条不是对象`,
    };
  }
  const record = raw as Record<string, unknown>;
  const matchType = typeof record.matchType === "string" ? record.matchType.trim() : "";
  const matchPattern = typeof record.matchPattern === "string" ? record.matchPattern.trim() : "";
  const matchScope = record.matchScope === undefined
    ? "full_context"
    : typeof record.matchScope === "string"
      ? record.matchScope.trim()
      : "";
  const injectContent = typeof record.injectContent === "string" ? record.injectContent.trim() : "";
  const priority = record.priority === undefined ? 0 : record.priority;
  const characterId = record.characterId === undefined ? "" : record.characterId;

  const invalid = (message: string): WorldBookImportRow => ({
    valid: false,
    matchType,
    matchPattern,
    message,
  });

  if (!["regex", "exact", "keyword"].includes(matchType)) {
    return invalid("matchType 必须是 regex、exact 或 keyword");
  }
  if (!matchPattern) return invalid("matchPattern 不能为空");
  if (!["full_context", "user_message", "assistant_reply"].includes(matchScope)) {
    return invalid("matchScope 必须是 full_context、user_message 或 assistant_reply");
  }
  if (!injectContent) return invalid("injectContent 不能为空");
  if (
    typeof priority !== "number"
    || !Number.isInteger(priority)
    || priority < 0
    || priority > 10
  ) {
    return invalid("priority 必须是 0-10 的整数");
  }
  if (typeof characterId !== "string") {
    return invalid("characterId 必须是字符串");
  }
  if (matchType === "regex") {
    try {
      new RegExp(matchPattern);
    } catch {
      return invalid("matchPattern 不是有效的正则表达式");
    }
  }
  return {
    valid: true,
    matchType,
    matchPattern,
    message: "字段有效，可以导入",
    item: {
      matchType,
      matchPattern,
      matchScope,
      injectContent,
      priority,
      characterId,
    },
  };
}
