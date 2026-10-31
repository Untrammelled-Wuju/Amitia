import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/settings/chat_permission_preferences.dart';
import 'package:amitia_app/core/settings/chat_appearance_preferences.dart';
import 'package:amitia_app/features/chat/runtime/conversation_runtime_controller.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeBackendApi extends Fake implements BackendServiceApi {}

class _FakeOwnedService extends DeviceOwnedChatService {
  _FakeOwnedService() : super(_FakeBackendApi()) {
    enabled = true;
    coreId = 'core';
  }

  late _FakeChatService owner;

  Map<String, dynamic> scope() => {
    'coreId': coreId,
    'roleId': 'role',
    'resourceOwnerId': 'device',
    'roleOwnerId': 'device',
  };

  @override
  Future<bool> refresh() async => enabled;

  @override
  Future<void> poll() async {}

  @override
  Future<Map<String, dynamic>> query(
    String conversationId, {
    String? characterId,
    bool older = false,
    String keyword = '',
  }) async => {
    'conversationId': conversationId,
    'executionScope': scope(),
    if (owner.groupSnapshot != null) 'snapshot': {'group': owner.groupSnapshot},
  };

  @override
  Stream<Map<String, dynamic>> sendGroup({
    required String requestId,
    required String message,
    required String conversationId,
    required String characterId,
    required OwnedGroupState group,
    required List<Map<String, dynamic>> mentions,
    int? modelConfigId,
    String? reasoningEffort,
    bool? reasoningEnabled,
    String? permissionMode,
    String? messageStyle,
    Map<String, dynamic>? context,
    List<Map<String, dynamic>>? attachments,
    bool discussionMode = false,
  }) {
    owner.submittedRequestId = requestId;
    owner.submittedGroupMentions = mentions;
    owner.submittedGroupDiscussionMode = discussionMode;
    return owner.groupStream!.stream;
  }

  @override
  List<MessageDto> messages(Map<String, dynamic> result) =>
      owner.snapshotMessages?.call(result['conversationId'].toString()) ?? [];

  @override
  bool hasMore(
    String conversationId, {
    String? characterId,
    String keyword = '',
  }) => false;

  @override
  Stream<Map<String, dynamic>> send({
    required String requestId,
    required String message,
    String? conversationId,
    String? projectId,
    String? characterId,
    int? modelConfigId,
    String? reasoningEffort,
    bool? reasoningEnabled,
    String? permissionMode,
    String? messageStyle,
    Map<String, dynamic>? context,
    List<Map<String, dynamic>>? attachments,
    Map<String, dynamic>? quote,
  }) async* {
    if (owner.submitError != null) throw owner.submitError!;
    owner.submitCount++;
    owner.submittedMessage = message;
    owner.submittedStyle = messageStyle ?? '';
    owner.submittedRequestId = requestId;
    if (owner.streamed != null) {
      yield* owner.streamed!.stream;
      return;
    }
    final id = conversationId?.isNotEmpty == true
        ? conversationId!
        : 'conversation-1';
    yield {'type': 'started', 'conversationId': id, 'executionScope': scope()};
    yield {
      'type': 'completed',
      'data': {
        'conversationId': id,
        'executionScope': scope(),
        'reply': '回答',
        if (owner.completedCitationSnapshot.isNotEmpty)
          'citationSnapshot': owner.completedCitationSnapshot,
        'saved': true,
        'memoryStatus': 'completed',
      },
    };
    if (owner.failAfterCompleted) {
      throw StateError('connection closed after commit');
    }
  }
}

class _FakeChatService extends ChatService {
  _FakeChatService(this.events, {this.submitError})
    : super(_FakeBackendApi(), owned: _FakeOwnedService()) {
    (owned as _FakeOwnedService).owner = this;
  }

  final StreamController<ChatStreamEvent> events;
  final Object? submitError;
  String submittedMessage = '';
  String submittedRequestId = '';
  String submittedStyle = '';
  List<Map<String, dynamic>> submittedGroupMentions = const [];
  bool submittedGroupDiscussionMode = false;
  int submitCount = 0;
  Completer<void>? mutation;
  List<MessageDto> Function(String)? snapshotMessages;
  List<AssistantTurnDto> snapshotTurns = [];
  bool failAfterCompleted = false;
  StreamController<Map<String, dynamic>>? streamed;
  StreamController<Map<String, dynamic>>? groupStream;
  Map<String, dynamic>? groupSnapshot;
  bool moreTurns = false;
  String turnCursor = '';
  String requestedTurnCursor = '';
  List<Map<String, dynamic>> completedCitationSnapshot =
      const <Map<String, dynamic>>[];
  Completer<AssistantTurnPageDto>? pendingTurnPage;

