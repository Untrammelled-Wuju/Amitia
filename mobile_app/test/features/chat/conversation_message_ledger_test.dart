import 'package:amitia_app/features/chat/runtime/conversation_message_ledger.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

ChatMessage _message({
  String id = 'assistant-1',
  String renderId = 'turn-1',
  String characterId = '',
  int sequence = 1,
  String content = '回复',
  String sourceOwnerId = '',
  int? sourceRevision,
}) {
  return ChatMessage(
    id: id,
    sourceOwnerId: sourceOwnerId,
    sourceRevision: sourceRevision,
    renderId: renderId,
    characterId: characterId,
    role: MessageRole.assistant,
    type: MessageType.text,
    content: content,
    time: DateTime(2026, 9, 20, 12),
    sequence: sequence,
  );
}

void main() {
  test('不同设备的同名消息和渲染编号独立保存并拒绝不明确的查找', () {
    final ledger = ConversationMessageLedger();
    final device = _message(sourceOwnerId: 'device', content: '设备历史');
    final core = _message(sourceOwnerId: 'core', content: '云端记录');
    ledger.upsert(device);
    ledger.upsert(core);
    expect(ledger.length, 2);
    expect(ledger.findById('assistant-1'), isNull);
    expect(ledger.findByRenderId('turn-1'), isNull);
    expect(ledger.findById('assistant-1', sourceOwnerId: 'device')?.content, '设备历史');
    ledger.upsert(_message(sourceOwnerId: 'core', content: '修改云端记录'));
    expect(ledger.findById('assistant-1', sourceOwnerId: 'device')?.content, '设备历史');
    expect(ledger.findById('assistant-1', sourceOwnerId: 'core')?.content, '修改云端记录');
    ledger.remove(device);
    expect(ledger.messages.single.sourceOwnerId, 'core');
  });

  test('读取版本变化会更新归属消息状态', () {
    final ledger = ConversationMessageLedger();
    ledger.upsert(_message(sourceOwnerId: 'device', sourceRevision: 1));
    expect(ledger.upsert(_message(sourceOwnerId: 'device', sourceRevision: 2)), isTrue);
    expect(ledger.messages.single.sourceRevision, 2);
  });
  test('同一 renderId 从实时 Turn 更新为持久消息时只保留一条', () {
    final ledger = ConversationMessageLedger();
    ledger.upsert(_message(id: 'turn:turn-1'));

    expect(
      ledger.upsert(
        _message(
          id: 'message-1',
          characterId: 'character-1',
          content: '最终回复',
        ),
      ),
      isTrue,
    );
    expect(ledger.length, 1);
    expect(ledger.messages.single.id, 'message-1');
    expect(ledger.messages.single.content, '最终回复');
  });

  test('完全相同的消息不会重复更新', () {
    final ledger = ConversationMessageLedger();
    final message = _message(characterId: 'character-1');

    ledger.upsert(message);
    expect(ledger.upsert(message), isFalse);
    expect(ledger.length, 1);
  });

  test('消息按服务端 sequence 稳定排序', () {
    final ledger = ConversationMessageLedger();
    ledger.upsert(_message(id: 'm2', renderId: 'm2', sequence: 2));
    ledger.upsert(_message(id: 'm1', renderId: 'm1', sequence: 1));

    expect(ledger.messages.map((item) => item.id), <String>['m1', 'm2']);
  });
}
