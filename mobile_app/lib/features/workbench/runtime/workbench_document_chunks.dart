import 'dart:convert';
import 'dart:typed_data';

class WorkbenchDocumentChunk {
  static const int chunkLimit = 262144;
  static const int previewLimit = 8 * 1024 * 1024;

  final Uint8List bytes;
  final bool isText;
  final int nextOffset;
  final bool hasMore;

  const WorkbenchDocumentChunk({required this.bytes, required this.isText, required this.nextOffset, required this.hasMore});

  factory WorkbenchDocumentChunk.parse(Map<String, dynamic>? value, int offset) {
    if (value == null || offset < 0 || value['isText'] is! bool) {
      throw const FormatException('工作区文件响应格式无效');
    }
    final raw = value['contentBase64'];
    final Uint8List bytes;
    if (raw is String) {
      if (raw.length > ((chunkLimit + 2) ~/ 3) * 4 + 4) throw const FormatException('工作区文件分片超过限制');
      bytes = base64Decode(raw);
    } else if (value['content'] is String) {
      bytes = Uint8List.fromList(utf8.encode(value['content'] as String));
    } else {
      throw const FormatException('工作区文件内容缺失');
    }
    if (bytes.length > chunkLimit) throw const FormatException('工作区文件分片超过限制');
    final nextOffset = offset + bytes.length;
    if (value['bytesRead'] != null && value['bytesRead'] != bytes.length) {
      throw const FormatException('工作区文件字节数不匹配');
    }
    if (value['nextOffset'] != null && value['nextOffset'] != nextOffset) {
      throw const FormatException('工作区文件游标不匹配');
    }
    final hasMore = value['hasMore'] is bool ? value['hasMore'] as bool : bytes.length == chunkLimit;
    if (bytes.isEmpty && hasMore) throw const FormatException('工作区文件空分片不可继续');
    return WorkbenchDocumentChunk(bytes: bytes, isText: value['isText'] as bool, nextOffset: nextOffset, hasMore: hasMore);
  }
}

class WorkbenchDocumentContent {
  final Uint8List bytes;
  final int offset;
  final bool isText;
  final bool hasMore;

  const WorkbenchDocumentContent({required this.bytes, required this.offset, required this.isText, required this.hasMore});

  String get text => isText ? utf8.decode(bytes, allowMalformed: true) : '';

  static WorkbenchDocumentContent apply(WorkbenchDocumentContent? previous, WorkbenchDocumentChunk next, int requestedOffset) {
    if (requestedOffset != (previous?.offset ?? 0)) throw const FormatException('工作区文件分片起点不匹配');
    final previousBytes = previous?.bytes ?? Uint8List(0);
    if (previousBytes.length + next.bytes.length > WorkbenchDocumentChunk.previewLimit) {
      throw const FormatException('预览内容已超过 8 MiB 上限');
    }
    final all = Uint8List(previousBytes.length + next.bytes.length)
      ..setRange(0, previousBytes.length, previousBytes)
      ..setRange(previousBytes.length, previousBytes.length + next.bytes.length, next.bytes);
    return WorkbenchDocumentContent(
      bytes: next.isText ? all : Uint8List(0),
      offset: next.nextOffset,
      isText: next.isText,
      hasMore: next.isText && next.hasMore && all.length < WorkbenchDocumentChunk.previewLimit,
    );
  }
}
