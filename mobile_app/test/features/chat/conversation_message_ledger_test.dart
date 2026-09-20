import 'package:amitia_app/features/chat/runtime/conversation_message_ledger.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

ChatMessage _message({
  String characterId = '',
  String responseGroupId = '',
  int deliverySequence = 0,
}) {
  return ChatMessage(
    id: 'assistant-1',
    characterId: characterId,
    role: MessageRole.assistant,
    type: MessageType.text,
    content: '回复',
    time: DateTime(2026, 9, 20, 12),
    responseGroupId: responseGroupId,
    deliverySequence: deliverySequence,
  );
}

void main() {
  test('身份字段变化时消息账本产生更新', () {
    final ledger = ConversationMessageLedger();
    ledger.upsert(_message());

    expect(
      ledger.upsert(
        _message(
          characterId: 'character-1',
          responseGroupId: 'request-1',
          deliverySequence: 1,
        ),
      ),
      isTrue,
    );
  });

  test('身份字段完全相同时消息账本不重复更新', () {
    final ledger = ConversationMessageLedger();
    final message = _message(
      characterId: 'character-1',
      responseGroupId: 'request-1',
      deliverySequence: 1,
    );

    ledger.upsert(message);
    expect(ledger.upsert(message), isFalse);
  });
}
