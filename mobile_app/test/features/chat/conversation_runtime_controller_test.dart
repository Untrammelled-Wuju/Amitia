import 'dart:async';

import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/services/channel_service.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/features/chat/runtime/conversation_runtime_controller.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeBackendApi extends Fake implements BackendServiceApi {}

class _FakeEmoteService extends Fake implements EmoteService {}

class _FakeChatService extends ChatService {
  _FakeChatService({
    required this.streamFactory,
    required this.messagesFactory,
    this.turnsFactory,
  }) : super(_FakeBackendApi());

  final Stream<ChatStreamEvent> Function(String requestId) streamFactory;
  final List<MessageDto> Function(String requestId, bool latest)
  messagesFactory;
  final List<AssistantTurnDto> Function()? turnsFactory;
  bool latestRequested = false;
  String? updatedMessageId;
  String? updatedMessageContent;

  @override
  ChatStreamCancellation createStreamCancellation() => ChatStreamCancellation();

  @override
  Stream<ChatStreamEvent> submitMessageStream({
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
    required ChatStreamCancellation cancellation,
  }) {
    return streamFactory(clientMessageId ?? '');
  }

  @override
  Future<List<MessageDto>> getMessages(
    String conversationId, {
    int page = 1,
    int pageSize = 200,
    bool latest = false,
  }) async {
    latestRequested = latest;
    return messagesFactory(clientMessageId, latest);
  }

  @override
  Future<List<AssistantTurnDto>> getAssistantTurns(
    String conversationId, {
    int limit = 500,
  }) async {
    return turnsFactory?.call() ?? const <AssistantTurnDto>[];
  }

  @override
  Future<String> generationStatus(String conversationId) async => 'completed';

  @override
  Future<void> updateMessage(String messageId, String content) async {
    updatedMessageId = messageId;
    updatedMessageContent = content;
  }

  @override
  Future<ConversationDto?> createConversation({String? projectId}) async {
    return ConversationDto(
      id: 'conversation-1',
      projectId: projectId?.trim() ?? '',
      title: '新对话',
      channel: 'web',
      source: 'mobile',
    );
  }

  String clientMessageId = '';

  void rememberRequestId(String value) {
    clientMessageId = value;
  }
}

MessageDto _message({
  required String id,
  required String role,
  required String content,
  required String requestId,
  required String createdAt,
  required int sequence,
  String characterId = '',
  String? responseGroupId,
  int deliverySequence = 0,
  String reasoningContent = '',
  int reasoningDurationMs = 0,
}) {
  return MessageDto(
    id: id,
    conversationId: 'conversation-1',
    characterId: characterId,
    role: role,
    content: content,
    requestId: requestId,
    responseGroupId: responseGroupId ?? requestId,
    deliverySequence: deliverySequence,
    sequence: sequence,
    createdAt: createdAt,
    reasoningContent: reasoningContent,
    reasoningDurationMs: reasoningDurationMs,
  );
}

