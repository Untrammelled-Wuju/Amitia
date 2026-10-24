export type OwnedRuntimeSSEFrame =
  | { type: 'agent_event'; data: string }
  | { type: 'snapshot_required'; data: string };

const maxBufferedCharacters = 1024 * 1024;

export class OwnedRuntimeSSEDecoder {
  private buffer = '';
  private pendingCR = false;

  push(chunk: string): OwnedRuntimeSSEFrame[] {
    if (this.pendingCR) {
      chunk = '\r' + chunk;
      this.pendingCR = false;
    }
    if (chunk.endsWith('\r')) {
      this.pendingCR = true;
      chunk = chunk.slice(0, -1);
    }
    this.buffer += chunk.replace(/\r\n?|\n/g, '\n');
    if (this.buffer.length > maxBufferedCharacters) throw new Error('Runtime SSE 事件超出限制');
    const frames: OwnedRuntimeSSEFrame[] = [];
    let end = this.buffer.indexOf('\n\n');
    while (end !== -1) {
      const raw = this.buffer.slice(0, end);
      this.buffer = this.buffer.slice(end + 2);
      const lines = raw.split('\n');
      let type = '';
      const data: string[] = [];
      for (const line of lines) {
        if (!line || line.startsWith(':')) continue;
        const colon = line.indexOf(':');
        const field = colon < 0 ? line : line.slice(0, colon);
        const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '');
        if (field === 'event') type = value;
        if (field === 'data') data.push(value);
      }
      if (type || data.length) {
        if (type !== 'agent_event' && type !== 'snapshot_required') {
          throw new Error('Runtime SSE 事件类型不符合服务端协议');
        }
        if (data.length === 0) throw new Error('Runtime SSE 事件缺少数据');
        frames.push({ type, data: data.join('\n') });
      }
      end = this.buffer.indexOf('\n\n');
    }
    return frames;
  }

  finish(): OwnedRuntimeSSEFrame[] {
    return this.pendingCR ? this.push('\n') : [];
  }
}
