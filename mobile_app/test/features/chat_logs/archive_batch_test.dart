import 'package:amitia_app/core/models/conversation.dart';
import 'package:amitia_app/core/models/project.dart';
import 'package:amitia_app/core/services/chat_service.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
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
  Future<ConversationPage> conversationPage({
    Map<String, ConversationListCursor>? cursors,
    String keyword = '',
    bool selectedRoleOnly = false,
    ConversationListFilter filter = const ConversationListFilter(),
  }) async =>
      (rows: List.of(rows), cursors: <String, ConversationListCursor>{});
  @override
  Future<ConversationSidebarDto> conversationSidebar([
    List<ConversationDto> conversations = const [],
  ]) async => const ConversationSidebarDto();
  @override
  Future<void> restoreArchivedConversation(
    String id, {
    String? characterId,
  }) async {
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

class _PagedArchiveService extends ArchiveService {
  int pageCalls = 0;

  @override
  Future<ConversationPage> conversationPage({
    Map<String, ConversationListCursor>? cursors,
    String keyword = '',
    bool selectedRoleOnly = false,
    ConversationListFilter filter = const ConversationListFilter(),
  }) async {
    pageCalls++;
    if (cursors == null) {
      return (
        rows: List.generate(
          24,
          (index) => ConversationDto(
            id: 'archive-$index',
            title: '归档条目 $index',
            archivedAt: '2026-01-01',
          ),
        ),
        cursors: {
          'role': const ConversationListCursor('scope', {'cursor': 'next'}),
        },
      );
    }
    expect(cursors['role']?.parameters, {'cursor': 'next'});
    return (
      rows: [
        ConversationDto(id: 'last', title: '下一页对话', archivedAt: '2025-12-31'),
      ],
      cursors: <String, ConversationListCursor>{},
    );
  }
}

void main() {
  testWidgets('归档滚动到列表末尾自动加载下一页', (tester) async {
    final service = _PagedArchiveService();
    final router = GoRouter(
      initialLocation: '/archive',
      routes: [
        GoRoute(path: '/archive', builder: (_, _) => const ChatLogsPage()),
      ],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [chatServiceProvider.overrideWithValue(service)],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    expect(service.pageCalls, 1);
    await tester.scrollUntilVisible(
      find.text('下一页对话'),
      360,
      maxScrolls: 20,
      scrollable: find.descendant(
        of: find.byType(RefreshIndicator),
        matching: find.byType(Scrollable),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('下一页对话'), findsOneWidget);
    expect(service.pageCalls, 2);
  });

  testWidgets('归档多选恢复保留失败项及原始错误', (tester) async {
    final service = ArchiveService()..failFirst = true;
    final router = GoRouter(
      initialLocation: '/archive',
      routes: [
        GoRoute(
          path: '/archive',
          builder: (context, state) => const ChatLogsPage(),
        ),
      ],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [chatServiceProvider.overrideWithValue(service)],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
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
    final router = GoRouter(
      initialLocation: '/archive',
      routes: [
        GoRoute(
          path: '/archive',
          builder: (context, state) => const ChatLogsPage(),
        ),
      ],
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [chatServiceProvider.overrideWithValue(service)],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
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
