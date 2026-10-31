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

  test('conversation parser accepts live activity and unread projection', () {
    final conversation = ConversationDto.fromJson({
      'id': 'conversation-1',
      'isGenerating': true,
      'unreadCompletedCount': 3,
      'activityLabel': '正在使用 workspace.read 工具',
      'activeToolName': 'workspace.read',
      'lastMessagePreview': '已经完成的内容',
      'lastMessageType': 'text',
    });

    expect(conversation.isGenerating, isTrue);
    expect(conversation.unreadCompletedCount, 3);
    expect(conversation.activityLabel, '正在使用 workspace.read 工具');
    expect(conversation.activeToolName, 'workspace.read');
    expect(conversation.lastMessage, '已经完成的内容');
  });
}
