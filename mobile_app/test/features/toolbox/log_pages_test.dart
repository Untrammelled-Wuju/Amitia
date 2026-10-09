import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/core/widgets/log_folder_button.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/services/extension_task_service.dart';
import 'package:amitia_app/features/toolbox/presentation/pages/toolbox_task_log_page.dart';
import 'package:amitia_app/features/toolbox/presentation/pages/toolbox_log_page.dart';
import 'package:amitia_app/features/toolbox/presentation/pages/toolbox_prompt_trace_page.dart';
import 'package:amitia_app/features/developer/presentation/pages/dev_console_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class _LogApi implements BackendServiceApi {
  final requests = <String>[];
  bool fail = false;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    if (fail) throw StateError('测试连接不可用');
    final Object data;
    switch (path) {
      case '/api/logs/recent':
        data = {
          'logs': [
            {
              'file': 'app.log',
              'line':
                  '{"@level":"error","@message":"真实运行错误","@timestamp":"2026-10-08","source":"backend"}',
            },
            {'file': 'server.log', 'line': '原始运行日志', 'time': '2026-10-08'},
          ],
        };
      case '/api/logs/files':
        data = {
          'directory': '/opt/amitia/logs',
          'files': [
            {'name': '运行 日志.log', 'size': 42, 'modTime': '2026-10-08'},
          ],
        };
      case '/api/logs/files/%E8%BF%90%E8%A1%8C%20%E6%97%A5%E5%BF%97.log':
        data = '文件末尾的日志内容';
      case '/api/logs/prompt-traces':
        data = {'traces': []};
      case '/api/extensions/tasks':
        expect(queryParameters, {'limit': 200});
        data = {'items': [], 'total': 0};
      case '/api/dev-console/overview':
        data = <String, dynamic>{};
      default:
        throw StateError('意外请求: $path');
    }
    return fromJson == null ? data as T : fromJson(data);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Future<void> _render(WidgetTester tester, _LogApi api, Widget page) async {
  final router = GoRouter(
    routes: [GoRoute(path: '/', builder: (_, _) => page)],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        backendServiceProvider.overrideWithValue(api),
        extensionTaskServiceProvider.overrideWithValue(
          ExtensionTaskService(api),
        ),
      ],
      child: MaterialApp.router(routerConfig: router),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('任务日志无记录时保留导航与文件夹入口', (tester) async {
    await _render(tester, _LogApi(), const ToolboxTaskLogPage());
    expect(find.text('任务日志'), findsOneWidget);
    expect(find.text('暂无任务记录'), findsOneWidget);
    expect(find.byTooltip('打开文件夹'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
  testWidgets('运行日志显示结构化与文本记录', (tester) async {
    await _render(tester, _LogApi(), const ToolboxLogPage());
    expect(find.text('真实运行错误'), findsOneWidget);
    expect(find.text('原始运行日志'), findsOneWidget);
    expect(find.byTooltip('打开文件夹'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('打开真实日志目录并读取含中文空格文件', (tester) async {
    final api = _LogApi();
    await _render(tester, api, const ToolboxLogPage());
    await tester.tap(find.byTooltip('打开文件夹'));
    await tester.pumpAndSettle();
    expect(find.text('日志文件夹'), findsOneWidget);
    expect(find.text('/opt/amitia/logs'), findsOneWidget);
    await tester.tap(find.text('运行 日志.log'));
    await tester.pumpAndSettle();
    expect(find.text('文件末尾的日志内容'), findsOneWidget);
    expect(
      api.requests.last,
      '/api/logs/files/%E8%BF%90%E8%A1%8C%20%E6%97%A5%E5%BF%97.log',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('运行日志失败保留文件夹入口并可以重试', (tester) async {
    final api = _LogApi()..fail = true;
    await _render(tester, api, const ToolboxLogPage());
    expect(find.text('运行日志'), findsOneWidget);
    expect(find.byTooltip('打开文件夹'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('真实运行错误'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('Prompt Trace 空列表保留文件夹入口', (tester) async {
    await _render(tester, _LogApi(), const ToolboxPromptTracePage());
    expect(find.text('暂无 Prompt Trace'), findsOneWidget);
    expect(find.byTooltip('打开文件夹'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('日志目录读取失败支持重试', (tester) async {
    final api = _LogApi()..fail = true;
    await _render(tester, api, const LogFolderPage());
    expect(find.text('日志文件夹'), findsOneWidget);
    api.fail = false;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(find.text('运行 日志.log'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('诊断控制台显示后端运行日志', (tester) async {
    final api = _LogApi();
    await _render(tester, api, const DevConsolePage());
    expect(find.text('真实运行错误'), findsOneWidget);
    expect(find.text('原始运行日志'), findsOneWidget);
    expect(api.requests, contains('/api/logs/recent'));
    expect(find.byTooltip('打开文件夹'), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
    expect(tester.takeException(), isNull);
  });
}
