import 'package:amitia_app/features/conversation/rendering/assistant_identity.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

ChatMessage _message({
  required String id,
  String characterId = 'character-1',
  String responseGroupId = 'request-1',
  MessageRole role = MessageRole.assistant,
}) {
  return ChatMessage(
    id: id,
    characterId: characterId,
    role: role,
    type: MessageType.text,
    content: id,
    time: DateTime(2026, 9, 20),
    responseGroupId: responseGroupId,
  );
}

void main() {
  test('同一 AI 连续消息只在第一条显示身份', () {
    final previous = _message(id: 'assistant-1');
    final current = _message(id: 'assistant-2');

    expect(shouldShowAssistantIdentity(current, previous), isFalse);
    expect(shouldCompactAfterAssistantMessage(previous, current), isTrue);
  });

  test('不同 AI 连续发言时重新显示身份', () {
    final previous = _message(id: 'assistant-1', characterId: 'character-1');
    final current = _message(id: 'assistant-2', characterId: 'character-2');

    expect(shouldShowAssistantIdentity(current, previous), isTrue);
    expect(shouldCompactAfterAssistantMessage(previous, current), isFalse);
  });

  test('同一 AI 连续回复按发送顺序合并为一个连续段', () {
    final previous = _message(id: 'assistant-1', responseGroupId: 'request-1');
    final current = _message(id: 'assistant-2', responseGroupId: 'request-2');

    expect(shouldShowAssistantIdentity(current, previous), isFalse);
    expect(shouldCompactAfterAssistantMessage(previous, current), isTrue);
  });

  test('用户消息重新开始发送者连续段', () {
    final previous = _message(
      id: 'user-1',
      characterId: '',
      responseGroupId: '',
      role: MessageRole.user,
    );
    final current = _message(id: 'assistant-1');

    expect(shouldShowAssistantIdentity(current, previous), isTrue);
  });

  test('发送者缺失时使用回复分组判断连续性', () {
    final previous = _message(
      id: 'assistant-1',
      characterId: '',
      responseGroupId: 'request-1',
    );
    final continuation = _message(
      id: 'assistant-2',
      characterId: '',
      responseGroupId: 'request-1',
    );
    final nextReply = _message(
      id: 'assistant-3',
      characterId: '',
      responseGroupId: 'request-2',
    );

    expect(shouldShowAssistantIdentity(continuation, previous), isFalse);
    expect(shouldShowAssistantIdentity(nextReply, previous), isTrue);
  });

  test('相邻消息角色变化时显示角色切换分隔', () {
    final previous = _message(id: 'a1', characterId: 'character-a');
    final current = _message(
      id: 'b1',
      characterId: 'character-b',
      role: MessageRole.user,
    );
    final currentAgain = _message(id: 'a2', characterId: 'character-a');

    expect(shouldShowRoleSwitch(previous, current), isTrue);
    expect(shouldShowRoleSwitch(current, currentAgain), isTrue);
  });

  test('同一角色连续消息不重复显示角色切换分隔', () {
    final previous = _message(id: 'a1', characterId: 'character-a');
    final current = _message(id: 'a2', characterId: 'character-a');

    expect(shouldShowRoleSwitch(previous, current), isFalse);
  });
}
