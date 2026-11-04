import { describe, expect, it } from 'vitest';
import { parseWorkbenchDocumentChunk } from '../components/workbench/workbench-document-chunks';

describe('workbench raw-byte file preview', () => {
  it('reconstructs a UTF-8 character split between responses', () => {
    const first = parseWorkbenchDocumentChunk({ isText: true, contentBase64: '5Lg=', bytesRead: 2, nextOffset: 2, hasMore: true }, null, 0);
    const second = parseWorkbenchDocumentChunk({ isText: true, contentBase64: 'rWE=', bytesRead: 2, nextOffset: 4, hasMore: false }, first, 2);
    expect(second.text).toBe('中a');
    expect(second.offset).toBe(4);
    expect(second.hasMore).toBe(false);
  });

  it('rejects ambiguous offsets and inconsistent byte counts', () => {
    expect(() => parseWorkbenchDocumentChunk({ isText: true, content: 'x', nextOffset: 6 }, null, 0)).toThrow();
    expect(() => parseWorkbenchDocumentChunk({ isText: true, content: 'x', bytesRead: 2 }, null, 0)).toThrow();
    expect(() => parseWorkbenchDocumentChunk({ isText: true, content: '', hasMore: true }, null, 0)).toThrow();
    const first = parseWorkbenchDocumentChunk({ isText: true, contentBase64: 'eA==', bytesRead: 1, nextOffset: 1, hasMore: false }, null, 0);
    expect(() => parseWorkbenchDocumentChunk({ isText: true, content: 'y' }, first, 0)).toThrow();
  });

  it('rejects content-only responses', () => {
    expect(() => parseWorkbenchDocumentChunk({ isText: true, content: '你好' }, null, 0)).toThrow();
  });
});
