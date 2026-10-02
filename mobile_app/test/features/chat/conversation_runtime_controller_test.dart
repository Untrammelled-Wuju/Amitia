import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/settings/chat_permission_preferences.dart';
import 'package:amitia_app/features/chat/runtime/conversation_runtime_controller.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeBackendApi extends Fake implements BackendServiceApi {}

class _FakeChatService extends ChatService {
  _FakeChatService(this.events, {this.submitError}) : super(_FakeBackendApi());

  final StreamController<ChatStreamEvent> events;
  final Object? submitError;
  String submittedMessage = '';
  String submittedRequestId = '';
  int submitCount = 0;

  @override
  Future<ChatSubmitResult> submitMessage({
    required String message,
    String? clientMessageId,
    String? conversationId,
    String? characterId,
    String? imageUrl,
    String? audioUrl,
    double audioDuration = 0,
    String? videoUrl,
    String? replyToMessageId,
    int? modelConfigId,
    String? reasoningEffort,
    bool? reasoningEnabled,
    String? permissionMode,
    ConversationWorkspaceDto? workspace,
  }) async {
    if (submitError != null) throw submitError!;
    submitCount += 1;
    submittedMessage = message;
    submittedRequestId = clientMessageId ?? 'request-1';
    return ChatSubmitResult(
      conversationId: conversationId?.isNotEmpty == true
          ? conversationId!
          : 'conversation-1',
      userMessageId: 'message-user-1',
      requestId: clientMessageId ?? 'request-1',
      turnId: 'turn-1',
      executionId: 'execution-1',
      status: 'queued',
    );
  }

  @override
  ChatStreamCancellation createStreamCancellation() => ChatStreamCancellation();

  @override
  Stream<ChatStreamEvent> conversationEvents({
    required String conversationId,
    required int afterSequence,
    required ChatStreamCancellation cancellation,
  }) {
    return events.stream;
  }

  @override
  Future<ConversationSnapshotDto> conversationSnapshot(
    String conversationId,
  ) async {
    return ConversationSnapshotDto(
      version: 1,
      revision: 1,
      lastEventSequence: 0,
      conversation: ConversationDto(
        id: conversationId,
        title: '测试会话',
        channel: 'web',
        source: 'mobile',
        reasoningEnabled: 1,
      ),
      workspace: null,
      messages: const <MessageDto>[],
      turns: const <AssistantTurnDto>[],
      messageNextBefore: 0,
      hasMoreMessages: false,
      turnNextBefore: 0,
      hasMoreTurns: false,
      activeTurn: null,
      approvals: const <Map<String, dynamic>>[],
    );
  }
}

Map<String, dynamic> _event({
  required int sequence,
  required String id,
  required String type,
  String status = '',
  String requestId = 'request-1',
}) {
  return <String, dynamic>{
    'version': 1,
    'eventId': id,
    'eventSequence': sequence,
    'conversationId': 'conversation-1',
    'requestId': requestId,
    'executionId': 'execution-1',
    'turnId': 'turn-1',
    'turnSequence': 1,
    'type': type,
    'status': status,
    'payload': <String, dynamic>{},
    'createdAt': '2026-09-21T10:00:00Z',
  };
}

void main() {
  test('自动化指示随工具事件更新，断线后等待确认，完成后撤下', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final controller = ConversationRuntimeController(_FakeChatService(events));
    await controller.openConversation('conversation-1');
    Future<void> emit(int sequence, String type, String status) async {
      events.add(ChatStreamEvent('agent_ui_event', {
        ..._event(sequence: sequence, id: 'automation-$sequence', type: type, status: status),
        'blockId': 'automation',
        'blockSequence': 1,
        'revision': sequence,
        'payload': {'toolName': 'android_interaction_click'},
      }));
      await Future<void>.delayed(const Duration(milliseconds: 30));
    }
    await emit(1, 'turn.started', 'running');
    await emit(2, 'tool.running', 'running');
    expect(controller.automationStatus?.label, '正在点击');
    events.addError(StateError('disconnected'));
    await Future<void>.delayed(const Duration(milliseconds: 30));
    expect(controller.automationStatus?.label, '正在同步自动化状态');
    await Future<void>.delayed(const Duration(milliseconds: 950));
    await emit(3, 'tool.completed', 'completed');
    expect(controller.automationStatus, isNull);
    controller.dispose();
    await events.close();
  });
  test('Command ACK 不在客户端伪造 Turn 状态，queued 必须来自服务端事件', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    final controller = ConversationRuntimeController(service);

    await controller.sendText('你好');

    expect(service.submitCount, 1);
    expect(service.submittedMessage, '你好');
    expect(controller.conversationId, 'conversation-1');
    expect(controller.activeTurnId, isEmpty);
    expect(controller.messages, hasLength(2));
    expect(
      controller.messages
          .singleWhere((message) => message.role == MessageRole.user)
          .id,
      'message-user-1',
    );
    final placeholder = controller.messages.singleWhere(
      (message) => message.role == MessageRole.assistant,
    );
    expect(placeholder.renderId, 'request:${service.submittedRequestId}');
    expect(placeholder.assistantTurn, isNull);
    expect(placeholder.status, MessageStatus.queued);

    events.add(
      ChatStreamEvent(
        'agent_ui_event',
        _event(
          sequence: 1,
          id: 'evt-1',
          type: 'turn.queued',
          status: 'queued',
          requestId: service.submittedRequestId,
        ),
      ),
    );
    await Future<void>.delayed(Duration.zero);
    await Future<void>.delayed(Duration.zero);
    await Future<void>.delayed(const Duration(milliseconds: 60));

    expect(controller.activeTurnId, 'turn-1');
    expect(controller.sending, isTrue);
    final assistant = controller.messages.singleWhere(
      (message) => message.role == MessageRole.assistant,
    );
    expect(assistant.renderId, 'request:${service.submittedRequestId}');
    expect(assistant.assistantTurn?.id, 'turn-1');

    controller.dispose();
    await events.close();
  });

  test('发送失败时移除 AI 占位消息', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events, submitError: StateError('failed'));
    final controller = ConversationRuntimeController(service);

    await controller.sendText('失败消息');

    expect(
      controller.messages.where((message) => message.role == MessageRole.user),
      hasLength(1),
    );
    expect(
      controller.messages.where(
        (message) => message.role == MessageRole.assistant,
      ),
      isEmpty,
    );
    expect(controller.sending, isFalse);

    controller.dispose();
    await events.close();
  });

  test('openConversation 只接受 v1 Snapshot 作为权威状态', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    final controller = ConversationRuntimeController(service);

    await controller.openConversation('conversation-1');

    expect(controller.conversationId, 'conversation-1');
    expect(controller.messages, isEmpty);
    expect(controller.activeTurnId, isEmpty);

    controller.dispose();
    await events.close();
  });

  test('新草稿恢复持久化的权限模式', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{
      chatPermissionModeStorageKey: chatPermissionFullAccess,
    });
    final preferences = ChatPermissionPreferencesNotifier();
    await preferences.init();
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    final controller = ConversationRuntimeController(
      service,
      permissionPreferences: preferences,
    );

    expect(controller.permissionMode, chatPermissionFullAccess);

    await controller.updatePermissionMode(chatPermissionRequestApproval);
    controller.startDraft();

    expect(controller.permissionMode, chatPermissionRequestApproval);
    expect(preferences.state, chatPermissionRequestApproval);

    controller.dispose();
    await events.close();
  });
}
