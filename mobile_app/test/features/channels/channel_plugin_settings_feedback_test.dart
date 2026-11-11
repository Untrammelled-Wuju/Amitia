import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/channel_sandbox_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/features/channels/presentation/pages/channel_plugin_settings_view.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class TestSandboxApi extends Fake implements BackendServiceApi {
  int reads = 0;
  int writes = 0;
  final pending = Completer<void>();
  Completer<Map<String, dynamic>>? pendingRead;
  Object? readFailure;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    reads++;
    if (readFailure != null) throw readFailure!;
    if (pendingRead != null) return await pendingRead!.future as T;
    return {
          'channelId': queryParameters?['channelId'],
          'enabled': false,
          'allowNetwork': true,
        }
        as T;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    writes++;
    await pending.future;
    return data as T;
  }
}

void main() {
  testWidgets('读取失败展示可复制的原始错误并支持重试', (tester) async {
    final api = TestSandboxApi()..readFailure = StateError('渠道读取原始错误');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          channelSandboxServiceProvider.overrideWithValue(
            ChannelSandboxService(api),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: ChannelPluginSettingsView(channelId: 'test', title: '渠道'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.textContaining('渠道读取原始错误'), findsOneWidget);
    expect(find.byType(SelectableText), findsOneWidget);
    api.readFailure = null;
    await tester.tap(find.text('重试'));
    await tester.pumpAndSettle();
    expect(api.reads, 2);
    expect(find.byType(SwitchListTile), findsNWidgets(2));
  });
  testWidgets('切换渠道后旧保存结果不会覆盖当前渠道', (tester) async {
    final api = TestSandboxApi();
    Widget page(String channelId) => ProviderScope(
      overrides: [
        channelSandboxServiceProvider.overrideWithValue(
          ChannelSandboxService(api),
        ),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: ChannelPluginSettingsView(channelId: channelId, title: '渠道'),
        ),
      ),
    );
    await tester.pumpWidget(page('first'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(SwitchListTile).first);
    await tester.pump();
    await tester.pumpWidget(page(''));
    await tester.pump();
    api.pending.complete();
    await tester.pumpAndSettle();
    expect(find.text('渠道标识为空，无法读取渠道设置。'), findsOneWidget);
    expect(find.byType(SwitchListTile), findsNothing);
  });
  testWidgets('切换渠道后旧保存失败仍可查看原始错误', (tester) async {
    final api = TestSandboxApi();
    Widget page(String channelId) => ProviderScope(
      overrides: [
        channelSandboxServiceProvider.overrideWithValue(
          ChannelSandboxService(api),
        ),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: ChannelPluginSettingsView(channelId: channelId, title: '渠道'),
        ),
      ),
    );
    await tester.pumpWidget(page('first'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(SwitchListTile).first);
    await tester.pump();
    await tester.pumpWidget(page(''));
    await tester.pump();
    api.pending.completeError(StateError('旧请求原始错误'));
    await tester.pumpAndSettle();
    expect(find.text('渠道 first 保存失败'), findsOneWidget);
    expect(find.textContaining('旧请求原始错误'), findsOneWidget);
  });
  testWidgets('切换为空渠道后旧读取结果不会覆盖提示', (tester) async {
    final api = TestSandboxApi()
      ..pendingRead = Completer<Map<String, dynamic>>();
    Widget page(String channelId) => ProviderScope(
      overrides: [
        channelSandboxServiceProvider.overrideWithValue(
          ChannelSandboxService(api),
        ),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: ChannelPluginSettingsView(channelId: channelId, title: '渠道'),
        ),
      ),
    );
    await tester.pumpWidget(page('test'));
    await tester.pump();
    expect(api.reads, 1);
    await tester.pumpWidget(page(''));
    await tester.pump();
    api.pendingRead!.complete({'channelId': 'test', 'enabled': true});
    await tester.pumpAndSettle();
    expect(find.text('渠道标识为空，无法读取渠道设置。'), findsOneWidget);
    expect(find.byType(SwitchListTile), findsNothing);
  });
  testWidgets('渠道标识为空时显示原因而不是持续加载', (tester) async {
    final api = TestSandboxApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          channelSandboxServiceProvider.overrideWithValue(
            ChannelSandboxService(api),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: ChannelPluginSettingsView(channelId: '', title: '渠道'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('渠道标识为空，无法读取渠道设置。'), findsOneWidget);
    expect(api.reads, 0);
  });
  testWidgets('渠道策略接口读取后限制未启用沙箱的联网设置', (tester) async {
    final api = TestSandboxApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          channelSandboxServiceProvider.overrideWithValue(
            ChannelSandboxService(api),
          ),
        ],
        child: const MaterialApp(
          home: Scaffold(
            body: ChannelPluginSettingsView(channelId: 'test', title: '渠道'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.reads, 1);
    expect(
      tester.widget<SwitchListTile>(find.byType(SwitchListTile).last).onChanged,
      isNull,
    );
    await tester.tap(find.byType(SwitchListTile).last);
    await tester.pump();
    expect(find.text('请先开启沙箱隔离。'), findsOneWidget);
    expect(api.writes, 0);
    ScaffoldMessenger.of(
      tester.element(find.byType(SwitchListTile).first),
    ).hideCurrentSnackBar();
    await tester.pumpAndSettle();
    await tester.tap(find.byType(SwitchListTile).first);
    await tester.pump();
    expect(api.writes, 1);
    await tester.tap(find.byType(SwitchListTile).last);
    await tester.pump();
    expect(find.text('正在保存渠道设置，请稍后重试。'), findsOneWidget);
    expect(api.writes, 1);
    api.pending.complete();
    await tester.pumpAndSettle();
  });
}
