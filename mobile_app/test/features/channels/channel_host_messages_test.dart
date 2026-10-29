import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:amitia_app/features/channels/presentation/pages/channel_messages_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _Extensions extends Fake implements ExtensionService {}

class _Runtime extends UIRuntimeController {
  _Runtime(bool single)
    : super(
        _Extensions(),
        onLastKnownGood: (_) {},
        cacheNamespace: () async => 'test',
        meshDeviceId: () async => null,
      ) {
    state = AsyncValue.data(
      UIProviderSnapshot.fromJson({
        'providers': [
          {
            'providerId': 'channel',
            'extensionId': 'test/channel',
            'capability': 'channel.presentation',
            'mode': 'augment',
            'enabled': true,
            'builtin': false,
            'generation': 1,
            'entries': {
              'android': {'type': 'declarative'},
            },
            'metadata': {
              'channelId': 'test',
              'displayName': '测试渠道',
              'singleConversation': single,
            },
          },
        ],
        'slots': [],
        'profile': {
          'profileId': 'default',
          'name': 'Default',
          'selections': {},
        },
        'providerContext': {
          'platform': 'android',
          'localRuntime': true,
          'deviceOnline': true,
        },
        'resolved': {},
        'providerVersion': 1,
      }),
    );
  }
}

class _Chat extends Fake implements ChatService {
  final requested = <String>[];
  @override
  Future<List<ConversationDto>> channelConversations() async => [
    ConversationDto(
      id: 'first',
      title: '当前对话',
      channel: 'test',
      messageCount: 1,
    ),
    ConversationDto(id: 'other', title: '其他渠道', channel: 'other'),
  ];
  @override
  Future<List<MessageDto>> getMessages(
    String conversationId, {
    String? characterId,
    int beforeSequence = 0,
    int limit = 50,
  }) async {
    requested.add(conversationId);
    return [
      MessageDto.fromJson({
        'id': 'msg',
        'conversationId': conversationId,
        'role': 'user',
        'content': '渠道消息正文',
      }),
    ];
  }
}

void main() {
  for (final single in [true, false]) {
    testWidgets(
      single
          ? 'single channel renders messages inline'
          : 'multiple channel opens selected conversation messages',
      (tester) async {
        final chat = _Chat();
        final container = ProviderContainer(
          overrides: [
            chatServiceProvider.overrideWithValue(chat),
            uiRuntimeProvider.overrideWith((ref) => _Runtime(single)),
          ],
        );
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp.router(
              routerConfig: GoRouter(
                routes: [
                  GoRoute(
                    path: '/',
                    builder: (_, _) => const Scaffold(
                      body: ChannelHostMessagesView(
                        channelId: 'test',
                        showSummary: true,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('其他渠道'), findsNothing);
        if (!single) {
          expect(find.text('渠道消息正文'), findsNothing);
          await tester.tap(find.text('当前对话'));
          await tester.pumpAndSettle();
        }
        expect(find.text('渠道消息正文'), findsOneWidget);
        expect(chat.requested, ['first']);
        await tester.pumpWidget(const SizedBox());
        container.dispose();
      },
    );
  }
}
