import 'dart:math';

import 'package:dio/dio.dart';

import 'backend_service_api.dart';
import 'role_management_target.dart';

const List<String> _deviceLocalApiPrefixes = <String>[
  '/api/extension-center',
  '/api/extensions/packages',
  '/api/extensions/updates',
  '/api/extensions/versions',
  '/api/extensions/ui',
  '/api/extensions/kernel',
  '/api/extensions/kernel/extensions',
  '/api/extensions/pet',
  '/api/extensions/desktop-pet',
  '/api/extension',
  '/api/mcp',
  '/api/proactive-sse',
  '/api/reminders',
  '/api/desktop-pets',
  '/api/desktop-pet',
  '/api/local/workflows',
  '/api/local/workflow-runs',
  '/api/local/workspaces',
  '/api/workspaces',
  '/api/workbench/browser',
  '/api/workbench/terminal',
  '/api/storage',
  '/api/imports',
  '/api/native-bridge/backend-action',
  '/internal/device-mesh',
];

bool isDeviceLocalApiPath(String path) {
  final normalized = path.split('?').first;
  if (normalized == '/api/data-portability' ||
      normalized.startsWith('/api/data-portability/')) {
    return normalized.startsWith('/api/data-portability/backups') ||
        normalized.startsWith('/api/data-portability/restores');
  }
  return _deviceLocalApiPrefixes.any(
    (prefix) => normalized == prefix || normalized.startsWith('$prefix/'),
  );
}

const String centerConsoleMCPPrefix = '/api/device-mesh/v1/business/console/mcp';

bool isDeviceMCPPath(String path) {
  final normalized = path.split('?').first;
  return normalized == '/api/mcp' || normalized.startsWith('/api/mcp/');
}

String centerConsoleMCPPath(String path) {
  if (!isDeviceMCPPath(path)) return path;
  return path.replaceFirst('/api/mcp', centerConsoleMCPPrefix);
}

const String deviceAgentSkillPrefix = '/api/extensions/kernel/agent-skills';
const String centerConsoleSkillPrefix =
    '/api/device-mesh/v1/business/console/skills';

bool isDeviceAgentSkillPath(String path) {
  final normalized = path.split('?').first;
  return normalized == deviceAgentSkillPrefix ||
      normalized.startsWith('$deviceAgentSkillPrefix/');
}

String centerConsoleSkillPath(String path) {
  if (!isDeviceAgentSkillPath(path)) return path;
  return path.replaceFirst(deviceAgentSkillPrefix, centerConsoleSkillPrefix);
}

const String deviceReminderPrefix = '/api/reminders';

bool isDeviceReminderPath(String path) {
  final normalized = path.split('?').first;
  return normalized == deviceReminderPrefix ||
      normalized.startsWith('$deviceReminderPrefix/');
}

bool isDeviceRoleManagementPath(String path) {
  final normalized = path.split('?').first;
  if (normalized == '/api/characters/generate-card' ||
      normalized.endsWith('/test')) {
    return false;
  }
  return const [
    '/api/characters',
    '/api/character-templates',
    '/api/companion/role-profile',
    '/api/roles',
  ].any((prefix) => normalized == prefix || normalized.startsWith('$prefix/'));
}

Map<String, String>? stripRoleManagementTarget(Map<String, String>? headers) {
  if (headers == null || !headers.containsKey(roleManagementTargetHeader)) {
    return headers;
  }
  return Map<String, String>.from(headers)..remove(roleManagementTargetHeader);
}

final class RoutedBackendServiceApiProxy implements BackendServiceApi {
  RoutedBackendServiceApiProxy({
    required BackendServiceApi businessApi,
    required BackendServiceApi deviceLocalApi,
    bool Function()? isCloudDeployment,
  }) : _businessApi = businessApi,
       _deviceLocalApi = deviceLocalApi,
       _isCloudDeployment = isCloudDeployment;

  final BackendServiceApi _businessApi;
  final BackendServiceApi _deviceLocalApi;
  final bool Function()? _isCloudDeployment;
  final Random _random = Random.secure();
  int _requestCounter = 0;

