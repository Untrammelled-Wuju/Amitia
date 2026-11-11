import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/channel_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/extensions/presentation/pages/mcp_detail_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

class TestMcpApi extends Fake implements BackendServiceApi {
  final pending = Completer<void>();
  int reads = 0;
  int writes = 0;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    reads++;
    if (path == '/api/mcp/servers/demo') {
      return {'name': '测试 MCP', 'status': 'disconnected'} as T;
    }
    if (path.endsWith('/resources')) {
      return {'resources': [], 'resourceTemplates': []} as T;
    }
    return <dynamic>[] as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    writes++;
    await pending.future;
    return <String, dynamic>{} as T;
  }
}

void main() {
  testWidgets('MCP 连接请求进行期间不重复发起操作', (tester) async {
    final api = TestMcpApi();
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => const McpDetailPage(mcpId: 'demo'),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [mcpServiceProvider.overrideWithValue(MCPService(api))],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.reads, greaterThan(0));
    await tester.tap(find.text('连接'));
    await tester.pump();
    expect(api.writes, 1);
    expect(find.text('处理中...'), findsOneWidget);
    for (final label in ['处理中...', '断开', '重新连接', '重新发现能力']) {
      await tester.tap(find.text(label));
      await tester.pump();
      expect(find.text('正在处理 MCP 连接，请稍后重试。'), findsOneWidget);
      expect(api.writes, 1);
      ScaffoldMessenger.of(
        tester.element(find.text(label)),
      ).hideCurrentSnackBar();
      await tester.pump(const Duration(seconds: 1));
    }
    api.pending.complete();
    await tester.pumpAndSettle();
  });
}
