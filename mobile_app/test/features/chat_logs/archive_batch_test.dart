import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/models/project.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/chat_logs/presentation/pages/chat_logs_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class ArchiveService extends Fake implements ChatService {
  final rows = <ConversationDto>[
    ConversationDto(id: 'one', title: '对话一', archivedAt: '2026-01-01'),
    ConversationDto(id: 'two', title: '对话二', archivedAt: '2026-01-02'),
  ];
  final restored = <String>[];
  final deleted = <String>[];
  bool failFirst = false;

  @override
  Future<List<ConversationDto>> archivedConversations({String projectId = '', String keyword = ''}) async => List.of(rows);
  @override
  Future<ConversationSidebarDto> conversationSidebar() async => const ConversationSidebarDto();
  @override
  Future<void> restoreArchivedConversation(String id, {String? characterId}) async {
    if (failFirst && id == 'one') throw StateError('原始恢复错误');
    restored.add(id);
    rows.removeWhere((row) => row.id == id);
  }
  @override
  Future<bool> deleteConversation(String id, {String? characterId}) async {
    deleted.add(id);
    rows.removeWhere((row) => row.id == id);
    return true;
  }
}

void main() {
  testWidgets('归档多选恢复保留失败项及原始错误', (tester) async {
    final service = ArchiveService()..failFirst = true;
    final router = GoRouter(initialLocation: '/archive', routes: [GoRoute(path: '/archive', builder: (context, state) => const ChatLogsPage())]);
    await tester.pumpWidget(ProviderScope(overrides: [chatServiceProvider.overrideWithValue(service)], child: MaterialApp.router(routerConfig: router)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('多选'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('全选'));
    await tester.pumpAndSettle();
    expect(find.text('取消全选'), findsOneWidget);
    await tester.tap(find.text('撤销'));
    await tester.pumpAndSettle();
    expect(service.restored, ['two']);
    expect(find.textContaining('原始恢复错误'), findsOneWidget);
    await tester.tap(find.text('关闭'));
    await tester.pumpAndSettle();
    expect(find.text('对话一'), findsOneWidget);
    expect(find.text('对话二'), findsNothing);
    expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isTrue);
  });

  testWidgets('归档永久删除需要确认', (tester) async {
    final service = ArchiveService();
    final router = GoRouter(initialLocation: '/archive', routes: [GoRoute(path: '/archive', builder: (context, state) => const ChatLogsPage())]);
    await tester.pumpWidget(ProviderScope(overrides: [chatServiceProvider.overrideWithValue(service)], child: MaterialApp.router(routerConfig: router)));
    await tester.pumpAndSettle();
    await tester.tap(find.text('多选'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('全选'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('删除（永久）'));
    await tester.pumpAndSettle();
    expect(service.deleted, isEmpty);
    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();
    expect(service.deleted, isEmpty);
    await tester.tap(find.text('删除（永久）'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('永久删除').last);
    await tester.pumpAndSettle();
    expect(service.deleted, ['one', 'two']);
    expect(find.text('暂无归档对话'), findsOneWidget);
  });
}
