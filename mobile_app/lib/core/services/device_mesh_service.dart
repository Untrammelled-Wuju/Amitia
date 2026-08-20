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

  Future<void> revokeDevice(String deviceId) async {
    await _api.delete('/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}');
  }

  Future<Map<String, dynamic>?> probeRuntime(String deviceId, String runtimeId) async {
    return _api.post<Map<String, dynamic>>(
      '/api/device-mesh/v1/devices/${Uri.encodeComponent(deviceId)}/runtimes/${Uri.encodeComponent(runtimeId)}/probe',
    );
  }
}
