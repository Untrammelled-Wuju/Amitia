import 'package:amitia_app/features/conversation/rendering/assistant_identity.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

ChatMessage _message({
  required String id,
  String characterId = 'character-1',
  MessageRole role = MessageRole.assistant,
}) {
  return ChatMessage(
    id: id,
    characterId: characterId,
    role: role,
    type: MessageType.text,
    content: id,
    time: DateTime(2026, 9, 20),
  );
}

void main() {
  test('同一 AI 相邻消息只在第一条显示身份', () {
    final previous = _message(id: 'assistant-1');
    final current = _message(id: 'assistant-2');

    expect(shouldShowAssistantIdentity(current, previous), isFalse);
    expect(shouldCompactAfterAssistantMessage(previous, current), isTrue);
  });

  test('不同 AI 相邻发言重新显示身份', () {
    final previous = _message(id: 'assistant-1', characterId: 'character-1');
    final current = _message(id: 'assistant-2', characterId: 'character-2');

    expect(shouldShowAssistantIdentity(current, previous), isTrue);
    expect(shouldCompactAfterAssistantMessage(previous, current), isFalse);
  });

  test('用户消息后重新显示 AI 身份', () {
    final previous = _message(
      id: 'user-1',
      characterId: '',
      role: MessageRole.user,
    );
    final current = _message(id: 'assistant-1');

    expect(shouldShowAssistantIdentity(current, previous), isTrue);
  });

  test('发送者缺失时不再通过回复分组猜测连续性', () {
    final previous = _message(id: 'assistant-1', characterId: '');
    final current = _message(id: 'assistant-2', characterId: '');

    expect(shouldShowAssistantIdentity(current, previous), isTrue);
    expect(shouldCompactAfterAssistantMessage(previous, current), isFalse);
  });

  test('相邻角色变化时显示角色切换分隔', () {
    final previous = _message(id: 'a1', characterId: 'character-a');
    final current = _message(id: 'b1', characterId: 'character-b');

    expect(shouldShowRoleSwitch(previous, current), isTrue);
  });
}
