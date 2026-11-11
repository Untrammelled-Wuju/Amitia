import 'dart:async';

import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';
import 'package:amitia_app/core/services/core_configuration_guard.dart';
import 'package:amitia_app/features/settings/presentation/pages/model_settings_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

void main() {
  testWidgets('权限检查失败时点击模型入口展示完整错误', (tester) async {
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, __) => const ModelSettingsPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          coreConfigurationAccessProvider.overrideWith(
            (ref) async => throw StateError('原始权限错误'),
          ),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('文本模型'));
    await tester.pumpAndSettle();
    expect(find.textContaining('原始权限错误'), findsOneWidget);
    expect(find.byType(SelectableText), findsOneWidget);
  });

  testWidgets('权限检查中点击模型入口解释状态', (tester) async {
    final pending = Completer<CoreConfigurationIntent>();
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, __) => const ModelSettingsPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          coreConfigurationAccessProvider.overrideWith((ref) => pending.future),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();
    await tester.tap(find.text('文本模型'));
    await tester.pumpAndSettle();
    expect(find.textContaining('正在确认模型配置权限，请稍后重试'), findsOneWidget);
  });
}
