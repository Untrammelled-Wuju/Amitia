import 'package:amitia_app/app/theme/app_theme.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/toolbox/presentation/pages/toolbox_file_browser_page.dart';
import 'package:amitia_app/features/toolbox/presentation/pages/toolbox_workspace_page.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_page.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_remote_browser_view.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _WorkspaceApi extends Fake implements BackendServiceApi {
  final requests = <String>[];

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    if (path == '/api/workspaces') return <dynamic>[] as T;
    if (path.startsWith('/api/workbench/browser/')) {
      return <String, dynamic>{'ready': false, 'observed': false} as T;
    }
    throw StateError('意外请求: $path');
  }
}

Widget _workbench({
  bool fullscreen = false,
  VoidCallback? onExpand,
  String conversationId = 'conversation-a',
  _WorkspaceApi? api,
}) {
  final workspaceApi = api ?? _WorkspaceApi();
  return ProviderScope(
    overrides: [backendServiceProvider.overrideWithValue(workspaceApi)],
    child: MaterialApp(
      theme: AppTheme.lightTheme(),
      home: WorkbenchPage(
        conversationId: conversationId,
        fullscreen: fullscreen,
        onExpand: onExpand,
      ),
    ),
  );
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('工作区在 360、390、768 宽度下没有布局溢出', (tester) async {
    final api = _WorkspaceApi();
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(360, 800);
    await tester.pumpWidget(
      _workbench(fullscreen: true, conversationId: '', api: api),
    );
    await tester.pumpAndSettle();
    expect(find.text('工作区'), findsOneWidget);
    for (final width in [360.0, 390.0, 768.0]) {
      tester.view.physicalSize = Size(width, 800);
      await tester.pump();
      expect(tester.takeException(), isNull, reason: '${width.toInt()}px');
    }
    tester.view.physicalSize = const Size(390, 800);
    await tester.pump();
    expect(find.text('工作区'), findsOneWidget);
    expect(find.text('文件'), findsOneWidget);
    expect(find.byTooltip('新建标签'), findsOneWidget);
    expect(find.byType(AppBar), findsNothing);

    await tester.tap(find.byTooltip('新建标签'));
    await tester.pumpAndSettle();
    expect(find.text('打开工作区'), findsOneWidget);
    expect(find.text('浏览工作区文件和文档'), findsOneWidget);
    expect(find.text('查看变更、Diff、分支与 Worktree'), findsOneWidget);
    expect(find.text('启动真实 Chromium 会话'), findsOneWidget);
    await tester.tap(find.text('Git'));
    await tester.pumpAndSettle();
    expect(find.text('Git'), findsOneWidget);
    expect(find.byType(AppBar), findsNothing);

    await tester.tap(find.byTooltip('更多'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('固定标签'));
    await tester.pumpAndSettle();
    expect(find.byIcon(Icons.push_pin), findsWidgets);

    await tester.tap(find.byTooltip('新建标签'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('浏览工作区文件和文档'));
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('更多'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('关闭其他标签'));
    await tester.pumpAndSettle();
    expect(find.text('文件'), findsOneWidget);
    expect(find.text('Git'), findsOneWidget);

    await tester.tap(find.byTooltip('更多'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('关闭未固定标签'));
    await tester.pumpAndSettle();
    expect(find.text('文件'), findsNothing);
    expect(find.text('Git'), findsOneWidget);
    await tester.tap(find.byTooltip('新建标签'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('启动真实 Chromium 会话'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    expect(
      api.requests.any(
        (path) => path.startsWith('/api/workbench/browser/__draft__/'),
      ),
      isTrue,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('嵌入式文件与 Git 页面不再渲染嵌套 AppBar', (tester) async {
    final api = _WorkspaceApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [backendServiceProvider.overrideWithValue(api)],
        child: MaterialApp(
          theme: AppTheme.lightTheme(),
          home: const Scaffold(
            body: Column(
              children: [
                Expanded(child: ToolboxFileBrowserPage(embedded: true)),
                Expanded(child: ToolboxWorkspacePage(embedded: true)),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(ToolboxFileBrowserPage), findsOneWidget);
    expect(find.byType(ToolboxWorkspacePage), findsOneWidget);
    expect(find.byType(AppBar), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('浏览器标签默认使用可启动的 HTTP(S) 地址', (tester) async {
    final api = _WorkspaceApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [backendServiceProvider.overrideWithValue(api)],
        child: MaterialApp(
          theme: AppTheme.lightTheme(),
          home: const Scaffold(
            body: WorkbenchRemoteBrowserView(
              conversationId: 'conversation-a',
              tabId: 'browser-tab',
              initialUrl: '',
              active: false,
            ),
          ),
        ),
      ),
    );
    final address = tester.widget<TextField>(find.byType(TextField));
    expect(address.controller?.text, 'https://example.com');
    expect(tester.takeException(), isNull);
  });
}
