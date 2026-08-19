import '../backend_access/business_backend_unavailable.dart';
import '../runtime/status/runtime_status_snapshot.dart';
import 'backend_service_api.dart';

typedef BackendServiceApiResolver = BackendServiceApi? Function();

final class DynamicBackendServiceApiProxy implements BackendServiceApi {
  DynamicBackendServiceApiProxy({
    required BackendServiceApiResolver currentApi,
    required RuntimeStatusSnapshot Function() currentStatus,
  })  : _currentApi = currentApi,
        _currentStatus = currentStatus;

  final BackendServiceApiResolver _currentApi;
  final RuntimeStatusSnapshot Function() _currentStatus;

  BackendServiceApi _requireCurrentApi() {
    final status = _currentStatus();
    if (!status.businessAvailable) {
      throw BusinessBackendUnavailable(
        phase: status.phase,
        generation: status.generation,
        primaryError: status.primaryError,
      );
    }
    final api = _currentApi();
    if (api == null) {
      throw BusinessBackendUnavailable(
        phase: status.phase,
        generation: status.generation,
        primaryError: status.primaryError,
      );
    }
    if (api.generation != status.generation) {
      throw BusinessBackendUnavailable(
        phase: status.phase,
        generation: status.generation,
        primaryError: status.primaryError,
      );
    }
    return api;
  }

  @override
  int get generation => _currentStatus().generation;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    T Function(dynamic)? fromJson,
  }) {
    final api = _requireCurrentApi();
    return api.get<T>(
      path,
      queryParameters: queryParameters,
      fromJson: fromJson,
    );
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    T Function(dynamic)? fromJson,
  }) {
    final api = _requireCurrentApi();
    return api.post<T>(path, data: data, fromJson: fromJson);
  }

  @override
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    T Function(dynamic)? fromJson,
  }) {
    final api = _requireCurrentApi();
    return api.postPayload<T>(path, data: data, fromJson: fromJson);
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    T Function(dynamic)? fromJson,
  }) {
    final api = _requireCurrentApi();
    return api.put<T>(path, data: data, fromJson: fromJson);
  }

  @override
  Future<void> delete(String path) {
    final api = _requireCurrentApi();
    return api.delete(path);
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    T Function(dynamic)? fromJson,
  }) {
    final api = _requireCurrentApi();
    return api.deleteWithResponse<T>(path, data: data, fromJson: fromJson);
  }
}
