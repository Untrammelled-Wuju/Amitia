import 'dart:convert';

import 'package:crypto/crypto.dart';

import '../../shared/models/models.dart';
import 'device_owned_realtime_service.dart';

Map<String, dynamic> ownedQuoteReference(ChatMessage message) {
  final scope = message.sourceScope;
  if (scope == null ||
      message.sourceOwnerId.isEmpty ||
      message.sourceConversationId.isEmpty ||
      message.characterId.isEmpty ||
      message.id.isEmpty ||
      (message.sourceRevision ?? 0) < 0 ||
      !const [MessageRole.user, MessageRole.assistant].contains(message.role) ||
      const [
        MessageStatus.sending,
        MessageStatus.error,
        MessageStatus.streaming,
      ].contains(message.status)) {
    throw StateError('引用消息缺少原归属或尚未保存，请重新加载后选择');
  }
  ownedRealtimeAuthority(scope);
  return {
    'ownerId': message.sourceOwnerId,
    'characterId': message.characterId,
    'conversationId': message.sourceConversationId,
    'messageId': message.id,
    'expectedRevision': message.sourceRevision ?? 0,
    'contentHash': sha256.convert(utf8.encode(message.content)).toString(),
    'expectedExecutionScope': Map<String, dynamic>.from(scope),
  };
}
