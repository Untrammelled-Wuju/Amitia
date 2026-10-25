import 'package:amitia_app/core/models/conversation.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('conversation parser accepts boolean reasoning state', () {
    final conversation = ConversationDto.fromJson({
      'id': 'conversation-1',
      'reasoningEnabled': true,
    });

    expect(conversation.id, 'conversation-1');
    expect(conversation.reasoningEnabled, 1);
  });

  test('conversation parser falls back to the latest message sequence', () {
    final conversation = ConversationDto.fromJson({
      'id': 'conversation-1',
      'lastMessageSequence': 2,
      'lastMessagePreview': '最后一条消息',
    });

    expect(conversation.messageCount, 2);
    expect(conversation.lastMessage, '最后一条消息');
  });
}
