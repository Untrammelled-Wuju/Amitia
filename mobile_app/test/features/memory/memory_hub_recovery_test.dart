import 'package:amitia_app/core/models/memory.dart';
import 'package:amitia_app/features/memory/presentation/pages/memory_hub_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  testWidgets('最近记忆保留可选取的原始错误，重试成功后恢复空态', (tester) async {
    tester.view.physicalSize = const Size(800, 1000);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (context, state) => const MemoryHubPage()),
      ],
    );
    addTearDown(router.dispose);
    var attempts = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          recentMemoryPageProvider.overrideWith((ref, cursor) async {
            if (++attempts == 1) {
              Error.throwWithStackTrace(
                StateError('原始 Core 错误：连接已断开'),
                StackTrace.fromString('qa-original-stack'),
              );
            }
            return const MemoryPage(items: [], nextCursor: '');
          }),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('原始错误'));
    await tester.pumpAndSettle();
    final raw = tester.widget<SelectableText>(find.byType(SelectableText));
    expect(raw.data, contains('原始 Core 错误：连接已断开'));
    expect(raw.data, contains('qa-original-stack'));
    await tester.tap(find.text('记忆加载失败，点击重试'));
    await tester.pumpAndSettle();
    expect(attempts, 2);
    expect(find.text('暂无最近记忆'), findsOneWidget);
    expect(find.text('原始错误'), findsNothing);
  });
}
