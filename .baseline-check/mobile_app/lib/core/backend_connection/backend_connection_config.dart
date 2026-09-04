import 'backend_connection_endpoint.dart';
import 'backend_connection_credential.dart';

enum BackendAuthStrategy {
  localToken,
  bearer,
}

class BackendConnectionConfig {
  final int schemaVersion;
  final int generation;
  final BackendConnectionEndpoint endpoint;
  final BackendAuthStrategy authStrategy;
  final BackendConnectionCredential credential;

  BackendConnectionConfig({
    required this.schemaVersion,
    required this.generation,
    required this.endpoint,
    required this.authStrategy,
    required this.credential,
  }) {
    if (schemaVersion != 1) {
      throw ArgumentError.value(schemaVersion, 'schemaVersion', 'must be 1');
    }
    if (generation <= 0) {
      throw ArgumentError.value(generation, 'generation', 'must be greater than 0');
    }
  }
}