  Future<_ResolvedCall> _apiFor(
    String path,
    Map<String, String>? headers,
  ) async {
    if (isDeviceMCPPath(path)) {
      final target = roleManagementTargetFromWire(
        headers?[roleManagementTargetHeader],
      );
      if (target != RoleManagementTarget.center) {
        return _ResolvedCall(_deviceLocalApi, path);
      }
      if (!(_isCloudDeployment?.call() ?? false)) {
        throw StateError('当前设备未接入统筹中心，无法使用统筹中心 MCP 数据');
      }
      return _ResolvedCall(_businessApi, centerConsoleMCPPath(path));
    }
    if (isDeviceAgentSkillPath(path)) {
      final target = roleManagementTargetFromWire(
        headers?[roleManagementTargetHeader],
      );
      if (target != RoleManagementTarget.center) {
        return _ResolvedCall(_deviceLocalApi, path);
      }
      if (!(_isCloudDeployment?.call() ?? false)) {
        throw StateError('当前设备未接入统筹中心，无法使用统筹中心 skill 数据');
      }
      return _ResolvedCall(_businessApi, centerConsoleSkillPath(path));
    }
    if (isDeviceReminderPath(path)) {
      final target = roleManagementTargetFromWire(
        headers?[roleManagementTargetHeader],
      );
      if (target != RoleManagementTarget.center) {
        return _ResolvedCall(_deviceLocalApi, path);
      }
      if (!(_isCloudDeployment?.call() ?? false)) {
        throw StateError('当前设备未接入统筹中心，无法使用统筹中心任务数据');
      }
      return _ResolvedCall(_businessApi, path);
    }
    if (!isDeviceRoleManagementPath(path)) {
      return _ResolvedCall(
        isDeviceLocalApiPath(path) ? _deviceLocalApi : _businessApi,
        path,
      );
    }
    final target = roleManagementTargetFromWire(
      headers?[roleManagementTargetHeader],
    );
    if (target == RoleManagementTarget.local) {
      return _ResolvedCall(_deviceLocalApi, path);
    }
    if (target == RoleManagementTarget.center &&
        !(_isCloudDeployment?.call() ?? false)) {
      throw StateError('当前设备未接入统筹中心，无法使用统筹中心角色数据');
    }
    return _ResolvedCall(_businessApi, path);
  }

  Map<String, String>? _headersForMutation(
    String path,
    Map<String, String>? headers,
  ) {
    final stripped = stripRoleManagementTarget(headers);
    if (!isDeviceLocalApiPath(path)) return stripped;
    final result = <String, String>{...?stripped};
    result.putIfAbsent('X-Amitia-Client-Type', () => 'mobile');
    result.putIfAbsent('Idempotency-Key', _nextIdempotencyKey);
    return result;
  }

  Map<String, String>? _headersForRead(
    String path,
    Map<String, String>? headers,
  ) {
    final stripped = stripRoleManagementTarget(headers);
    if (!isDeviceLocalApiPath(path)) return stripped;
    return <String, String>{...?stripped, 'X-Amitia-Client-Type': 'mobile'};
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
    final call = await _apiFor(path, headers);
    return call.api.get<T>(
      call.path,
      queryParameters: queryParameters,
      headers: _headersForRead(call.path, headers),
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
    final call = await _apiFor(path, headers);
    return call.api.getStream(
      call.path,
      queryParameters: queryParameters,
      headers: _headersForRead(call.path, headers),
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
    final call = await _apiFor(path, headers);
    return call.api.postStream(
      call.path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(call.path, headers),
      cancelToken: cancelToken,
    );
  }

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final call = await _apiFor(path, headers);
    return call.api.postMultipart<T>(
      call.path,
      fields: fields,
      files: files,
      queryParameters: queryParameters,
      headers: _headersForMutation(call.path, headers),
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
    final call = await _apiFor(path, headers);
    return call.api.post<T>(
      call.path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(call.path, headers),
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
    final call = await _apiFor(path, headers);
    return call.api.postPayload<T>(
      call.path,
      data: data,
      headers: _headersForMutation(call.path, headers),
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
    final call = await _apiFor(path, headers);
    return call.api.put<T>(
      call.path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(call.path, headers),
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
    final call = await _apiFor(path, headers);
    return call.api.patch<T>(
      call.path,
      data: data,
      queryParameters: queryParameters,
      headers: _headersForMutation(call.path, headers),
      fromJson: fromJson,
    );
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    final call = await _apiFor(path, headers);
    return call.api.delete(
      call.path,
      queryParameters: queryParameters,
      headers: _headersForMutation(call.path, headers),
    );
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final call = await _apiFor(path, headers);
    return call.api.deleteWithResponse<T>(
      call.path,
      data: data,
      headers: _headersForMutation(call.path, headers),
      fromJson: fromJson,
    );
  }
}

final class _ResolvedCall {
  const _ResolvedCall(this.api, this.path);

  final BackendServiceApi api;
  final String path;
}
