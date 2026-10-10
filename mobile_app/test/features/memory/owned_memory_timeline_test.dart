import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_page.dart';
import 'package:amitia_app/features/memory/presentation/pages/owned_memory_timeline.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

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

  testWidgets('绑定时间线使用Owner入口，未绑定才使用旧页面', (tester) async {
    Widget? selected;
    Future<void> render(bool bound) async {
      await tester.pumpWidget(
        ProviderScope(
          key: ValueKey(bound),
          overrides: [
            ownedMemoryModeProvider.overrideWith((ref) async => bound),
          ],
          child: MaterialApp(
            home: Consumer(
              builder: (context, ref, child) {
                selected = ownedMemoryGate(ref, timeline: true);
                return const SizedBox();
              },
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
    }

    await render(true);
    expect(selected, isA<OwnedMemoryPage>());
    expect((selected as OwnedMemoryPage).timeline, isTrue);
    await render(false);
    expect(selected, isNull);
  });

  testWidgets('归属查询失败不回退读取旧Core时间线', (tester) async {
    Widget? selected;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ownedMemoryModeProvider.overrideWith(
            (ref) async => throw StateError('Core已切换'),
          ),
        ],
        child: MaterialApp(
          home: Consumer(
            builder: (context, ref, child) {
              selected = ownedMemoryGate(ref, timeline: true);
              return const SizedBox();
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(selected, isNotNull);
    expect(selected, isNot(isA<OwnedMemoryPage>()));
  });
}
