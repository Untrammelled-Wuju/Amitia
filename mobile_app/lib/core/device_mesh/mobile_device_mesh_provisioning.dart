import 'package:dio/dio.dart';
import 'package:dio/io.dart';

import '../errors/original_error.dart';

import '../runtime/backend/backend_topology_resolver.dart';
import '../security/certificate_pin.dart';
import 'mobile_cloud_device_credential_store.dart';

class MobileDeviceMeshProvisioning {
  final MobileCloudDeviceCredentialStore _store;

  const MobileDeviceMeshProvisioning({
    MobileCloudDeviceCredentialStore store =
        const MobileCloudDeviceCredentialStore(),
  }) : _store = store;

  Future<MobileCloudDeviceCredential> exchangeBootstrapTicket({
    required String coreUri,
    required String bootstrapTicket,
    required String deviceId,
    required String runtimeId,
    required String platform,
    required String fingerprint,
  }) async {
    final core = normalizeRemoteCoreUri(coreUri);
    final ticket = bootstrapTicket.trim();
    final normalizedDeviceId = deviceId.trim();
    final normalizedRuntimeId = runtimeId.trim();
    final normalizedPlatform = platform.trim().toLowerCase();
    final normalizedFingerprint = fingerprint.trim().toLowerCase();
    if (ticket.isEmpty ||
        normalizedDeviceId.isEmpty ||
        normalizedRuntimeId.isEmpty ||
        normalizedPlatform.isEmpty ||
        !RegExp(r'^[0-9a-f]{64}$').hasMatch(normalizedFingerprint)) {
      throw StateError('Device Mesh bootstrap identity is incomplete');
    }

    final dio = Dio(
      BaseOptions(
        baseUrl: core.toString(),
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 20),
        headers: <String, String>{
          'Accept': 'application/json',
          'Content-Type': 'application/json',
          'Authorization': 'AmitiaBootstrap $ticket',
          'X-Amitia-Client-Type': 'mobile',
        },
      ),
    );
    dio.httpClientAdapter = IOHttpClientAdapter(
      createHttpClient: () =>
          createCertificatePinnedHttpClient(normalizedFingerprint),
    );
    try {
      final response = await dio.post<dynamic>(
        '/api/public/device-mesh/v1/bootstrap/exchange',
        data: <String, dynamic>{
          'deviceId': normalizedDeviceId,
          'runtimeId': normalizedRuntimeId,
          'platform': normalizedPlatform,
          'runtimeVersion': 'mobile-1',
        },
      );
      final raw = response.data;
      if (raw is! Map) {
        throw StateError('统筹中心返回了无效的 Device Credential');
      }
      final data = Map<String, dynamic>.from(raw);
      final expiresAt = DateTime.tryParse(
        (data['expiresAt'] ?? '').toString(),
      )?.toUtc();
      final credential = MobileCloudDeviceCredential(
        cloudBaseUrl: core.toString().replaceAll(RegExp(r'/+$'), ''),
        credentialId: (data['credentialId'] ?? '').toString().trim(),
        credential: (data['credential'] ?? '').toString().trim(),
        spaceId: (data['spaceId'] ?? '').toString().trim(),
        deviceId: (data['deviceId'] ?? '').toString().trim(),
        runtimeId: (data['runtimeId'] ?? '').toString().trim(),
        fingerprint: normalizedFingerprint,
        expiresAt:
            expiresAt ?? DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
        protocol: (data['protocol'] ?? '').toString().trim(),
        envelopeVersion: (data['envelopeVersion'] as num?)?.toInt() ?? 0,
        schemaVersion: (data['schemaVersion'] ?? '').toString().trim(),
        websocketPath: (data['websocketPath'] ?? '').toString().trim(),
      );
      final validated = MobileCloudDeviceCredential.fromJson(
        credential.toJson(),
      );
      if (validated == null ||
          validated.deviceId != normalizedDeviceId ||
          validated.runtimeId != normalizedRuntimeId ||
          validated.isExpired) {
        throw StateError('统筹中心 Device Credential 验证失败');
      }
      await _store.save(validated);
      return validated;
    } on DioException catch (error) {
      final body = error.response?.data;
      final message = body is Map
          ? (body['message'] ?? body['msg'] ?? '').toString().trim()
          : '';
      throw StateError(message.isNotEmpty ? message : originalErrorText(error));
    } finally {
      dio.close(force: true);
    }
  }

  Future<MobileCloudDeviceCredential?> currentFor(String coreUri) async {
    final core = normalizeRemoteCoreUri(coreUri);
    final stored = await _store.load();
    if (stored == null || stored.isExpired) return null;
    if (normalizeRemoteCoreUri(stored.cloudBaseUrl).origin != core.origin) {
      return null;
    }
    return stored;
  }

  Future<void> clear() => _store.clear();
}
