import '../artifact/artifact_model.dart';

Map<String, dynamic> ownedAttachmentFromArtifact(
  ArtifactMetadata artifact, {
  String? kind,
}) {
  final sha256 = artifact.blobDigest.startsWith('sha256:')
      ? artifact.blobDigest.substring(7)
      : '';
  if (artifact.id.isEmpty ||
      artifact.ownerDeviceId.isEmpty ||
      artifact.storageDeviceId.isEmpty ||
      artifact.filename.isEmpty ||
      artifact.mimeType.isEmpty ||
      !RegExp(r'^[a-f0-9]{64}$').hasMatch(sha256) ||
      artifact.sizeBytes <= 0) {
    throw StateError('附件元数据不完整');
  }
  return {
    'artifactId': artifact.id,
    'ownerDeviceId': artifact.ownerDeviceId,
    'storageDeviceId': artifact.storageDeviceId,
    'kind': kind ?? artifact.kind.value,
    'name': artifact.filename,
    'mimeType': artifact.mimeType,
    'sha256': sha256,
    'sizeBytes': artifact.sizeBytes,
  };
}

Map<String, dynamic>? ownedAttachment(dynamic value) {
  if (value is! Map) return null;
  final row = Map<String, dynamic>.from(value);
  final artifactId = (row['artifactId'] ?? '').toString();
  final ownerDeviceId = (row['ownerDeviceId'] ?? '').toString();
  final storageDeviceId = (row['storageDeviceId'] ?? '').toString();
  final kind = (row['kind'] ?? '').toString();
  final name = (row['name'] ?? '').toString();
  final mimeType = (row['mimeType'] ?? '').toString();
  final sha256 = (row['sha256'] ?? '').toString();
  final sizeBytes = (row['sizeBytes'] as num?)?.toInt() ?? -1;
  if (artifactId.isEmpty ||
      artifactId.length > 256 ||
      RegExp(r'[\x00\r\n/\\]').hasMatch(artifactId) ||
      ownerDeviceId.isEmpty ||
      ownerDeviceId.length > 256 ||
      RegExp(r'[\x00\r\n/\\]').hasMatch(ownerDeviceId) ||
      storageDeviceId.isEmpty ||
      storageDeviceId.length > 256 ||
      RegExp(r'[\x00\r\n/\\]').hasMatch(storageDeviceId) ||
      !const ['image', 'audio', 'video', 'file'].contains(kind) ||
      name.isEmpty ||
      name.length > 256 ||
      RegExp(r'[\x00\r\n]').hasMatch(name) ||
      mimeType.isEmpty ||
      !RegExp(r'^[a-f0-9]{64}$').hasMatch(sha256) ||
      sizeBytes < 0) {
    return null;
  }
  return {
    'artifactId': artifactId,
    'ownerDeviceId': ownerDeviceId,
    'storageDeviceId': storageDeviceId,
    'kind': kind,
    'name': name,
    'mimeType': mimeType,
    'sha256': sha256,
    'sizeBytes': sizeBytes,
  };
}

String? ownedAttachmentContentURI(
  dynamic value, {
  required String conversationId,
  required String messageId,
  required String characterId,
  required String conversationOwnerId,
  String historicalRoleId = '',
}) {
  final attachment = ownedAttachment(value);
  if (attachment == null ||
      conversationId.isEmpty ||
      messageId.isEmpty ||
      characterId.isEmpty ||
      conversationOwnerId.isEmpty) {
    return null;
  }
  final query = <String, String>{
    'conversationId': conversationId,
    'messageId': messageId,
    'characterId': characterId,
    'conversationOwnerId': conversationOwnerId,
    if (historicalRoleId.isNotEmpty) 'historicalRoleId': historicalRoleId,
  };
  final encoded = Uri(queryParameters: query).query;
  return '/api/device-mesh/v1/business/attachments/${Uri.encodeComponent(attachment['artifactId'] as String)}/content?$encoded';
}

Map<String, dynamic>? ownedFileMetadata(
  dynamic attachments, {
  String conversationId = '',
  String messageId = '',
  String characterId = '',
  String conversationOwnerId = '',
  String historicalRoleId = '',
}) {
  if (attachments is! List) return null;
  for (final item in attachments) {
    final attachment = ownedAttachment(item);
    if (attachment == null ||
        !const ['file', 'video'].contains(attachment['kind'])) {
      continue;
    }
    final uri = ownedAttachmentContentURI(
      attachment,
      conversationId: conversationId,
      messageId: messageId,
      characterId: characterId,
      conversationOwnerId: conversationOwnerId,
      historicalRoleId: historicalRoleId,
    );
    if (uri == null) continue;
    return {...attachment, 'uri': uri};
  }
  return null;
}

String? ownedAttachmentURL(
  dynamic attachments, {
  required String kind,
  required String conversationId,
  required String messageId,
  required String characterId,
  required String conversationOwnerId,
  String historicalRoleId = '',
}) {
  if (attachments is! List) return null;
  for (final item in attachments) {
    final attachment = ownedAttachment(item);
    if (attachment == null || attachment['kind'] != kind) continue;
    return ownedAttachmentContentURI(
      attachment,
      conversationId: conversationId,
      messageId: messageId,
      characterId: characterId,
      conversationOwnerId: conversationOwnerId,
      historicalRoleId: historicalRoleId,
    );
  }
  return null;
}

String? ownedImageURL(
  dynamic attachments, {
  String conversationId = '',
  String messageId = '',
  String characterId = '',
  String conversationOwnerId = '',
  String historicalRoleId = '',
}) =>
    ownedAttachmentURL(
      attachments,
      kind: 'image',
      conversationId: conversationId,
      messageId: messageId,
      characterId: characterId,
      conversationOwnerId: conversationOwnerId,
      historicalRoleId: historicalRoleId,
    );

String? ownedAudioURL(
  dynamic attachments, {
  String conversationId = '',
  String messageId = '',
  String characterId = '',
  String conversationOwnerId = '',
  String historicalRoleId = '',
}) =>
    ownedAttachmentURL(
      attachments,
      kind: 'audio',
      conversationId: conversationId,
      messageId: messageId,
      characterId: characterId,
      conversationOwnerId: conversationOwnerId,
      historicalRoleId: historicalRoleId,
    );

String? ownedVideoURL(
  dynamic attachments, {
  String conversationId = '',
  String messageId = '',
  String characterId = '',
  String conversationOwnerId = '',
  String historicalRoleId = '',
}) =>
    ownedAttachmentURL(
      attachments,
      kind: 'video',
      conversationId: conversationId,
      messageId: messageId,
      characterId: characterId,
      conversationOwnerId: conversationOwnerId,
      historicalRoleId: historicalRoleId,
    );

String ownedMessageText(Map row) {
  return row['transcription'] is String &&
          (row['transcription'] as String).isNotEmpty &&
          row['content'] == row['transcriptionSourceContent']
      ? row['transcription'] as String
      : (row['content'] ?? '').toString();
}
