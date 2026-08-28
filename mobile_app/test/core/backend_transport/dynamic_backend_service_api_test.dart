import 'package:amitia_app/core/backend_access/business_backend_unavailable.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/dynamic_backend_service_api.dart';
import 'package:amitia_app/core/runtime/runtime_bridge_state.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_phase.dart';
import 'package:amitia_app/core/runtime/status/runtime_status_snapshot.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('本地运行环境未就绪时记录并阻止业务请求', () {
    BusinessBackendUnavailable? recorded;
    final api = DynamicBackendServiceApiProxy(
      currentApi: () => _FakeApi(1),
      currentStatus: _installRequiredStatus,
      onUnavailable: (error) => recorded = error,
    );

    expect(
      () => api.get('/api/test'),
      throwsA(isA<BusinessBackendUnavailable>()),
    );
    expect(recorded?.phase, RuntimeStatusPhase.installRequired);
    expect(recorded?.generation, 0);
  });

  test('远程业务后端可用时不受本地运行环境状态阻塞', () async {
    final rawApi = _FakeApi(1);
    final api = DynamicBackendServiceApiProxy(
      currentApi: () => rawApi,
      currentStatus: _installRequiredStatus,
      canUseApi: (_, currentApi) => currentApi != null,
    );

    await api.get('/api/test');

    expect(rawApi.getCount, 1);
  });
}

RuntimeStatusSnapshot _installRequiredStatus() {
  return const RuntimeStatusSnapshot(
    phase: RuntimeStatusPhase.installRequired,
    runtimeState: RuntimeBridgeState.notInstalled,
    runtimeReady: false,
    runtimeInstalled: false,
    backendConfigured: false,
    httpAvailable: false,
    webSocketConnected: false,
    businessAvailable: false,
    generation: 0,
  );
}

class _FakeApi implements BackendServiceApi {
  _FakeApi(this.generation);

  @override
  final int generation;
  int getCount = 0;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    getCount++;
    return null;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {}

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async => null;
}
