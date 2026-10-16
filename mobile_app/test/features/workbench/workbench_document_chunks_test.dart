import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/runtime/workbench_document_chunks.dart';

void main() {
  test('UTF-8 split across chunk boundaries is restored without loss', () {
    final left = WorkbenchDocumentChunk.parse({
      'isText': true,
      'contentBase64': base64Encode([0xe4, 0xb8]),
      'bytesRead': 2,
      'nextOffset': 2,
      'hasMore': true,
    }, 0);
    final part = WorkbenchDocumentContent.apply(null, left, 0);
    final right = WorkbenchDocumentChunk.parse({
      'isText': true,
      'contentBase64': base64Encode([0xad, 0x61]),
      'bytesRead': 2,
      'nextOffset': 4,
      'hasMore': false,
    }, 2);
    final full = WorkbenchDocumentContent.apply(part, right, 2);
    expect(full.text, '中a');
    expect(full.offset, 4);
    expect(full.hasMore, false);
  });

  test('mismatched offsets and lengths cannot corrupt a document', () {
    expect(() => WorkbenchDocumentChunk.parse({'isText': true, 'contentBase64': 'YQ==', 'bytesRead': 9}, 0), throwsFormatException);
    expect(() => WorkbenchDocumentChunk.parse({'isText': true, 'content': 'x', 'nextOffset': 11}, 0), throwsFormatException);
    expect(() => WorkbenchDocumentChunk.parse({'isText': true, 'content': '', 'hasMore': true}, 0), throwsFormatException);
    final base = WorkbenchDocumentContent.apply(null, WorkbenchDocumentChunk.parse({'isText': true, 'content': 'a'}, 0), 0);
    expect(() => WorkbenchDocumentContent.apply(base, WorkbenchDocumentChunk.parse({'isText': true, 'content': 'b'}, 2), 2), throwsFormatException);
  });

  test('legacy text-only response remains compatible', () {
    final chunk = WorkbenchDocumentChunk.parse({'isText': true, 'content': '旧版本'}, 0);
    final result = WorkbenchDocumentContent.apply(null, chunk, 0);
    expect(result.text, '旧版本');
    expect(result.offset, utf8.encode('旧版本').length);
    expect(result.hasMore, false);
  });
}
