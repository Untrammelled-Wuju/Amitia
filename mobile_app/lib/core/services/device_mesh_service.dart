import 'dart:convert';

import '../backend_transport/backend_service_api.dart';
import 'package:dio/dio.dart';

List<Map<String, dynamic>> excludeDeviceMeshIdentity(
  List<Map<String, dynamic>> devices,
  String? localDeviceId,
) {
  final local = localDeviceId?.trim() ?? '';
  if (local.isEmpty) {
    return devices;
  }
  return devices
      .where((device) => device['deviceId']?.toString() != local)
      .toList(growable: false);
}

class DeviceMeshService {
  final BackendServiceApi _api;

  DeviceMeshService(this._api);

  Future<List<Map<String, dynamic>>> capabilityGrants(String target) async {
    final result = await _api.get<Map<String, dynamic>>(
      '/api/device-mesh/v1/business/devices/${Uri.encodeComponent(target)}/grants',
    );
    if (result?['grants'] is! List) {
      throw StateError('能力授权列表不可用');
    }
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

  Stream<List<Map<String, dynamic>>> approvalEvents(
    CancelToken cancelToken,
  ) async* {
    final stream = await _api.getStream(
      '/api/device-mesh/v1/pairing/approvals/stream',
      headers: const {'Accept': 'text/event-stream'},
      cancelToken: cancelToken,
    );
    final buffer = StringBuffer();
    await for (final chunk in stream.transform(utf8.decoder)) {
      buffer.write(chunk);
      var value = buffer.toString();
      var boundary = value.indexOf('\n\n');
      while (boundary >= 0) {
        final block = value.substring(0, boundary);
        value = value.substring(boundary + 2);
        buffer
          ..clear()
          ..write(value);
        var event = 'message';
        final data = <String>[];
        for (final line in const LineSplitter().convert(block)) {
          if (line.startsWith('event:')) event = line.substring(6).trim();
          if (line.startsWith('data:')) data.add(line.substring(5).trimLeft());
        }
        if (event != 'pairing_approvals' || data.isEmpty) {
          boundary = value.indexOf('\n\n');
          continue;
        }
        final decoded = jsonDecode(data.join('\n'));
        if (decoded is! Map || decoded['requests'] is! List) {
          throw StateError('配对审批事件格式无效');
        }
        yield (decoded['requests'] as List)
            .whereType<Map>()
            .map((row) => Map<String, dynamic>.from(row))
            .toList(growable: false);
        boundary = value.indexOf('\n\n');
      }
    }
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
    int ttlSeconds = 1800,
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

  Future<Map<String, dynamic>> capabilityExposure() async {
    final response = await _api.get<Map<String, dynamic>>(
      '/internal/device-mesh/capabilities',
    );
    if (response == null) throw StateError('无法读取设备能力开放状态');
    return response;
  }

  Future<Map<String, dynamic>> setCapabilityExposure({
    required String capability,
    required bool enabled,
    required int expectedRevision,
  }) async {
    final response = await _api.put<Map<String, dynamic>>(
      '/internal/device-mesh/capabilities/${Uri.encodeComponent(capability)}',
      data: <String, dynamic>{
        'enabled': enabled,
        'expectedRevision': expectedRevision,
      },
    );
    if (response == null || response['state'] is! Map) {
      throw StateError('设备未确认能力开放状态保存结果');
    }
    return response;
  }

  Future<Map<String, dynamic>> bootstrap({
    required String cloudBaseUrl,
    required String bootstrapTicket,
    String fingerprint = '',
    String coreId = '',
    bool replaceCandidate = true,
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/internal/device-mesh/bootstrap',
      data: <String, dynamic>{
        'cloudBaseUrl': cloudBaseUrl,
        'bootstrapTicket': bootstrapTicket,
        if (fingerprint.isNotEmpty) 'fingerprint': fingerprint,
        if (coreId.isNotEmpty) 'coreId': coreId,
        'replaceCandidate': replaceCandidate,
      },
      headers: const {'X-Amitia-Request-Timeout-Seconds': '50'},
    );
    if (response == null) throw StateError('本机设备绑定失败：后端未返回结果');
    return response;
  }

  Future<Map<String, dynamic>> updateEndpoint({
    required String cloudBaseUrl,
    required String coreId,
    String fingerprint = '',
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/internal/device-mesh/endpoint/update',
      data: <String, dynamic>{
        'cloudBaseUrl': cloudBaseUrl,
        'coreId': coreId,
        if (fingerprint.isNotEmpty) 'fingerprint': fingerprint,
      },
      headers: const {'X-Amitia-Request-Timeout-Seconds': '50'},
    );
    if (response?['ok'] != true) {
      throw StateError('本机设备未确认统筹中心地址更新结果');
    }
    return response!;
  }

  Future<void> deleteCredential() async {
    await _api.delete('/internal/device-mesh/credential');
  }
}