void main() {
  test('starting a draft resets the conversation and emits a draft epoch', () {
    final controller = ConversationRuntimeController(
      _FakeChatService(
        streamFactory: (_) => const Stream<ChatStreamEvent>.empty(),
        messagesFactory: (_, _) => const <MessageDto>[],
      ),
      _FakeEmoteService(),
    );

    expect(controller.draftEpoch, 0);
    controller.startDraft();
    expect(controller.conversationId, isNull);
    expect(controller.messages, isEmpty);
    expect(controller.workspace, isNull);
    expect(controller.draftEpoch, 1);

    controller.startDraft(
      workspace: const ConversationWorkspaceDto(
        projectId: 'project-1',
        workspaceId: 'workspace-1',
        workspaceName: '项目',
        rootUri: 'amitia://workspace/@workspace-1/',
      ),
    );
    expect(controller.workspace?.projectId, 'project-1');
    expect(controller.draftEpoch, 2);

    controller.dispose();
  });

  test('send retries while the business backend is starting', () async {
    late _FakeChatService service;
    var attempts = 0;
    service = _FakeChatService(
      streamFactory: (requestId) async* {
        attempts++;
        if (attempts == 1) {
          throw const BusinessBackendUnavailable(
            phase: RuntimeStatusPhase.starting,
            generation: 1,
          );
        }
        service.rememberRequestId(requestId);
        yield ChatStreamEvent('queued', <String, dynamic>{
          'conversationId': 'conversation-1',
          'requestId': requestId,
        });
      },
      messagesFactory: (requestId, latest) => <MessageDto>[
        _message(
          id: 'user-1',
          role: 'user',
          content: '你好',
          requestId: requestId,
          createdAt: '2026-09-19 18:00:00',
          sequence: 1,
        ),
        _message(
          id: 'assistant-1',
          role: 'assistant',
          content: '你好呀',
          characterId: 'character-1',
          deliverySequence: 1,
          requestId: requestId,
          createdAt: '2026-09-19 18:00:01',
          sequence: 2,
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.sendText('你好');

    expect(attempts, 2);
    expect(controller.lastError, isNull);
    expect(controller.messages, hasLength(2));
    expect(controller.messages.last.status, MessageStatus.delivered);
    expect(controller.messages.last.characterId, 'character-1');
    expect(controller.messages.last.responseGroupId, isNotEmpty);
    expect(controller.messages.last.deliverySequence, 1);

    controller.dispose();
  });

  test('conversation id is assigned before the reply stream starts', () async {
    final streamStarted = Completer<void>();
    final releaseStream = Completer<void>();
    late _FakeChatService service;
    service = _FakeChatService(
      streamFactory: (requestId) async* {
        streamStarted.complete();
        await releaseStream.future;
        service.rememberRequestId(requestId);
        yield ChatStreamEvent('queued', <String, dynamic>{
          'conversationId': 'conversation-1',
          'requestId': requestId,
        });
      },
      messagesFactory: (requestId, latest) => <MessageDto>[
        _message(
          id: 'user-1',
          role: 'user',
          content: '你好',
          requestId: requestId,
          createdAt: '2026-09-19 18:00:00',
          sequence: 1,
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    final send = controller.sendText('你好');
    await streamStarted.future;

    expect(controller.conversationId, 'conversation-1');

    releaseStream.complete();
    await send;
    controller.dispose();
  });

  test('queued reply keeps user and assistant identities distinct', () async {
    late final _FakeChatService service;
    service = _FakeChatService(
      streamFactory: (requestId) async* {
        service.rememberRequestId(requestId);
        yield ChatStreamEvent('queued', <String, dynamic>{
          'conversationId': 'conversation-1',
          'requestId': requestId,
        });
      },
      messagesFactory: (requestId, latest) => <MessageDto>[
        _message(
          id: 'user-1',
          role: 'user',
          content: '你好',
          requestId: requestId,
          createdAt: '2026-09-19 18:00:00',
          sequence: 1,
        ),
        _message(
          id: 'assistant-1',
          role: 'assistant',
          content: '你好呀',
          requestId: requestId,
          createdAt: '2026-09-19 18:00:00',
          sequence: 2,
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.sendText('你好');

    expect(controller.messages, hasLength(2));
    expect(controller.messages.map((message) => message.role), <MessageRole>[
      MessageRole.user,
      MessageRole.assistant,
    ]);
    expect(
      controller.messages.map((message) => message.renderId).toSet(),
      hasLength(2),
    );
    expect(controller.messages.map((message) => message.sequence), <int?>[
      1,
      2,
    ]);
    expect(controller.messages.first.content, '你好');
    expect(controller.messages.last.content, '你好呀');

    controller.dispose();
  });

  test('latest sync keeps a streaming assistant reply', () async {
    late final _FakeChatService service;
    service = _FakeChatService(
      streamFactory: (requestId) async* {
        service.rememberRequestId(requestId);
        yield ChatStreamEvent('message_start', <String, dynamic>{
          'conversationId': 'conversation-1',
          'userMessageId': 'user-1',
        });
        yield ChatStreamEvent('token', <String, dynamic>{
          'id': 'assistant-1',
          'conversationId': 'conversation-1',
          'role': 'assistant',
          'content': '正在回复',
          'createdAt': '2026-09-19T18:00:00Z',
        });
      },
      messagesFactory: (requestId, latest) => <MessageDto>[
        _message(
          id: 'user-1',
          role: 'user',
          content: '你好',
          requestId: requestId,
          createdAt: '2026-09-19T18:00:00Z',
          sequence: 1,
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.sendText('你好');

    expect(service.latestRequested, isTrue);
    expect(controller.messages.map((message) => message.id), <String>[
      'user-1',
      'assistant-1',
    ]);
    expect(controller.messages.last.content, '正在回复');

    await controller.openConversation('conversation-1');

    expect(controller.messages.map((message) => message.id), <String>[
      'user-1',
      'assistant-1',
    ]);
    expect(controller.messages.last.content, '正在回复');

    controller.dispose();
  });

  test('latest sync replaces the oldest page with the newest page', () async {
    final service = _FakeChatService(
      streamFactory: (requestId) => const Stream<ChatStreamEvent>.empty(),
      messagesFactory: (requestId, latest) => latest
          ? <MessageDto>[
              _message(
                id: 'latest-user',
                role: 'user',
                content: '最新消息',
                requestId: 'latest-request',
                createdAt: '2026-09-19T18:00:00Z',
                sequence: 201,
              ),
              _message(
                id: 'latest-assistant',
                role: 'assistant',
                content: '最新回复',
                requestId: 'latest-request',
                createdAt: '2026-09-19T18:00:01Z',
                sequence: 202,
              ),
            ]
          : <MessageDto>[
              _message(
                id: 'old-user',
                role: 'user',
                content: '旧消息',
                requestId: 'old-request',
                createdAt: '2026-09-19T17:00:00Z',
                sequence: 1,
              ),
            ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.openConversation('conversation-1');

    expect(service.latestRequested, isTrue);
    expect(controller.messages.map((message) => message.id), <String>[
      'latest-user',
      'latest-assistant',
    ]);

    controller.dispose();
  });

  test('edit message updates the backend and local ledger', () async {
    final service = _FakeChatService(
      streamFactory: (_) => const Stream<ChatStreamEvent>.empty(),
      messagesFactory: (_, _) => <MessageDto>[
        _message(
          id: 'user-edit',
          role: 'user',
          content: '旧内容',
          requestId: 'request-edit',
          createdAt: '2026-09-20 10:00:00',
          sequence: 1,
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.openConversation('conversation-1');
    await controller.editMessage('user-edit', '  新内容  ');

    expect(service.updatedMessageId, 'user-edit');
    expect(service.updatedMessageContent, '新内容');
    expect(controller.messages.single.content, '新内容');

    controller.dispose();
  });

  test('assistant failures are not retryable', () async {
    final service = _FakeChatService(
      streamFactory: (_) => const Stream<ChatStreamEvent>.empty(),
      messagesFactory: (_, _) => <MessageDto>[
        _message(
          id: 'user-retry',
          role: 'user',
          content: '你好',
          requestId: 'request-retry',
          createdAt: '2026-09-20 10:00:00',
          sequence: 1,
        ),
        MessageDto(
          id: 'assistant-retry',
          conversationId: 'conversation-1',
          role: 'assistant',
          content: '',
          requestId: 'request-retry',
          createdAt: '2026-09-20 10:00:01',
          sequence: 2,
          status: 'failed',
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.openConversation('conversation-1');

    expect(controller.canRetryMessage(0), isFalse);
    expect(controller.canRetryMessage(1), isFalse);

    controller.dispose();
  });

  test('missing backend reasoning duration stays unset', () async {
    final service = _FakeChatService(
      streamFactory: (_) => const Stream<ChatStreamEvent>.empty(),
      messagesFactory: (_, _) => <MessageDto>[
        _message(
          id: 'user-duration',
          role: 'user',
          content: '你好',
          requestId: 'request-duration',
          createdAt: '2026-09-20 10:00:00.000',
          sequence: 1,
        ),
        _message(
          id: 'assistant-duration',
          role: 'assistant',
          content: '你好呀',
          requestId: 'request-duration',
          createdAt: '2026-09-20 10:00:02.850',
          sequence: 2,
          reasoningContent: '思考内容',
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.openConversation('conversation-1');

    expect(controller.messages.last.reasoningDurationMs, 0);

    controller.dispose();
  });

  test('live reasoning duration uses API duration only', () async {
    final service = _FakeChatService(
      streamFactory: (requestId) async* {
        yield ChatStreamEvent('message_start', <String, dynamic>{
          'conversationId': 'conversation-1',
          'userMessageId': 'user-live',
          'reasoningContent': '思考内容',
          'reasoningDurationMs': 2850,
        });
        await Future<void>.delayed(const Duration(milliseconds: 30));
        yield ChatStreamEvent('token', <String, dynamic>{
          'id': 'assistant-live',
          'conversationId': 'conversation-1',
          'role': 'assistant',
          'content': '回复',
          'createdAt': '2026-09-20 10:00:02.850',
        });
      },
      messagesFactory: (_, _) => <MessageDto>[
        _message(
          id: 'user-live',
          role: 'user',
          content: '你好',
          requestId: 'request-live',
          createdAt: '2026-09-20 10:00:00.000',
          sequence: 1,
        ),
        _message(
          id: 'assistant-live',
          role: 'assistant',
          content: '回复',
          requestId: 'request-live',
          createdAt: '2026-09-20 10:00:02.850',
          sequence: 2,
          reasoningContent: '思考内容',
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.sendText('你好');

    expect(controller.messages.last.reasoningDurationMs, 2850);

    controller.dispose();
  });

  test('assistant turn projection keeps ordered items and suppresses split messages', () async {
    final service = _FakeChatService(
      streamFactory: (_) => const Stream<ChatStreamEvent>.empty(),
      messagesFactory: (_, _) => <MessageDto>[
        _message(
          id: 'user-turn',
          role: 'user',
          content: '执行工具',
          requestId: 'request-turn',
          createdAt: '2026-09-20 10:00:00.000',
          sequence: 1,
        ),
        _message(
          id: 'assistant-turn-1',
          role: 'assistant',
          content: '第一段',
          requestId: 'request-turn',
          deliverySequence: 1,
          createdAt: '2026-09-20 10:00:01.000',
          sequence: 2,
        ),
        _message(
          id: 'assistant-turn-2',
          role: 'assistant',
          content: '第二段',
          requestId: 'request-turn',
          deliverySequence: 2,
          createdAt: '2026-09-20 10:00:02.000',
          sequence: 3,
        ),
      ],
      turnsFactory: () => <AssistantTurnDto>[
        AssistantTurnDto(
          id: 'turn-1',
          conversationId: 'conversation-1',
          requestId: 'request-turn',
          responseGroupId: 'request-turn',
          status: 'completed',
          items: const <AssistantTurnItemDto>[
            AssistantTurnItemDto(
              id: 'item-1',
              turnId: 'turn-1',
              sequence: 1,
              type: 'thinking',
              status: 'completed',
              content: '思考',
            ),
            AssistantTurnItemDto(
              id: 'item-2',
              turnId: 'turn-1',
              sequence: 2,
              type: 'tool_call',
              status: 'completed',
              callId: 'call-1',
              toolName: 'query_memory',
            ),
            AssistantTurnItemDto(
              id: 'item-3',
              turnId: 'turn-1',
              sequence: 3,
              type: 'tool_result',
              status: 'completed',
              callId: 'call-1',
              toolName: 'query_memory',
              resultJson: '"ok"',
            ),
            AssistantTurnItemDto(
              id: 'item-4',
              turnId: 'turn-1',
              sequence: 4,
              type: 'text',
              status: 'completed',
              content: '最终回复',
              isFinal: true,
            ),
          ],
        ),
      ],
    );
    final controller = ConversationRuntimeController(
      service,
      _FakeEmoteService(),
    );

    await controller.openConversation('conversation-1');

    final first = controller.messages.firstWhere(
      (message) => message.id == 'assistant-turn-1',
    );
    final second = controller.messages.firstWhere(
      (message) => message.id == 'assistant-turn-2',
    );
    expect(first.assistantTurn, isNotNull);
    expect(
      first.assistantTurn!.items.map((item) => item.type).toList(),
      <String>['thinking', 'tool_call', 'tool_result', 'text'],
    );
    expect(second.assistantTurnSuppressed, isTrue);

    controller.dispose();
  });
}
