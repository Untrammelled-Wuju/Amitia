import 'dart:math';

import 'package:dio/dio.dart';

import 'backend_service_api.dart';

const List<String> _deviceLocalApiPrefixes = <String>[
  '/api/desktop-pets',
  '/api/desktop-pet',
  '/api/local/workflows',
  '/api/local/workflow-runs',
  '/api/local/workspaces',
  '/api/workspaces',
  '/api/storage',
  '/api/native-bridge/backend-action',
  '/internal/device-mesh',
];

bool isDeviceLocalApiPath(String path) {
  final normalized = path.split('?').first;
  return _deviceLocalApiPrefixes.any(
    (prefix) => normalized == prefix || normalized.startsWith('$prefix/'),
  );
}

bool isDeviceRoleManagementPath(String path) {
  final normalized = path.split('?').first;
  if (normalized == '/api/characters/generate-card' || normalized.endsWith('/test')) return false;
  return const ['/api/characters', '/api/character-templates', '/api/companion/role-profile'].any((prefix) => normalized == prefix || normalized.startsWith('$prefix/'));
}

final class RoutedBackendServiceApiProxy implements BackendServiceApi {
  RoutedBackendServiceApiProxy({
    required BackendServiceApi businessApi,
    required BackendServiceApi deviceLocalApi,
    bool Function()? isCloudDeployment,
  })  : _businessApi = businessApi,
        _deviceLocalApi = deviceLocalApi,
        _isCloudDeployment = isCloudDeployment;

  final BackendServiceApi _businessApi;
  final BackendServiceApi _deviceLocalApi;
  final bool Function()? _isCloudDeployment;
  final Random _random = Random.secure();
  int _requestCounter = 0;

  Future<BackendServiceApi> _apiFor(String path) async {
    if (isDeviceRoleManagementPath(path) && (_isCloudDeployment?.call() ?? false)) {
      final generation = _businessApi.generation;
      final payload = await _businessApi.get<Map<String, dynamic>>('/api/device-mesh/v1/coordination/me');
      if (generation != _businessApi.generation || !(_isCloudDeployment?.call() ?? false)) throw StateError('Core 已切换，请重新加载角色');
      final data = payload?['data'] is Map ? payload!['data'] as Map : payload;
      final policy = data?['policy'];
      if (policy is! Map || policy['coordinated'] is! bool) throw StateError('无法确认角色所属设备，请恢复 Core 连接后重试');
      return policy['coordinated'] == true ? _businessApi : _deviceLocalApi;
    }
    return isDeviceLocalApiPath(path) ? _deviceLocalApi : _businessApi;
  }

  Map<String, String>? _headersForMutation(
    String path,
    Map<String, String>? headers,
  ) {
    if (!isDeviceLocalApiPath(path)) return headers;
    final result = <String, String>{...?headers};
    result.putIfAbsent('X-Amitia-Client-Type', () => 'mobile');
    result.putIfAbsent('Idempotency-Key', _nextIdempotencyKey);
    return result;
  }

  String _nextIdempotencyKey() {
    _requestCounter++;
    final random = _random.nextInt(0x7fffffff).toRadixString(16);
    return 'mobile-${DateTime.now().microsecondsSinceEpoch}-$_requestCounter-$random';
  }

  @override
  int get generation => _businessApi.generation;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final routedHeaders = isDeviceLocalApiPath(path)
        ? <String, String>{...?headers, 'X-Amitia-Client-Type': 'mobile'}
        : headers;
    return (await _apiFor(path)).get<T>(
      path,
      queryParameters: queryParameters,
      headers: routedHeaders,
      fromJson: fromJson,
    );
  }

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    final routedHeaders = isDeviceLocalApiPath(path)
        ? <String, String>{...?headers, 'X-Amitia-Client-Type': 'mobile'}
        : headers;
    return (await _apiFor(path)).getStream(
      path,
      queryParameters: queryParameters,
      headers: routedHeaders,
      cancelToken: cancelToken,
    );
  }

  @override
  Future<Stream<List<int>>> postStream(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    return (await _apiFor(path)).postStream(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(path, headers),
      cancelToken: cancelToken,
    );
  }

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
    T Function(dynamic)? fromJson,
  }) async {
    return (await _apiFor(path)).postMultipart<T>(
      path,
      fields: fields,
      files: files,
      queryParameters: queryParameters,
      fromJson: fromJson,
    );
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    return (await _apiFor(path)).post<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(path, headers),
      fromJson: fromJson,
    );
  }

  @override
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    return (await _apiFor(path)).postPayload<T>(
      path,
      data: data,
      headers: _headersForMutation(path, headers),
      fromJson: fromJson,
    );
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    return (await _apiFor(path)).put<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(path, headers),
      fromJson: fromJson,
    );
  }

  @override
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    return (await _apiFor(path)).patch<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(path, headers),
      fromJson: fromJson,
    );
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    return (await _apiFor(path)).delete(
      path,
      queryParameters: queryParameters,
      headers: _headersForMutation(path, headers),
    );
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    return (await _apiFor(path)).deleteWithResponse<T>(
      path,
      data: data,
      headers: _headersForMutation(path, headers),
      fromJson: fromJson,
    );
  }
}
