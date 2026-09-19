import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/services/channel_service.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/features/chat/runtime/conversation_runtime_controller.dart';
import 'package:amitia_app/shared/models/models.dart';
import 'package:flutter_test/flutter_test.dart';

class _FakeBackendApi extends Fake implements BackendServiceApi {}

class _FakeEmoteService extends Fake implements EmoteService {}

class _FakeChatService extends ChatService {
  _FakeChatService({required this.streamFactory, required this.messagesFactory})
    : super(_FakeBackendApi());

  final Stream<ChatStreamEvent> Function(String requestId) streamFactory;
  final List<MessageDto> Function(String requestId) messagesFactory;

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
  }) async {
    return messagesFactory(clientMessageId);
  }

  @override
  Future<String> generationStatus(String conversationId) async => 'completed';

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
}) {
  return MessageDto(
    id: id,
    conversationId: 'conversation-1',
    role: role,
    content: content,
    requestId: requestId,
    sequence: sequence,
    createdAt: createdAt,
  );
}

void main() {
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
      messagesFactory: (requestId) => <MessageDto>[
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
}
