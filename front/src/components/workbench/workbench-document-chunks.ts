export interface WorkbenchDocumentContent {
  bytes: Uint8Array;
  text: string;
  offset: number;
  isText: boolean;
  hasMore: boolean;
}

const readLimit = 262144;
const previewLimit = 8 * 1024 * 1024;

export function parseWorkbenchDocumentChunk(payload: unknown, previous: WorkbenchDocumentContent | null, offset: number): WorkbenchDocumentContent {
  if (!payload || typeof payload !== 'object' || !Number.isSafeInteger(offset) || offset < 0 || offset !== (previous?.offset ?? 0)) {
    throw new Error('工作区文件分片起点不匹配');
  }
  const data = payload as Record<string, unknown>;
  if (typeof data.isText !== 'boolean') throw new Error('工作区文件类型无效');
  let bytes: Uint8Array;
  if (typeof data.contentBase64 === 'string') {
    if (data.contentBase64.length > Math.ceil(readLimit / 3) * 4 + 4) throw new Error('工作区文件分片超过限制');
    const binary = atob(data.contentBase64);
    bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  } else if (typeof data.content === 'string') {
    bytes = new TextEncoder().encode(data.content);
  } else {
    throw new Error('工作区文件内容缺失');
  }
  if (bytes.byteLength > readLimit || (data.bytesRead !== undefined && data.bytesRead !== bytes.byteLength) ||
      (data.nextOffset !== undefined && data.nextOffset !== offset + bytes.byteLength)) {
    throw new Error('工作区文件分片长度或游标异常');
  }
  const hasMore = typeof data.hasMore === 'boolean' ? data.hasMore : bytes.byteLength === readLimit;
  if (hasMore && bytes.byteLength === 0) throw new Error('工作区文件空分片不可继续');
  const old = previous?.bytes ?? new Uint8Array(0);
  if (old.byteLength + bytes.byteLength > previewLimit) throw new Error('预览内容已超过 8 MiB 上限');
  const merged = new Uint8Array(old.byteLength + bytes.byteLength);
  merged.set(old);
  merged.set(bytes, old.byteLength);
  const isText = data.isText;
  return {
    bytes: isText ? merged : new Uint8Array(0),
    text: isText ? new TextDecoder('utf-8').decode(merged) : '',
    isText,
    offset: offset + bytes.byteLength,
    hasMore: isText && hasMore && merged.byteLength < previewLimit,
  };
}
