import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class MobileCloudDeviceCredential {
  final String cloudBaseUrl;
  final String credentialId;
  final String credential;
  final String spaceId;
  final String deviceId;
  final String runtimeId;
  final DateTime expiresAt;
  final String protocol;
  final int envelopeVersion;
  final String schemaVersion;
  final String websocketPath;

  const MobileCloudDeviceCredential({
    required this.cloudBaseUrl,
    required this.credentialId,
    required this.credential,
    required this.spaceId,
    required this.deviceId,
    required this.runtimeId,
    required this.expiresAt,
    required this.protocol,
    required this.envelopeVersion,
    required this.schemaVersion,
    required this.websocketPath,
  });

  bool get isExpired => !expiresAt.isAfter(DateTime.now().toUtc());

  Map<String, dynamic> toJson() => <String, dynamic>{
    'cloudBaseUrl': cloudBaseUrl,
    'credentialId': credentialId,
    'credential': credential,
    'spaceId': spaceId,
    'deviceId': deviceId,
    'runtimeId': runtimeId,
    'expiresAt': expiresAt.toUtc().toIso8601String(),
    'protocol': protocol,
    'envelopeVersion': envelopeVersion,
    'schemaVersion': schemaVersion,
    'websocketPath': websocketPath,
  };

  static MobileCloudDeviceCredential? fromJson(Map<String, dynamic> json) {
    final expiresAt = DateTime.tryParse((json['expiresAt'] ?? '').toString());
    final value = MobileCloudDeviceCredential(
      cloudBaseUrl: (json['cloudBaseUrl'] ?? '').toString().trim(),
      credentialId: (json['credentialId'] ?? '').toString().trim(),
      credential: (json['credential'] ?? '').toString().trim(),
      spaceId: (json['spaceId'] ?? '').toString().trim(),
      deviceId: (json['deviceId'] ?? '').toString().trim(),
      runtimeId: (json['runtimeId'] ?? '').toString().trim(),
      expiresAt: expiresAt?.toUtc() ?? DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
      protocol: (json['protocol'] ?? '').toString().trim(),
      envelopeVersion: (json['envelopeVersion'] as num?)?.toInt() ?? 0,
      schemaVersion: (json['schemaVersion'] ?? '').toString().trim(),
      websocketPath: (json['websocketPath'] ?? '').toString().trim(),
    );
    if (value.cloudBaseUrl.isEmpty ||
        value.credentialId.isEmpty ||
        value.credential.isEmpty ||
        value.spaceId.isEmpty ||
        value.deviceId.isEmpty ||
        value.runtimeId.isEmpty ||
        value.protocol != 'amitia.device-runtime' ||
        value.envelopeVersion != 1 ||
        value.schemaVersion != '1.0.0' ||
        value.websocketPath != '/api/device-mesh/v1/runtime/ws') {
      return null;
    }
    return value;
  }
}

class MobileCloudDeviceCredentialStore {
  static const _key = 'amitia.device-mesh.cloud-credential.v1';
  final FlutterSecureStorage _storage;

  const MobileCloudDeviceCredentialStore({
    FlutterSecureStorage storage = const FlutterSecureStorage(),
  }) : _storage = storage;

  Future<MobileCloudDeviceCredential?> load() async {
    final raw = await _storage.read(key: _key);
    if (raw == null || raw.trim().isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map) return null;
      return MobileCloudDeviceCredential.fromJson(
        Map<String, dynamic>.from(decoded),
      );
    } catch (_) {
      return null;
    }
  }

  Future<void> save(MobileCloudDeviceCredential credential) async {
    await _storage.write(key: _key, value: jsonEncode(credential.toJson()));
  }

  Future<void> clear() => _storage.delete(key: _key);
}
