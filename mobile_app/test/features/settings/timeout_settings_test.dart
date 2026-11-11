import 'dart:async';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/settings/presentation/widgets/timeout_settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/runtime/backend/mobile_backend_providers.dart';
import 'package:amitia_app/core/runtime/backend/mobile_deployment_mode.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:shared_preferences/shared_preferences.dart';

class TimeoutApi implements BackendServiceApi {
  @override
  int get generation => 1;
  bool admin = false;
  int permissionRevision = 1;
  int reads = 0;
  Map<String, dynamic> settings = {'disabled': false, 'seconds': 180};
  int writes = 0;
  Completer<void>? pendingWrite;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (path.endsWith('/coordination/me')) {
      return {
            'coreId': 'core',
            'coordinationAvailable': true,
            'canAdminister': admin,
            'policy': {
              'coordinated': true,
              'providerEpoch': 1,
              'modeRevision': 1,
              'permissionRevision': permissionRevision,
            },
          }
          as T;
    }
    reads++;
    return Map<String, dynamic>.from(settings) as T;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    await pendingWrite?.future;
    settings = Map<String, dynamic>.from(data as Map);
    writes++;
    return settings as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  testWidgets('disable preserves duration and save persists slider changes', (
    tester,
  ) async {
    final api = TimeoutApi();
    Widget page() => ProviderScope(
      overrides: [
        backendServiceProvider.overrideWithValue(api),
        rawBackendServiceApiProvider.overrideWithValue(api),
      ],
      child: const MaterialApp(home: Scaffold(body: TimeoutSettings())),
    );
    await tester.pumpWidget(page());
    await tester.pumpAndSettle();
    expect(find.text('超时时间：3 分钟'), findsOneWidget);
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    expect(tester.widget<Slider>(find.byType(Slider)).onChanged, isNull);
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(api.settings, {'disabled': true, 'seconds': 180});
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    tester.widget<Slider>(find.byType(Slider)).onChanged!(450);
    await tester.pump();
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(api.settings, {'disabled': false, 'seconds': 450});
    await tester.pumpWidget(const SizedBox());
    await tester.pumpWidget(page());
    await tester.pumpAndSettle();
    expect(find.text('超时时间：7 分钟 30 秒'), findsOneWidget);
    expect(api.writes, 2);
  });
  testWidgets('普通绑定设备不读取和修改Core超时配置', (tester) async {
    final api = TimeoutApi();
    final container = ProviderContainer(
      overrides: [
        backendServiceProvider.overrideWithValue(api),
        rawBackendServiceApiProvider.overrideWithValue(api),
        deviceMeshServiceProvider.overrideWithValue(DeviceMeshService(api)),
      ],
    );
    addTearDown(container.dispose);
    await container
        .read(mobileDeploymentConfigProvider.notifier)
        .update(
          const MobileDeploymentConfig(
            mode: MobileDeploymentMode.cloud,
            remoteCoreUri: 'https://core.example',
          ),
        );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(home: Scaffold(body: TimeoutSettings())),
      ),
    );
    await tester.pumpAndSettle();
    expect(api.reads, 0);
    expect(api.writes, 0);
    expect(find.byType(Slider), findsNothing);
  });
  testWidgets('管理员撤权和重新授予后旧超时表单不可保存', (tester) async {
    final api = TimeoutApi()..admin = true;
    final container = ProviderContainer(
      overrides: [
        backendServiceProvider.overrideWithValue(api),
        rawBackendServiceApiProvider.overrideWithValue(api),
        deviceMeshServiceProvider.overrideWithValue(DeviceMeshService(api)),
      ],
    );
    addTearDown(container.dispose);
    await container
        .read(mobileDeploymentConfigProvider.notifier)
        .update(
          const MobileDeploymentConfig(
            mode: MobileDeploymentMode.cloud,
            remoteCoreUri: 'https://core.example',
          ),
        );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(home: Scaffold(body: TimeoutSettings())),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(Slider), findsOneWidget);
    api.permissionRevision = 3;
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(api.writes, 0);
    expect(find.byType(Slider), findsNothing);
    await tester.pumpWidget(const SizedBox());
  });
  testWidgets('停用超时和保存中点击禁用控件会提示原因', (tester) async {
    final api = TimeoutApi();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          backendServiceProvider.overrideWithValue(api),
          rawBackendServiceApiProvider.overrideWithValue(api),
        ],
        child: const MaterialApp(home: Scaffold(body: TimeoutSettings())),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byType(Switch));
    await tester.pumpAndSettle();
    await tester.tapAt(
      tester.getCenter(find.byType(Slider), warnIfMissed: false),
    );
    await tester.pump();
    expect(find.text('请先关闭“停用超时”。'), findsOneWidget);
    ScaffoldMessenger.of(
      tester.element(find.byType(Slider)),
    ).hideCurrentSnackBar();
    await tester.pumpAndSettle();
    api.pendingWrite = Completer<void>();
    await tester.tap(find.text('保存'));
    await tester.pump();
    expect(
      tester.widget<SwitchListTile>(find.byType(SwitchListTile)).onChanged,
      isNull,
    );
    await tester.tap(find.byType(SwitchListTile));
    await tester.pump();
    expect(find.text('正在保存，请稍后重试。'), findsOneWidget);
    ScaffoldMessenger.of(
      tester.element(find.byType(SwitchListTile)),
    ).hideCurrentSnackBar();
    await tester.pumpAndSettle();
    await tester.tap(find.text('保存中…'));
    await tester.pump();
    expect(find.text('正在保存，请稍后重试。'), findsOneWidget);
    expect(api.writes, 0);
    api.pendingWrite!.complete();
    await tester.pumpAndSettle();
    expect(api.writes, 1);
  });
}
