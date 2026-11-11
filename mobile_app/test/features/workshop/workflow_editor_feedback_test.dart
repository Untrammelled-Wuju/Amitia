import 'dart:async';

import 'package:amitia_app/core/services/extension_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/workshop/presentation/pages/workflow_editor_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class PendingWorkflowService extends Fake implements ExtensionService {
  final pending = Completer<Map<String, dynamic>>();
  int reads = 0;

  @override
  Future<Map<String, dynamic>> getWorkflow(
    String id, {
    WorkflowApiTarget target = const WorkflowApiTarget.cloud(),
  }) {
    reads++;
    return pending.future;
  }
}

void main() {
  testWidgets('工作流接口加载期间操作保持禁用', (tester) async {
    final service = PendingWorkflowService();
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) =>
              const WorkflowEditorPage(workflowId: 'test'),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [extensionServiceProvider.overrideWithValue(service)],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pump();
    expect(service.reads, 1);
    for (final tooltip in ['校验', '预检', '保存', '运行']) {
      final button = find.byWidgetPredicate(
        (widget) => widget is IconButton && widget.tooltip == tooltip,
      );
      expect(tester.widget<IconButton>(button).onPressed, isNull);
      await tester.tapAt(tester.getCenter(button));
      await tester.pump();
      expect(find.text('正在加载工作流，请稍后重试。'), findsOneWidget);
      ScaffoldMessenger.of(tester.element(button)).hideCurrentSnackBar();
      await tester.pump(const Duration(seconds: 1));
    }
    expect(service.reads, 1);
  });
}
