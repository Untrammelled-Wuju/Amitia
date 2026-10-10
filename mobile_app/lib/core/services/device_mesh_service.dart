import '../backend_transport/backend_service_api.dart';
import 'package:dio/dio.dart';
import 'device_owned_chat_service.dart';

class DeviceMeshService {
  final BackendServiceApi _api;

  DeviceMeshService(this._api);

  Future<Map<String, dynamic>> prepareCall(String target) async {
    final before = await coordination();
    if (before['coordinationAvailable'] != true)
      throw StateError('Core 的设备业务服务尚未就绪');
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/roles',
      queryParameters: {'targetDeviceId': target},
    );
    final after = await coordination();
    if (after['coreId'] != before['coreId'] ||
        after['coordinationAvailable'] != true ||
        result?['roles'] is! List ||
        result?['roleOwnerId'] is! String ||
        result?['executionScope'] is! Map ||
        result?['executionScope']['coreId'] != before['coreId'] ||
        result?['executionScope']['targetDeviceId'] != target ||
        result?['executionScope']['resourceOwnerId'] !=
            result?['roleOwnerId']) {
      throw StateError('服务提供者已变化，请刷新后重新选择角色');
    }
    return {...result!, 'coreId': before['coreId']};
  }

  Stream<Map<String, dynamic>> call({
    required String target,
    required String role,
    required String core,
    required String owner,
    required String requestId,
    required String message,
    required Map<String, dynamic> expectedScope,
    required CancelToken cancelToken,
  }) async* {
    final stream = await _api.postStream(
      '/api/device-mesh/v1/business/messages',
      data: {
        'targetDeviceId': target,
        'characterId': role,
        'requestId': requestId,
        'message': message,
        'expectedExecutionScope': expectedScope,
      },
      headers: {'Accept': 'text/event-stream'},
      cancelToken: cancelToken,
    );
    await for (final event in decodeOwnedChatStream(stream, requestId)) {
      final scope =
          event['executionScope'] ??
          (event['data'] is Map ? event['data']['executionScope'] : null);
      if (scope is Map && scope['coreId'] != core) {
        cancelToken.cancel('服务提供者已变化');
        throw StateError('云端服务提供者已从「$core」切换为「${scope['coreId']}」，当前调用已中断。');
      }
      if (scope is Map &&
          (scope['targetDeviceId'] != target ||
              scope['resourceOwnerId'] != owner ||
              scope['roleId'] != role)) {
        cancelToken.cancel('目标数据归属或角色已变化');
        throw StateError('目标设备的数据归属或角色已变化，当前调用已中断。');
      }
      if (cancelToken.isCancelled) throw StateError('当前调用已中断，迟到回复已拦截');
      yield event;
    }
  }

  Future<void> interruptCall(String requestId) async {
    await _api.post(
      '/api/device-mesh/v1/business/messages/${Uri.encodeComponent(requestId)}/interrupt',
    );
  }

  Future<List<Map<String, dynamic>>> capabilityGrants(String target) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/devices/${Uri.encodeComponent(target)}/grants',
    );
    if (result?['grants'] is! List) throw StateError('能力授权列表不可用');
    return (result!['grants'] as List)
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .toList();
  }

  Future<Map<String, dynamic>> setCapabilityGrant({
    required String target,
    required String caller,
    required String capability,
    required bool allowed,
    required int expectedRevision,
    required String expectedCoreId,
    Map<String, String>? headers,
  }) async {
    final result = await _api.put<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/devices/${Uri.encodeComponent(target)}/grants',
      headers: headers,
      data: {
        'callerId': caller,
        'capability': capability.trim(),
        'allowed': allowed,
        'expectedRevision': expectedRevision,
        'expectedCoreId': expectedCoreId,
      },
    );
    final grant = result?['grant'];
    if (grant is! Map ||
        grant['targetId'] != target ||
        grant['callerId'] != caller ||
        grant['capability'] != capability.trim() ||
        grant['allowed'] != allowed ||
        grant['revision'] != expectedRevision + 1)
      throw StateError('Core未确认原能力授权保存结果');
    return Map<String, dynamic>.from(grant);
  }

  Future<List<Map<String, dynamic>>> pendingApprovals() async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/pairing/approvals',
    );
    final requests = result?['requests'];
    return requests is List
        ? requests
              .whereType<Map>()
              .map((value) => Map<String, dynamic>.from(value))
              .toList()
        : const [];
  }

  Future<void> decideApproval(
    String requestId, {
    required bool allow,
    required int expectedRevision,
    Map<String, String>? headers,
  }) async {
    await _api.put(
      '/api/device-mesh/v1/pairing/approvals/${Uri.encodeComponent(requestId)}',
      headers: headers,
      data: {'allow': allow, 'expectedRevision': expectedRevision},
    );
  }

  Future<Map<String, dynamic>> coordination() async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
    );
    if (result == null) throw StateError('无法读取当前设备统筹状态');
    return result;
  }

  Future<Map<String, dynamic>> changeCoordination({
    required bool coordinated,
    required int expectedRevision,
    String selectedRole = '',
    Map<String, String>? headers,
  }) async {
    final result = await _api.put<Map<String, dynamic>>(
      '/api/device-mesh/v1/coordination/me',
      headers: headers,
      data: {
        'coordinated': coordinated,
        'expectedRevision': expectedRevision,
        'selectedRole': selectedRole,
      },
    );
    if (result == null) throw StateError('统筹模式更新失败');
    return result;
  }

  Future<void> grantAdministrator(
    String deviceId, {
    required bool grant,
    required int expectedRevision,
    Map<String, String>? headers,
  }) async {
    await _api.put(
      '/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}/administrator',
      headers: headers,
      data: {'grant': grant, 'expectedRevision': expectedRevision},
    );
  }

  Future<List<Map<String, dynamic>>> devices() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/devices',
    );
    final items = response?['devices'];
    if (items is! List) return const [];
    return items
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList();
  }

  Future<Map<String, dynamic>> createPairingOffer({
    int ttlSeconds = 600,
    String endpoint = '',
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/pairing/offers',
      data: <String, dynamic>{
        'ttlSeconds': ttlSeconds,
        if (endpoint.isNotEmpty) 'endpoint': endpoint,
      },
    );
    if (response == null) {
      throw StateError('云端未返回设备配对 Offer');
    }
    return response;
  }

  Future<void> revokeDevice(
    String deviceId, {
    Map<String, String>? headers,
  }) async {
    await _api.delete(
      '/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}',
      headers: headers,
    );
  }

  Future<Map<String, dynamic>?> probeRuntime(
    String deviceId,
    String runtimeId, {
    Map<String, String>? headers,
  }) async {
    return _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}/runtimes/${Uri.encodeComponent(runtimeId)}/probe',
      headers: headers,
    );
  }

  Future<Map<String, dynamic>?> syncStatus(
    String deviceId, {
    Map<String, String>? headers,
  }) async {
    return _api.get<Map<String, dynamic>>(
      '/api/v1/sync/status',
      queryParameters: <String, dynamic>{'deviceId': deviceId},
      headers: headers,
    );
  }
}

