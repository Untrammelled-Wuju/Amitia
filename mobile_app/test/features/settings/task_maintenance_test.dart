import 'package:amitia_app/core/services/task_maintenance_contract.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';
import 'package:amitia_app/features/settings/presentation/pages/long_running_page.dart';

class MaintenanceApi implements BackendServiceApi {
  @override
  int get generation => 1;
  final paths = <String>[];
  Map<String, dynamic> current = config();
  bool badAck = false;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    final value = path.endsWith('/tasks/config')
        ? current
        : path.endsWith('/tasks/status')
        ? <String, dynamic>{
            'source': 'task-runtime',
            'running': true,
            'activeCount': 1,
            'localProcessCount': 1,
            'tasks': [
              {
                'id': 'real-run',
                'title': '真实任务',
                'status': 'running',
                'placement': 'local',
              },
            ],
          }
        : <String, dynamic>{};
    return value as T;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    paths.add(path);
    final request = data as Map;
    current = {
      ...current,
      'maxTasks': request['maxTasks'],
      'timeout': request['timeout'],
      'revision': request['expectedRevision'] + 1,
      'timeoutRevision': request['expectedTimeoutRevision'] + 1,
      'applied': !badAck,
    };
    return current as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> config() => {
  'source': 'task-runtime',
  'applied': true,
  'revision': 2,
  'timeoutRevision': 3,
  'maxTasks': 4,
  'timeout': {'disabled': false, 'seconds': 180},
};

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  testWidgets(
    'maintenance page reads real tasks and requires applied save acknowledgement',
    (tester) async {
      final api = MaintenanceApi();
      final router = GoRouter(
        routes: [
          GoRoute(
            path: '/',
            builder: (context, state) => const LongRunningPage(),
          ),
        ],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            backendServiceProvider.overrideWithValue(api),
            rawBackendServiceApiProvider.overrideWithValue(api),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.textContaining('真实任务'), findsOneWidget);
      expect(api.paths, contains('/api/runtime/tasks/status'));
      await tester.ensureVisible(find.text('保存长期运行配置'));
      await tester.tap(find.text('保存长期运行配置'));
      await tester.pumpAndSettle();
      expect(find.text('执行器应用配置完成'), findsOneWidget);
      await tester.pump(const Duration(seconds: 5));
      await tester.pumpAndSettle();
      api.badAck = true;
      await tester.tap(find.text('保存长期运行配置'));
      await tester.pumpAndSettle();
      expect(find.textContaining('任务执行器未确认配置生效'), findsOneWidget);
      expect(api.paths.any((path) => path.contains('long-running')), isFalse);
    },
  );
  test('only accepts the real executor and valid effective policy', () {
    expect(requireAppliedTaskConfig(config())['maxTasks'], 4);
    for (final invalid in [
      {...config(), 'source': 'legacy'},
      {...config(), 'applied': false},
      {...config(), 'maxTasks': 0},
      {
        ...config(),
        'timeout': {'disabled': false, 'seconds': 31},
      },
    ]) {
      expect(() => requireAppliedTaskConfig(invalid), throwsStateError);
    }
  });
  test('save acknowledgement must match both policy revisions and values', () {
    final request = {
      'expectedRevision': 1,
      'expectedTimeoutRevision': 2,
      'maxTasks': 4,
      'timeout': {'disabled': false, 'seconds': 180},
    };
    requireSavedTaskConfig(config(), request);
    expect(
      () => requireSavedTaskConfig({...config(), 'revision': 1}, request),
      throwsStateError,
    );
    expect(
      () => requireSavedTaskConfig({...config(), 'maxTasks': 8}, request),
      throwsStateError,
    );
  });
}
