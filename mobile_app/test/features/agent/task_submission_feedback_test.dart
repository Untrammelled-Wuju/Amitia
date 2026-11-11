import 'dart:async';

import 'package:amitia_app/core/services/extension_task_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/agent/presentation/pages/agent_page.dart';
import 'package:amitia_app/features/agent/presentation/providers/agent_tasks_provider.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class TestTaskNotifier extends AgentTaskNotifier {
  int writes = 0;
  @override
  Future<List<AgentTaskItem>> build() async => [];
  @override
  Future<void> createTask(Map<String, dynamic> submission) async {
    writes++;
  }
}

class TestTaskService extends Fake implements ExtensionTaskService {
  int reads = 0;
  final pending = Completer<Map<String, dynamic>>();
  final options = <String, dynamic>{
    'roles': [
      {'id': 'role', 'name': '角色', 'revision': 1},
    ],
    'executionScope': <String, dynamic>{},
  };
  @override
  Future<Map<String, dynamic>> prepareOwnedTask(String targetDeviceId) async {
    reads++;
    return reads == 1 ? options : pending.future;
  }

  @override
  Future<List<Map<String, dynamic>>> listOwnedTaskDefinitions(
    Map<String, dynamic> options,
    String characterId,
  ) async => [
    {'taskId': 'task'},
  ];
  @override
  Future<Map<String, dynamic>> listOwnedDeviceTaskCatalog(
    Map<String, dynamic> options,
    String characterId, {
    String cursor = '',
  }) async => {'revision': '1', 'entries': [], 'nextCursor': ''};
}

void main() {
  testWidgets('任务提交期间禁用控件提示且不重复请求', (tester) async {
    final service = TestTaskService();
    final notifier = TestTaskNotifier();
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (context, state) => const AgentPage()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          agentTasksProvider.overrideWith(() => notifier),
          agentTaskDevicesProvider.overrideWith(
            (ref) async => [
              const AgentTaskDeviceOption(
                deviceId: 'device',
                label: '设备',
                platform: 'android',
              ),
            ],
          ),
          extensionTaskServiceProvider.overrideWithValue(service),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('新建任务'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('处理文件'));
    await tester.pumpAndSettle();
    final definitions = find.byType(DropdownButtonFormField<String>).last;
    await tester.ensureVisible(definitions);
    await tester.tap(definitions);
    await tester.pumpAndSettle();
    await tester.tap(find.text('task · local').last);
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('开始任务'));
    await tester.tap(find.text('开始任务'));
    await tester.pump();
    expect(service.reads, 2);
    await tester.tapAt(
      tester.getCenter(find.text('提交中…'), warnIfMissed: false),
    );
    await tester.pump();
    expect(find.text('正在提交任务，请稍后重试。'), findsOneWidget);
    expect(service.reads, 2);
    ScaffoldMessenger.of(
      tester.element(find.text('提交中…')),
    ).hideCurrentSnackBar();
    await tester.pump(const Duration(seconds: 1));
    final device = find.byType(DropdownButtonFormField<String>).first;
    await tester.ensureVisible(device);
    await tester.tapAt(tester.getCenter(device, warnIfMissed: false));
    await tester.pump();
    expect(find.text('正在提交任务，请稍后重试。'), findsOneWidget);
    expect(service.reads, 2);
    service.pending.complete(service.options);
    await tester.pumpAndSettle();
    expect(notifier.writes, 1);
  });
}