class DeviceMeshLocalService {
  final BackendServiceApi _api;

  DeviceMeshLocalService(this._api);

  Future<List<Map<String, dynamic>>> lanEndpoints() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/internal/device-mesh/lan',
    );
    final endpoints = response?['endpoints'];
    return endpoints is List
        ? endpoints
              .whereType<Map>()
              .map((entry) => Map<String, dynamic>.from(entry))
              .toList()
        : const [];
  }

  Future<Map<String, dynamic>> identity() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/internal/device-mesh/identity',
    );
    if (response == null) throw StateError('无法读取本机 Device Mesh 身份');
    return response;
  }

  Future<Map<String, dynamic>> status() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/internal/device-mesh/status',
    );
    if (response == null) throw StateError('无法读取本机 Device Mesh 状态');
    return response;
  }

  Future<Map<String, dynamic>> bootstrap({
    required String cloudBaseUrl,
    required String bootstrapTicket,
    String fingerprint = '',
    String coreId = '',
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/internal/device-mesh/bootstrap',
      data: <String, dynamic>{
        'cloudBaseUrl': cloudBaseUrl,
        'bootstrapTicket': bootstrapTicket,
        if (fingerprint.isNotEmpty) 'fingerprint': fingerprint,
        if (coreId.isNotEmpty) 'coreId': coreId,
      },
    );
    if (response == null) throw StateError('本机设备绑定失败：后端未返回结果');
    return response;
  }

  Future<void> deleteCredential() async {
    await _api.delete('/internal/device-mesh/credential');
  }
}
