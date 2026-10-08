import 'dart:convert';

import 'package:amitia_app/core/services/owned_quote_reference.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';

import 'owned_realtime_test.dart' show scope;

ChatMessage quoted({
  String conversation = 'actual-owner-conversation',
  int? revision = 7,
  MessageStatus status = MessageStatus.sent,
}) => ChatMessage(
  id: 'original',
  sourceOwnerId: 'phone',
  sourceScope: scope(),
  sourceConversationId: conversation,
  sourceRevision: revision,
  characterId: 'role',
  role: MessageRole.assistant,
  type: MessageType.text,
  content: '实际消息正文',
  time: DateTime.now(),
  status: status,
);

void main() {
  test('引用保留实际所有者会话编号及完整正文哈希', () {
    final reference = ownedQuoteReference(quoted());
    expect(reference['conversationId'], 'actual-owner-conversation');
    expect(reference['expectedRevision'], 7);
    expect(
      reference['contentHash'],
      sha256.convert(utf8.encode('实际消息正文')).toString(),
    );
    expect(reference['expectedExecutionScope'], scope());
  });
  test('历史消息保持版本零且不伪造CAS版本', () {
    expect(ownedQuoteReference(quoted(revision: null))['expectedRevision'], 0);
  });
  test('缺少原会话和未保存回复不能引用', () {
    expect(
      () => ownedQuoteReference(quoted(conversation: '')),
      throwsStateError,
    );
    expect(
      () => ownedQuoteReference(quoted(status: MessageStatus.streaming)),
      throwsStateError,
    );
  });
}
