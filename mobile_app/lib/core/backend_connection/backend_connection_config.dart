import 'backend_connection_endpoint.dart';
import 'backend_connection_credential.dart';

enum BackendAuthStrategy { localToken, deviceCredential }

class BackendConnectionConfig {
  final int schemaVersion;
  final int generation;
  final BackendConnectionEndpoint endpoint;
  final BackendAuthStrategy authStrategy;
  final BackendConnectionCredential credential;
  final String spaceId;
  final String deviceId;
  final String runtimeId;
  final String fingerprint;

  BackendConnectionConfig({
    required this.schemaVersion,
    required this.generation,
    required this.endpoint,
    required this.authStrategy,
    required this.credential,
    this.spaceId = '',
    this.deviceId = '',
    this.runtimeId = '',
    this.fingerprint = '',
  }) {
    if (schemaVersion != 1) {
      throw ArgumentError.value(schemaVersion, 'schemaVersion', 'must be 1');
    }
    if (generation <= 0) {
      throw ArgumentError.value(
        generation,
        'generation',
        'must be greater than 0',
      );
    }
    if (authStrategy == BackendAuthStrategy.deviceCredential) {
      if (spaceId.trim().isEmpty ||
          deviceId.trim().isEmpty ||
          runtimeId.trim().isEmpty) {
        throw ArgumentError(
          'device credential connections require spaceId, deviceId and runtimeId',
        );
      }
      if (fingerprint.trim().isNotEmpty &&
          !RegExp(r'^[0-9a-f]{64}$').hasMatch(fingerprint.trim())) {
        throw ArgumentError.value(
          fingerprint,
          'fingerprint',
          'must be a lowercase SHA-256 fingerprint',
        );
      }
    }
  }
}
