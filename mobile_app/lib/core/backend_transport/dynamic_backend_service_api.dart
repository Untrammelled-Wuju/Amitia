import '../backend_access/business_backend_unavailable.dart';
import '../runtime/status/runtime_status_snapshot.dart';
import 'backend_service_api.dart';

typedef BackendServiceApiResolver = BackendServiceApi? Function();
typedef BusinessApiAvailabilityResolver =
    bool Function(RuntimeStatusSnapshot status, BackendServiceApi? api);
typedef BusinessBackendUnavailableListener =
    void Function(BusinessBackendUnavailable error);

final class DynamicBackendServiceApiProxy implements BackendServiceApi {
  DynamicBackendServiceApiProxy({
    required BackendServiceApiResolver currentApi,
    required RuntimeStatusSnapshot Function() currentStatus,
    BusinessApiAvailabilityResolver? canUseApi,
    BusinessBackendUnavailableListener? onUnavailable,
  }) : _currentApi = currentApi,
       _currentStatus = currentStatus,
       _canUseApi = canUseApi ?? _defaultCanUseApi,
       _onUnavailable = onUnavailable;

  final BackendServiceApiResolver _currentApi;
  final RuntimeStatusSnapshot Function() _currentStatus;
  final BusinessApiAvailabilityResolver _canUseApi;
  final BusinessBackendUnavailableListener? _onUnavailable;

  BackendServiceApi _requireCurrentApi() {
    final status = _currentStatus();
    final api = _currentApi();
    if (!_canUseApi(status, api)) {
      final error = BusinessBackendUnavailable(
        phase: status.phase,
        generation: status.generation,
        primaryError: status.primaryError,
      );
      _onUnavailable?.call(error);
      throw error;
    }
    return api!;
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
  }) {
    final api = _requireCurrentApi();
    return api.get<T>(
      path,
      queryParameters: queryParameters,
      headers: headers,
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
    final api = _requireCurrentApi();
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
  }) {
    final api = _requireCurrentApi();
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
  }) {
    final api = _requireCurrentApi();
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
  }) {
    final api = _requireCurrentApi();
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
  }) {
    final api = _requireCurrentApi();
    return api.delete(path, queryParameters: queryParameters, headers: headers);
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) {
    final api = _requireCurrentApi();
    return api.deleteWithResponse<T>(
      path,
      data: data,
      headers: headers,
      fromJson: fromJson,
    );
  }
}
