import '../services/device_mesh_service.dart';
import 'mobile_cloud_device_credential_store.dart';
import 'system_device_name.dart';

class DeviceNameReporter {
  final DeviceMeshService _service;
  final MobileCloudDeviceCredentialStore _store;
  String? _reportedKey;

  DeviceNameReporter(
    this._service, {
    MobileCloudDeviceCredentialStore store =
        const MobileCloudDeviceCredentialStore(),
  }) : _store = store;

  Future<Map<String, dynamic>?> reportCurrentName() async {
    final credential = await _store.load();
    if (credential == null || credential.isExpired) return null;
    final name = await resolveSystemDeviceName();
    if (name.isEmpty) return null;
    final key = '${credential.cloudBaseUrl}|${credential.deviceId}|$name';
    if (_reportedKey == key) return null;
    final device = await _service.reportDeviceName(credential.deviceId, name);
    _reportedKey = key;
    return device;
  }
}
