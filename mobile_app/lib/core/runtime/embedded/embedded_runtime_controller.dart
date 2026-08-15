import '../backend/backend_topology.dart';

enum EmbeddedRuntimeStatus {
  notInstalled,
  stopped,
  starting,
  ready,
  stopping,
  failed,
  unsupported,
}

abstract interface class EmbeddedRuntimeController {
  Future<EmbeddedRuntimeStatus> ensureRunning(EmbeddedRuntimeProfile profile);
  Future<void> stop();
  Future<EmbeddedRuntimeStatus> getStatus();
  Future<BackendEndpoint> getEndpoint();
}
