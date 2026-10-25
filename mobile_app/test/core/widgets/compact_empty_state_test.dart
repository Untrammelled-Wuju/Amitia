import 'package:amitia_app/core/widgets/compact_empty_state.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('compact empty state keeps a list-sized footprint', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              CompactEmptyState(
                icon: Icons.person_add_alt_1_outlined,
                title: '还没有可用角色',
                message: '创建角色后即可开始聊天和创建群聊。',
                actionLabel: '创建角色',
                onAction: () {},
              ),
            ],
          ),
        ),
      ),
    );

    expect(find.text('还没有可用角色'), findsOneWidget);
    expect(find.text('创建角色'), findsOneWidget);
    expect(
      tester.getSize(find.byType(CompactEmptyState)).height,
      lessThan(140),
    );
  });
}