  @override
  Future<AssistantTurnPageDto> getAssistantTurnPage(
    String conversationId, {
    int before = 0,
    String cursor = '',
    int limit = 50,
  }) async {
    requestedTurnCursor = cursor;
    return pendingTurnPage?.future ??
        const AssistantTurnPageDto(items: [], nextBefore: 0, hasMore: false);
  }

  @override
  Future<void> deleteMessage(
    String messageId, {
    Map<String, dynamic>? expectedScope,
    String? expectedOwnerId,
    int? expectedRevision,
  }) async {
    await mutation?.future;
  }

  @override
  Future<void> updateMessage(
    String messageId,
    String content, {
    Map<String, dynamic>? expectedScope,
    String? expectedOwnerId,
    int? expectedRevision,
  }) async {
    await mutation?.future;
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
      messages: snapshotMessages?.call(conversationId) ?? const <MessageDto>[],
      turns: snapshotTurns,
      messageNextBefore: 0,
      hasMoreMessages: false,
      turnNextBefore: 0,
      turnNextCursor: turnCursor,
      hasMoreTurns: moreTurns,
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
  test('群成员并发流各自更新且分别完成', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final phases = StreamController<Map<String, dynamic>>();
    final service = _FakeChatService(events)
      ..groupStream = phases
      ..groupSnapshot = {
        'conversationId': 'conversation-1',
        'conversationRoleId': 'role',
        'roleOwnerId': 'device',
        'members': [
          {
            'memberId': 'a',
            'characterId': 'role-a',
            'characterName': '甲',
            'characterAvatar': 'avatar-a',
            'status': 'active',
          },
          {
            'memberId': 'b',
            'characterId': 'role-b',
            'characterName': '乙',
            'characterAvatar': 'avatar-b',
            'status': 'active',
          },
        ],
      };
    final controller = ConversationRuntimeController(service);
    await controller.openConversation('conversation-1', characterId: 'role');
    final sending = controller.sendText('@所有成员 你好');
    await Future<void>.delayed(Duration.zero);
    Future<void> emit(Map<String, dynamic> event) async {
      phases.add(event);
      await Future<void>.delayed(Duration.zero);
    }

    final scopeA = {
      ...(service.owned as _FakeOwnedService).scope(),
      'roleId': 'role-a',
      'requestId': 'child-a',
    };
    final scopeB = {
      ...(service.owned as _FakeOwnedService).scope(),
      'roleId': 'role-b',
      'requestId': 'child-b',
    };
    Map<String, dynamic> member(
      String type,
      String id,
      int index,
      Map<String, dynamic> scope,
    ) => {
      'type': type,
      'conversationId': 'conversation-1',
      'groupRoundId': 'round',
      'deliveryId': id,
      'childRequestId': scope['requestId'],
      'deliveryIndex': index,
      'characterId': scope['roleId'],
      'characterName': id == 'a' ? '甲' : '乙',
      'executionScope': scope,
    };
    await emit(member('group.delivery.started', 'a', 0, scopeA));
    await emit(member('group.delivery.started', 'b', 1, scopeB));
    events.add(
      ChatStreamEvent(
        'agent_ui_event',
        _event(
          sequence: 1,
          id: 'start-a',
          type: 'turn.started',
          requestId: 'child-a',
        ),
      ),
    );
    await Future<void>.delayed(const Duration(milliseconds: 30));
    expect(
      controller.messages
          .where((message) => message.role == MessageRole.assistant)
          .length,
      2,
    );
    await emit(member('started', 'a', 0, scopeA));
    await emit(member('started', 'b', 1, scopeB));
    await emit({...member('delta', 'a', 0, scopeA), 'text': '甲回复'});
    await emit({...member('delta', 'b', 1, scopeB), 'text': '乙回复'});
    await emit(member('group.delivery.completed', 'b', 1, scopeB));
    events.add(const ChatStreamEvent('snapshot.required', {}));
    await Future<void>.delayed(const Duration(milliseconds: 20));
    final rows = controller.messages
        .where((message) => message.deliveryId.isNotEmpty)
        .toList();
    expect(rows.map((message) => message.content), ['甲回复', '乙回复']);
    expect(rows.map((message) => message.characterAvatar), [
      'avatar-a',
      'avatar-b',
    ]);
    expect(rows.map((message) => message.status), [
      MessageStatus.streaming,
      MessageStatus.delivered,
    ]);
    expect(controller.sending, isTrue);
    await emit(member('group.delivery.completed', 'a', 0, scopeA));
    await emit({
      'type': 'group.completed',
      'data': {
        'conversationId': 'conversation-1',
        'userMessageId': '${service.submittedRequestId}/user',
        'deliveries': [
          {
            'requestId': 'child-a',
            'deliveryId': 'a',
            'groupRoundId': 'round',
            'executionScope': scopeA,
            'reply': '甲回复',
            'saved': true,
          },
          {
            'requestId': 'child-b',
            'deliveryId': 'b',
            'groupRoundId': 'round',
            'executionScope': scopeB,
            'reply': '乙回复',
            'saved': true,
          },
        ],
      },
    });
    await phases.close();
    await sending;
    expect(
      controller.messages
          .where((message) => message.deliveryId.isNotEmpty)
          .length,
      2,
    );
    expect(controller.sending, isFalse);
    events.add(
      ChatStreamEvent(
        'agent_ui_event',
        _event(
          sequence: 1,
          id: 'late-group-start',
          type: 'turn.started',
          requestId: 'child-b',
        ),
      ),
    );
    await Future<void>.delayed(const Duration(milliseconds: 30));
    expect(controller.sending, isFalse);
    controller.dispose();
    await events.close();
  });
  test('群聊讨论模式发送空点名并声明讨论调度', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final phases = StreamController<Map<String, dynamic>>();
    final service = _FakeChatService(events)
      ..groupStream = phases
      ..groupSnapshot = {
        'conversationId': 'conversation-1',
        'conversationRoleId': 'role',
        'roleOwnerId': 'device',
        'membershipRevision': 1,
        'settingsRevision': 2,
        'groupSettings': {'allowCharacterMentions': true, 'maxAutoReplies': 6},
        'members': [
          {
            'memberId': 'a',
            'characterId': 'role-a',
            'characterName': '甲',
            'status': 'active',
          },
          {
            'memberId': 'b',
            'characterId': 'role-b',
            'characterName': '乙',
            'status': 'active',
          },
        ],
      };
    final controller = ConversationRuntimeController(service);
    await controller.openConversation('conversation-1', characterId: 'role');
    controller.setGroupDiscussionMode(true);
    final sending = controller.sendText('讨论小商家服务方案');
    await Future<void>.delayed(Duration.zero);
    expect(service.submittedGroupDiscussionMode, isTrue);
    expect(service.submittedGroupMentions, isEmpty);
    phases.add({
      'type': 'group.completed',
      'data': {
        'conversationId': 'conversation-1',
        'userMessageId': '${service.submittedRequestId}/user',
        'deliveries': [],
      },
    });
    await phases.close();
    await sending;
    controller.dispose();
    await events.close();
  });
  test('发送立即显示占位，思考和正文分别结束且等待保存确认', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final phases = StreamController<Map<String, dynamic>>();
    final service = _FakeChatService(events)..streamed = phases;
    final controller = ConversationRuntimeController(service);
    final sending = controller.sendText('你好');
    await Future<void>.delayed(Duration.zero);
    ChatMessage assistant() => controller.messages
        .where((m) => m.role == MessageRole.assistant)
        .single;
    expect(assistant().status, MessageStatus.queued);
    Future<void> emit(Map<String, dynamic> event) async {
      phases.add(event);
      await Future<void>.delayed(Duration.zero);
    }

    await emit({
      'type': 'started',
      'conversationId': 'conversation-1',
      'executionScope': (service.owned as _FakeOwnedService).scope(),
    });
    expect(assistant().status, MessageStatus.streaming);
    await emit({'type': 'delta', 'reasoning': true, 'text': '思考内容'});
    expect(assistant().reasoningComplete, isFalse);
    await emit({'type': 'reasoning-completed'});
    await emit({'type': 'delta', 'text': '正文'});
    expect(assistant().reasoningComplete, isTrue);
    await emit({'type': 'output-completed'});
    expect(assistant().outputComplete, isTrue);
    expect(assistant().sourceRevision, isNull);
    expect(controller.sending, isTrue);
    await emit({
      'type': 'completed',
      'data': {'reply': '正文', 'reasoning': '思考内容', 'saved': true},
    });
    await phases.close();
    await sending;
    expect(assistant().status, MessageStatus.delivered);
    expect(controller.sending, isFalse);
    controller.dispose();
    await events.close();
  });

  test('保存成功后的连接异常不能把回复改回中断', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events)..failAfterCompleted = true;
    final controller = ConversationRuntimeController(service);
    await controller.sendText('你好');
    expect(
      controller.messages
          .where((message) => message.role == MessageRole.assistant)
          .single
          .status,
      MessageStatus.delivered,
    );
    expect(controller.lastError, isNull);
    controller.dispose();
    await events.close();
  });

  test('已保存消息恢复为完成执行时同步清除中断状态', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    service.snapshotMessages = (_) => [
      MessageDto(
        id: 'reply',
        conversationId: 'conversation-1',
        role: 'assistant',
        content: '成功回复',
        status: 'interrupted',
        createdAt: '2026-10-24T00:00:00Z',
      ),
    ];
    service.snapshotTurns = [
      AssistantTurnDto(
        id: 'turn-1',
        conversationId: 'conversation-1',
        requestId: 'request-1',
        executionId: 'execution-1',
        status: 'completed',
        items: [
          AssistantTurnItemDto(
            id: 'text-1',
            turnId: 'turn-1',
            conversationId: 'conversation-1',
            type: 'text',
            status: 'completed',
            messageId: 'reply',
            content: '成功回复',
          ),
        ],
      ),
    ];
    final controller = ConversationRuntimeController(service);
    await controller.openConversation('conversation-1');
    expect(controller.messages.single.status, MessageStatus.delivered);
    expect(controller.messages.single.assistantTurn!.status, 'completed');
    controller.dispose();
    await events.close();
  });

  test('执行历史分页使用服务端游标并在页面结束后停止', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events)
      ..moreTurns = true
      ..turnCursor = 'owner-bound-cursor';
    final controller = ConversationRuntimeController(service);
    await controller.openConversation('conversation-1');
    expect(controller.hasMoreHistory, isTrue);
    await controller.loadOlderMessages();
    expect(service.requestedTurnCursor, 'owner-bound-cursor');
    expect(controller.hasMoreHistory, isFalse);
    controller.dispose();
    await events.close();
  });

  test('切换会话后忽略延迟返回的执行历史页面', () async {
    SharedPreferences.setMockInitialValues({});
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events)
      ..moreTurns = true
      ..turnCursor = 'first-owner-page';
    final controller = ConversationRuntimeController(service);
    await controller.openConversation('first');
    service.pendingTurnPage = Completer<AssistantTurnPageDto>();
    final pending = controller.loadOlderMessages();
    await Future<void>.delayed(Duration.zero);
    service.moreTurns = false;
    await controller.openConversation('second');
    service.pendingTurnPage!.complete(
      const AssistantTurnPageDto(
        items: [],
        nextBefore: 0,
        nextCursor: 'foreign-cursor',
        hasMore: true,
      ),
    );
    expect(await pending, isFalse);
    expect(controller.conversationId, 'second');
    expect(controller.hasMoreHistory, isFalse);
    controller.dispose();
    await events.close();
  });
  test('旧页面的编辑或删除确认不能修改后来打开的同名消息', () async {
    SharedPreferences.setMockInitialValues({});
    for (final edit in [true, false]) {
      final events = StreamController<ChatStreamEvent>.broadcast();
      final service = _FakeChatService(events);
      service.snapshotMessages = (conversationId) => [
        MessageDto(
          id: 'same-message',
          conversationId: conversationId,
          role: 'user',
          content: conversationId,
          createdAt: '2026-10-04T00:00:00Z',
        ),
      ];
      final controller = ConversationRuntimeController(service);
      await controller.openConversation('first');
      service.mutation = Completer<void>();
      final operation = edit
          ? controller.editMessage('same-message', 'edited')
          : controller.deleteMessage('same-message');
      await Future<void>.delayed(Duration.zero);
      await controller.openConversation('second');
      service.mutation!.complete();
      await operation;
      expect(controller.conversationId, 'second');
      expect(controller.messages.single.content, 'second');
      controller.dispose();
      await events.close();
    }
  });
  test('loads saved bubble mode before sending the request', () async {
    SharedPreferences.setMockInitialValues({
      chatMessageStyleStorageKey: 'bubble',
    });
    final appearance = ChatAppearancePreferencesNotifier();
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    final controller = ConversationRuntimeController(
      service,
      appearancePreferences: appearance,
    );
    await controller.sendText('你好');
    expect(service.submittedStyle, 'bubble');
    controller.dispose();
    appearance.dispose();
    await events.close();
  });
  test('完成回执中的引用快照写入当前助手消息', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events)
      ..completedCitationSnapshot = const [
        {
          'marker': '[K1]',
          'kind': 'knowledge',
          'index': 1,
          'sourceId': 'source-a',
          'title': '项目资料',
          'excerpt': '项目代号是晨星。',
        },
      ];
    final controller = ConversationRuntimeController(service);
    await controller.sendText('请检索知识库资料');
    final assistant = controller.messages.singleWhere(
      (message) => message.role == MessageRole.assistant,
    );
    expect(assistant.citationSnapshot, service.completedCitationSnapshot);
    controller.dispose();
    await events.close();
  });
  test('自动化指示随工具事件更新，断线后等待确认，完成后撤下', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final controller = ConversationRuntimeController(_FakeChatService(events));
    await controller.openConversation('conversation-1');
    Future<void> emit(int sequence, String type, String status) async {
      events.add(
        ChatStreamEvent('agent_ui_event', {
          ..._event(
            sequence: sequence,
            id: 'automation-$sequence',
            type: type,
            status: status,
          ),
          'blockId': 'automation',
          'blockSequence': 1,
          'revision': sequence,
          'payload': {'toolName': 'android_interaction_click'},
        }),
      );
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
  test('Owned保存回执不伪造Turn状态，运行状态只接受服务端事件', () async {
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
      isNotEmpty,
    );
    final placeholder = controller.messages.singleWhere(
      (message) => message.role == MessageRole.assistant,
    );
    expect(placeholder.renderId, 'request:${service.submittedRequestId}');
    expect(placeholder.assistantTurn, isNull);
    expect(placeholder.status, MessageStatus.delivered);

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

  test('Core解绑后原内联附件不能回退为本地模型请求', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    service.owned.enabled = false;
    final controller = ConversationRuntimeController(service);
    await controller.sendImage(
      resourceUri: 'data:image/png;base64,AQID',
      displayUrl: 'data:image/png;base64,AQID',
      fileName: 'picture.png',
    );
    await controller.sendVideo(
      resourceUri: 'data:video/mp4;base64,AQID',
      displayUrl: 'data:video/mp4;base64,AQID',
      fileName: 'clip.mp4',
    );
    await controller.sendFile(
      resourceUri: 'data:text/plain;base64,AQID',
      fileName: 'notes.txt',
      sizeBytes: 3,
      mimeType: 'text/plain',
    );
    expect(service.submitCount, 0);
    expect(controller.messages, isEmpty);
    expect(controller.lastError.toString(), contains('设备对话服务尚未就绪'));
    controller.dispose();
    await events.close();
  });

  test('openConversation 只接受 v1 Snapshot 作为权威状态', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    final controller = ConversationRuntimeController(service);

    await controller.openConversation('conversation-1');

    expect(controller.conversationId, 'conversation-1');
    expect(controller.conversationTitle, '测试会话');
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

  test('不同会话运行实例可以并发发送并独立保留后台完成数', () async {
    final events = StreamController<ChatStreamEvent>.broadcast();
    final service = _FakeChatService(events);
    final registry = ConversationRuntimeRegistry(service);
    final first = registry.session('conversation-a');
    final second = registry.session('conversation-b');
    await first.openConversation('conversation-a');
    await second.openConversation('conversation-b');
    registry.setVisibleConversation('conversation-a');

    await Future.wait([first.sendText('第一条'), second.sendText('第二条')]);

    expect(identical(first, second), isFalse);
    expect(service.submitCount, 2);
    final completedConversation = second.conversationId!;
    expect(
      registry.activityFor(completedConversation)?.unreadCompletedCount,
      1,
    );

    registry.dispose();
    await events.close();
  });

  test('群聊手动输入和粘贴的成员名称直接生成提及元数据', () {
    final members = [
      OwnedGroupMember.fromJson({
        'memberId': 'member-a',
        'characterId': 'role-a',
        'roleOwnerId': 'owner',
        'characterName': '角色甲',
        'status': 'active',
      }),
      OwnedGroupMember.fromJson({
        'memberId': 'member-b',
        'characterId': 'role-b',
        'roleOwnerId': 'owner',
        'characterName': '角色乙',
        'status': 'active',
      }),
      OwnedGroupMember.fromJson({
        'memberId': 'member-left',
        'characterId': 'role-left',
        'roleOwnerId': 'owner',
        'characterName': '已离开',
        'status': 'left',
      }),
    ];

    expect(groupMentionsFromText('请 @角色甲 和 @角色乙 一起回答', members), [
      {'memberId': 'member-a', 'characterId': 'role-a', 'roleOwnerId': 'owner'},
      {'memberId': 'member-b', 'characterId': 'role-b', 'roleOwnerId': 'owner'},
    ]);
    expect(groupMentionsFromText('@所有成员 你好', members), [
      {'memberId': 'member-a', 'characterId': 'role-a', 'roleOwnerId': 'owner'},
      {'memberId': 'member-b', 'characterId': 'role-b', 'roleOwnerId': 'owner'},
    ]);
  });
}
