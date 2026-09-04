import '../backend_transport/backend_service_api.dart';

class DeviceMeshService {
  final BackendServiceApi _api;

  DeviceMeshService(this._api);

  Future<List<Map<String, dynamic>>> devices() async {
    final response = await _api.get<Map<String, dynamic>>('/api/device-mesh/v1/devices');
    final items = response?['devices'];
    if (items is! List) return const [];
    return items.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList();
  }

  Future<Map<String, dynamic>> createBootstrapTicket({
    required String deviceId,
    required String runtimeId,
    required String platform,
    String label = '',
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/bootstrap-tickets',
      data: <String, dynamic>{
        'deviceId': deviceId,
        'runtimeId': runtimeId,
        'platform': platform,
        if (label.trim().isNotEmpty) 'label': label.trim(),
      },
    );
    if (response == null) {
      throw StateError('云端未返回设备绑定凭据');
    }
    return response;
  }

  Future<void> revokeDevice(String deviceId) async {
    await _api.delete('/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}');
  }

  Future<Map<String, dynamic>?> probeRuntime(String deviceId, String runtimeId) async {
    return _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}/runtimes/${Uri.encodeComponent(runtimeId)}/probe',
    );
  }

  Future<Map<String, dynamic>?> syncStatus(String deviceId) async {
    return _api.get<Map<String, dynamic>>(
      '/api/v1/sync/status',
      queryParameters: <String, dynamic>{'deviceId': deviceId},
    );
  }
}

class DeviceMeshLocalService {
  final BackendServiceApi _api;

  DeviceMeshLocalService(this._api);

  Future<Map<String, dynamic>> identity() async {
    final response = await _api.get<Map<String, dynamic>>('/internal/device-mesh/identity');
    if (response == null) throw StateError('无法读取本机 Device Mesh 身份');
    return response;
  }

  Future<Map<String, dynamic>> status() async {
    final response = await _api.get<Map<String, dynamic>>('/internal/device-mesh/status');
    if (response == null) throw StateError('无法读取本机 Device Mesh 状态');
    return response;
  }

  Future<Map<String, dynamic>> bootstrap({
    required String cloudBaseUrl,
    required String bootstrapTicket,
  }) async {
    final response = await _api.post<Map<String, dynamic>>(
      '/internal/device-mesh/bootstrap',
      data: <String, dynamic>{
        'cloudBaseUrl': cloudBaseUrl,
        'bootstrapTicket': bootstrapTicket,
      },
    );
    if (response == null) throw StateError('本机设备绑定失败：后端未返回结果');
    return response;
  }

  Future<void> deleteCredential() async {
    await _api.delete('/internal/device-mesh/credential');
  }
}
