import 'backend_connection_endpoint.dart';
import 'backend_connection_credential.dart';

enum BackendAuthStrategy {
  localToken,
  deviceCredential,
}

class BackendConnectionConfig {
  final int schemaVersion;
  final int generation;
  final BackendConnectionEndpoint endpoint;
  final BackendAuthStrategy authStrategy;
  final BackendConnectionCredential credential;
  final String spaceId;
  final String deviceId;
  final String runtimeId;

  BackendConnectionConfig({
    required this.schemaVersion,
    required this.generation,
    required this.endpoint,
    required this.authStrategy,
    required this.credential,
    this.spaceId = '',
    this.deviceId = '',
    this.runtimeId = '',
  }) {
    if (schemaVersion != 1) {
      throw ArgumentError.value(schemaVersion, 'schemaVersion', 'must be 1');
    }
    if (generation <= 0) {
      throw ArgumentError.value(generation, 'generation', 'must be greater than 0');
    }
    if (authStrategy == BackendAuthStrategy.deviceCredential) {
      if (spaceId.trim().isEmpty || deviceId.trim().isEmpty || runtimeId.trim().isEmpty) {
        throw ArgumentError('device credential connections require spaceId, deviceId and runtimeId');
      }
    }
  }
}
