import 'package:dio/dio.dart';

import '../backend_access/business_backend_unavailable.dart';
import '../runtime/status/runtime_status_snapshot.dart';
import 'backend_service_api.dart';

typedef BackendServiceApiResolver = BackendServiceApi? Function();
typedef BusinessApiAvailabilityResolver =
    bool Function(RuntimeStatusSnapshot status, BackendServiceApi? api);
typedef BusinessBackendUnavailableListener =
    void Function(BusinessBackendUnavailable error);
typedef BusinessApiAvailabilityWaiter = Future<void> Function();

final class DynamicBackendServiceApiProxy implements BackendServiceApi {
  DynamicBackendServiceApiProxy({
    required BackendServiceApiResolver currentApi,
    required RuntimeStatusSnapshot Function() currentStatus,
    BusinessApiAvailabilityResolver? canUseApi,
    BusinessBackendUnavailableListener? onUnavailable,
    BusinessApiAvailabilityWaiter? waitForAvailability,
  }) : _currentApi = currentApi,
       _currentStatus = currentStatus,
       _canUseApi = canUseApi ?? _defaultCanUseApi,
       _onUnavailable = onUnavailable,
       _waitForAvailability = waitForAvailability;

  final BackendServiceApiResolver _currentApi;
  final RuntimeStatusSnapshot Function() _currentStatus;
  final BusinessApiAvailabilityResolver _canUseApi;
  final BusinessBackendUnavailableListener? _onUnavailable;
  final BusinessApiAvailabilityWaiter? _waitForAvailability;

  Future<BackendServiceApi> _requireCurrentApi() async {
    final status = _currentStatus();
    final api = _currentApi();
    if (_canUseApi(status, api)) return api!;
    final waiter = _waitForAvailability;
    if (waiter != null) {
      await waiter();
      final readyStatus = _currentStatus();
      final readyApi = _currentApi();
      if (_canUseApi(readyStatus, readyApi)) return readyApi!;
    }
    final error = BusinessBackendUnavailable(
      phase: status.phase,
      generation: status.generation,
      primaryError: status.primaryError,
    );
    _onUnavailable?.call(error);
    throw error;
  }

  static bool _defaultCanUseApi(
    RuntimeStatusSnapshot status,
    BackendServiceApi? api,
  ) {
    return status.businessAvailable &&
        api != null &&
        api.generation == status.generation;
  }

  @override
  int get generation => _currentStatus().generation;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final api = await _requireCurrentApi();
    return api.get<T>(
      path,
      queryParameters: queryParameters,
      headers: headers,
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
    final api = await _requireCurrentApi();
    return api.getStream(
      path,
      queryParameters: queryParameters,
      headers: headers,
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
    final api = await _requireCurrentApi();
    return api.postStream(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: headers,
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
    final api = await _requireCurrentApi();
    return api.postMultipart<T>(
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
    final api = await _requireCurrentApi();
    return api.post<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: headers,
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
    final api = await _requireCurrentApi();
    return api.postPayload<T>(
      path,
      data: data,
      headers: headers,
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
    final api = await _requireCurrentApi();
    return api.put<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: headers,
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
    final api = await _requireCurrentApi();
    return api.patch<T>(
      path,
      data: data,
      queryParameters: queryParameters,
      headers: headers,
      fromJson: fromJson,
    );
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    final api = await _requireCurrentApi();
    return api.delete(path, queryParameters: queryParameters, headers: headers);
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final api = await _requireCurrentApi();
    return api.deleteWithResponse<T>(
      path,
      data: data,
      headers: headers,
      fromJson: fromJson,
    );
  }
}
