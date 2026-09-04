import 'dart:math';

import 'backend_service_api.dart';

const List<String> _deviceLocalApiPrefixes = <String>[
  '/api/desktop-pets',
  '/api/desktop-pet',
  '/api/local/workflows',
  '/api/local/workflow-runs',
  '/api/local/workspaces',
  '/api/workspaces',
  '/internal/device-mesh',
];

bool isDeviceLocalApiPath(String path) {
  final normalized = path.split('?').first;
  return _deviceLocalApiPrefixes.any(
    (prefix) => normalized == prefix || normalized.startsWith('$prefix/'),
  );
}

final class RoutedBackendServiceApiProxy implements BackendServiceApi {
  RoutedBackendServiceApiProxy({
    required BackendServiceApi businessApi,
    required BackendServiceApi deviceLocalApi,
  })  : _businessApi = businessApi,
        _deviceLocalApi = deviceLocalApi;

  final BackendServiceApi _businessApi;
  final BackendServiceApi _deviceLocalApi;
  final Random _random = Random.secure();
  int _requestCounter = 0;

  BackendServiceApi _apiFor(String path) {
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
  }) {
    final routedHeaders = isDeviceLocalApiPath(path)
        ? <String, String>{...?headers, 'X-Amitia-Client-Type': 'mobile'}
        : headers;
    return _apiFor(path).get<T>(
      path,
      queryParameters: queryParameters,
      headers: routedHeaders,
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
  }) {
    return _apiFor(path).post<T>(
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
  }) {
    return _apiFor(path).postPayload<T>(
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
  }) {
    return _apiFor(path).put<T>(
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
  }) {
    return _apiFor(path).patch<T>(
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
  }) {
    return _apiFor(path).delete(
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
  }) {
    return _apiFor(path).deleteWithResponse<T>(
      path,
      data: data,
      headers: _headersForMutation(path, headers),
      fromJson: fromJson,
    );
  }
}
