import 'package:amitia_app/core/artifact/artifact_model.dart';
import 'package:amitia_app/core/services/device_owned_attachments.dart';
import 'package:flutter_test/flutter_test.dart';

ArtifactMetadata artifact({
  String kind = 'file',
  String name = 'payload.bin',
  String mimeType = 'application/octet-stream',
  int size = 1048577,
}) {
  final now = DateTime.utc(2026, 11, 20);
  return ArtifactMetadata(
    id: 'art-a',
    ownerSpaceId: 'space',
    ownerDeviceId: 'device-a',
    storageDeviceId: 'core-a',
    workspaceId: '',
    kind: ArtifactKind.values.firstWhere((item) => item.value == kind),
    blobDigest: 'sha256:${'a' * 64}',
    sizeBytes: size,
    mimeType: mimeType,
    filename: name,
    fileExtension: '.bin',
    status: ArtifactStatus.ready,
    source: 'chat_upload',
    width: 0,
    height: 0,
    durationMs: 0,
    revision: 1,
    createdAt: now,
    updatedAt: now,
  );
}

void main() {
  test('任意类型和大小的附件只保存引用元数据', () {
    final item = ownedAttachmentFromArtifact(
      artifact(name: 'archive.tar.zst', size: 9 * 1024 * 1024),
    );
    expect(item, {
      'artifactId': 'art-a',
      'ownerDeviceId': 'device-a',
      'storageDeviceId': 'core-a',
      'kind': 'file',
      'name': 'archive.tar.zst',
      'mimeType': 'application/octet-stream',
      'sha256': 'a' * 64,
      'sizeBytes': 9 * 1024 * 1024,
    });
    expect(item.containsKey('data'), isFalse);
  });

  test('历史消息附件通过已保存消息生成授权读取地址', () {
    final item = ownedAttachmentFromArtifact(
      artifact(kind: 'image', name: 'photo.png', mimeType: 'image/png'),
    );
    final uri = ownedImageURL(
      [item],
      conversationId: 'chat-a',
      messageId: 'request/user',
      characterId: 'one',
      conversationOwnerId: 'device-a',
    );
    expect(uri, contains('/api/device-mesh/v1/business/attachments/art-a/content?'));
    expect(uri, contains('messageId=request%2Fuser'));
    expect(uri, contains('conversationOwnerId=device-a'));
  });

  test('无效或伪造的附件引用不能生成读取地址', () {
    expect(
      ownedAttachment({
        'artifactId': '../secret',
        'ownerDeviceId': 'device-a',
        'storageDeviceId': 'core-a',
        'kind': 'file',
        'name': 'a',
        'mimeType': 'application/octet-stream',
        'sha256': 'a' * 64,
        'sizeBytes': 1,
      }),
      isNull,
    );
    expect(
      ownedImageURL(
        [
          {
            'artifactId': 'art-a',
            'ownerDeviceId': 'device-a',
            'storageDeviceId': 'core-a',
            'kind': 'image',
            'name': 'a.png',
            'mimeType': 'image/png',
            'sha256': 'bad',
            'sizeBytes': 1,
          },
        ],
        conversationId: 'chat',
        messageId: 'message',
        characterId: 'one',
        conversationOwnerId: 'device-a',
      ),
      isNull,
    );
  });

  test('转写只使用仍匹配原内容的文本', () {
    expect(
      ownedMessageText({
        'content': '[语音]',
        'transcriptionSourceContent': '[语音]',
        'transcription': '喝茶',
      }),
      '喝茶',
    );
    expect(
      ownedMessageText({
        'content': '修改后的文字',
        'transcriptionSourceContent': '[语音]',
        'transcription': '喝茶',
      }),
      '修改后的文字',
    );
  });
}
