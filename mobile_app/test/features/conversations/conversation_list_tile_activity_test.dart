import 'package:amitia_app/core/models/character.dart';
import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/features/conversations/presentation/widgets/conversation_list_tile.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('消息项显示工具状态、未读数和时间下方加载图标', (tester) async {
    final conversation = ConversationDto(
      id: 'conversation-a',
      characterId: 'role-a',
      title: '测试对话',
      lastMessage: '上一条内容',
      lastTerminalTurnSequence: 3,
      lastReadTurnSequence: 0,
      unreadCompletedCount: 3,
      hasUnread: true,
      isGenerating: true,
      activityLabel: '正在使用 workspace.read 工具',
      updatedAt: DateTime.now().toIso8601String(),
    );

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: ConversationListTile(
              conversation: conversation,
              character: CharacterDto(id: 'role-a', name: '角色 A'),
              timeText: '12:30',
              showDivider: false,
              onTap: () {},
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.text('正在使用 workspace.read 工具'), findsOneWidget);
    expect(find.text('3'), findsOneWidget);
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.textContaining('条消息'), findsNothing);
  });
}
