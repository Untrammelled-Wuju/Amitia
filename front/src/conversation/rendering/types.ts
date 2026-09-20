export type MessageState =
  | "queued"
  | "streaming"
  | "completed"
  | "interrupted"
  | "failed"
  | "cancelled";

export interface CharacterIdentity {
  id: string;
  name: string;
  avatar?: string;
}

export interface ThinkingBlock {
  content: string;
  state: MessageState;
  duration?: number;
}

export interface CitationSource {
  id: string;
  title: string;
  url?: string;
  fileId?: string;
  snippet?: string;
}

export interface ToolBlock {
  kind: "tool";
  id: string;
  name: string;
  arguments?: unknown;
  result?: unknown;
  duration?: number;
  error?: string;
  status: "queued" | "running" | "success" | "failed" | "cancelled";
}

export interface FileBlock {
  kind: "file";
  id: string;
  name: string;
  mimeType?: string;
  size?: number;
  url?: string;
  status: "loading" | "ready" | "failed";
  error?: string;
}

export interface ImageBlock {
  kind: "image";
  id: string;
  url: string;
  alt?: string;
  mimeType?: string;
  animated?: boolean;
  width?: number;
  height?: number;
  status?: "loading" | "ready" | "failed";
}

export interface AudioBlock {
  kind: "audio";
  id: string;
  url: string;
  title?: string;
  duration?: number;
  status?: "loading" | "ready" | "failed";
}

export interface VideoBlock {
  kind: "video";
  id: string;
  url: string;
  title?: string;
  poster?: string;
  duration?: number;
  status?: "loading" | "ready" | "failed";
}

export interface ArtifactBlock {
  kind: "artifact";
  id: string;
  artifactKind: string;
  title: string;
  mimeType?: string;
  content?: string;
  fileId?: string;
  url?: string;
  size?: number;
}

export interface AgentTaskStep {
  id: string;
  title: string;
  status: "pending" | "running" | "done" | "failed";
  meta?: string;
}

export interface AgentTaskBlock {
  kind: "agent-task";
  id: string;
  title: string;
  status: "pending" | "running" | "done" | "failed";
  steps: AgentTaskStep[];
  progress?: number;
  elapsed?: string;
  error?: string;
}

export interface ExtensionBlock {
  kind: "extension";
  id: string;
  rendererId: string;
  version: number;
  payload: unknown;
}

export interface UnknownRichBlock {
  kind: "unknown";
  id: string;
  type: string;
  payload: unknown;
}

export type RichBlock =
  | ToolBlock
  | FileBlock
  | ImageBlock
  | AudioBlock
  | VideoBlock
  | ArtifactBlock
  | AgentTaskBlock
  | ExtensionBlock
  | UnknownRichBlock;

export interface AIMessageData {
  id: string;
  conversationId?: string;
  role: "assistant" | "user" | "system";
  character?: CharacterIdentity;
  markdown: string;
  thinking?: ThinkingBlock;
  blocks: RichBlock[];
  sources: CitationSource[];
  state: MessageState;
  createdAt: number;
  raw: Record<string, unknown>;
}

export interface AssistantTurnItem {
  id: string;
  turnId: string;
  conversationId: string;
  sequence: number;
  type: "thinking" | "tool_call" | "tool_result" | "text" | string;
  status: string;
  callId?: string;
  toolName?: string;
  content?: string;
  argumentsJson?: string;
  resultJson?: string;
  errorCode?: string;
  durationMs?: number;
  isFinal?: number;
  legacyMessageId?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface AssistantTurnData {
  id: string;
  conversationId: string;
  characterId?: string;
  userMessageId?: string;
  requestId?: string;
  responseGroupId?: string;
  sequence: number;
  status: string;
  createdAt?: string;
  updatedAt?: string;
  completedAt?: string;
  items: AssistantTurnItem[];
}

export interface MarkdownSegment {
  id: string;
  type:
    | "markdown"
    | "code"
    | "diff"
    | "terminal"
    | "mermaid"
    | "latex"
    | "html-preview";
  content: string;
  language?: string;
  filename?: string;
  streaming?: boolean;
}
