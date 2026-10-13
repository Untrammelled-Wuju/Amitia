import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_page.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_timeline.dart';
import 'package:amitia_app/features/memory/presentation/pages/memory_timeline_page.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import '../../core/services/owned_task_submission_test.dart' show TaskTransport;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  test('真实更新时间排序保留不同Owner同ID和无时间记录，不修改原数据', () {
    final rows = [
      <String, dynamic>{
        'id': 'same',
        'ownerId': 'source',
        'body': {'created_at': '2026-10-01T10:00:00Z'},
      },
      <String, dynamic>{
        'id': 'same',
        'ownerId': 'core',
        'updatedAt': '2026-10-02T10:00:00Z',
      },
      <String, dynamic>{
        'id': 'unknown',
        'ownerId': 'core',
        'updatedAt': 'invalid',
      },
    ];
    final sorted = ownedMemoryTimelineRows(rows);
    expect(sorted.map((row) => '${row['ownerId']}/${row['id']}'), [
      'core/same',
      'source/same',
      'core/unknown',
    ]);
    expect(rows.first['ownerId'], 'source');
    expect(ownedMemoryResourceTime(rows.last), isNull);
    expect(identical(sorted[1], rows.first), isTrue);
  });

  test('Owner内容和旧Source记录的嵌套时间使用真实字段，更新时间优先', () {
    final row = <String, dynamic>{
      'body': {
        'createdAt': '2026-09-01T00:00:00Z',
        'content': {
          'updated_at': '2026-10-01T00:00:00Z',
          'created_at': '2026-08-01T00:00:00Z',
        },
      },
    };
    expect(ownedMemoryResourceTime(row), DateTime.utc(2026, 10, 1));
  });

  testWidgets('时间线始终复用当前唯一记忆入口', (tester) async {
    Widget? selected;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) {
            selected = const MemoryTimelinePage().build(context);
            return const SizedBox();
          },
        ),
      ),
    );
    expect(selected, isA<OwnedMemoryPage>());
    expect((selected as OwnedMemoryPage).timeline, isTrue);
  });

  testWidgets('归属服务失败明确显示错误，不回退旧时间线', (tester) async {
    final source = FailingMemorySource(BackendServiceApi(TaskTransport([]), 1));
    final router = GoRouter(routes: [
      GoRoute(path: '/', builder: (_, _) => const MemoryTimelinePage()),
    ]);
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [memoryDataSourceProvider.overrideWithValue(source)],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('当前记忆服务不可用'), findsOneWidget);
  });
}

class FailingMemorySource extends DeviceOwnedChatService {
  FailingMemorySource(super.api);
  @override
  Future<bool> refresh() async => throw StateError('当前记忆服务不可用');
}
